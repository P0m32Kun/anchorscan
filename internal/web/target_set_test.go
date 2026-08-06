package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// Ticket 07: the Target Set import endpoint must accept/normalize/dedup/reject
// with visible statistics, survive a project delete, and prefill Scan Create.

func seedProjectWithTargetSet(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	dir := t.TempDir()
	handler, err := NewServer(ServerOptions{ConfigPath: dir + "/config.yaml", DBPath: dir + "/scan.db"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	s, err := store.Open(dir + "/scan.db")
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}
	defer s.Close()
	if err := s.SaveProject(store.Project{ID: "ts-web", Name: "TS Web", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("ts-web"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if _, err := s.SaveTargetSet("ts-web", "", "192.0.2.10\n198.51.100.0/24"); err != nil {
		t.Fatalf("SaveTargetSet returned error: %v", err)
	}
	return handler, "ts-web", dir + "/scan.db"
}

func TestTargetSetImportEndpoint(t *testing.T) {
	handler, projectID, _ := seedProjectWithTargetSet(t)
	form := url.Values{
		"targets": {"192.0.2.10\n2001:DB8::1\n2001:db8::1\nhttps://example.com\n-sV\n192.0.2.999\n"},
	}
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/target-set", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", res.Code)
	}
	loc := res.Header().Get("Location")
	if !strings.Contains(loc, "targets_imported=1") {
		t.Fatalf("redirect must carry import marker: %s", loc)
	}
	if !strings.Contains(loc, "accepted=2") || !strings.Contains(loc, "duplicates=1") || !strings.Contains(loc, "rejected=3") {
		t.Fatalf("redirect must carry accepted/duplicate/rejected counts: %s", loc)
	}
	// Reasons are carried for display.
	if !strings.Contains(loc, "reasons=") {
		t.Fatalf("redirect must carry rejection reasons: %s", loc)
	}
}

func TestTargetSetImportPersistsCanonicalTargets(t *testing.T) {
	handler, projectID, dbPath := seedProjectWithTargetSet(t)
	form := url.Values{"targets": {"192.0.2.10\n198.51.100.7\n2001:DB8::1\n-flag\n"}}
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/target-set", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", res.Code)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}
	defer s.Close()
	ts, err := s.GetTargetSet(projectID)
	if err != nil {
		t.Fatalf("GetTargetSet returned error: %v", err)
	}
	got := strings.Join(ts.TargetList(), ",")
	want := "192.0.2.10,198.51.100.7,2001:db8::1"
	if got != want {
		t.Fatalf("stored targets mismatch: got %q want %q", got, want)
	}
}

func TestTargetSetPrefillsScanCreate(t *testing.T) {
	handler, projectID, _ := seedProjectWithTargetSet(t)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/scans/new", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "192.0.2.10") || !strings.Contains(body, "198.51.100.0/24") {
		t.Fatalf("scan create must prefill the Target Set targets")
	}
	// Prefill must not inject into a rerun form.
	res2 := httptest.NewRecorder()
	handler.ServeHTTP(res2, httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/scans/new?rerun=missing", nil))
	if res2.Code != http.StatusBadRequest {
		t.Fatalf("rerun with unknown run must still 400, got %d", res2.Code)
	}
}

func TestTargetSetImportEmptyReplacesWithEmpty(t *testing.T) {
	handler, projectID, dbPath := seedProjectWithTargetSet(t)
	form := url.Values{"targets": {"   \n,  \n"}}
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/target-set", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", res.Code)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}
	defer s.Close()
	ts, err := s.GetTargetSet(projectID)
	if err != nil {
		t.Fatalf("GetTargetSet returned error: %v", err)
	}
	if len(ts.TargetList()) != 0 {
		t.Fatalf("empty import must clear the target set, got %v", ts.TargetList())
	}
}

func TestProjectDeleteRemovesTargetSet(t *testing.T) {
	handler, projectID, dbPath := seedProjectWithTargetSet(t)
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}
	defer s.Close()
	if err := s.DeleteProjectCascade(projectID); err != nil {
		t.Fatalf("DeleteProjectCascade returned error: %v", err)
	}
	if _, err := s.GetTargetSet(projectID); err == nil {
		t.Fatalf("target set must be gone after project delete")
	}
	_ = handler
}
