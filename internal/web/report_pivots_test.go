package web

import (
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// Ticket 05: the report page exposes host/port/service/product/vulnerability
// pivots and a host x service matrix as deep-linkable JSON, and the product
// filter narrows the rendered assets.

func seedPivotRun(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "pivot-run", Target: "192.0.2.0/24", Ports: "443,80", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	fps := []fingerprint.ServiceFingerprint{
		{IP: "192.0.2.10", Port: 443, Protocol: "tcp", Service: "https", Product: "nginx"},
		{IP: "192.0.2.10", Port: 80, Protocol: "tcp", Service: "http", Product: "nginx"},
		{IP: "192.0.2.11", Port: 22, Protocol: "tcp", Service: "ssh", Product: "OpenSSH"},
		{IP: "192.0.2.12", Port: 6379, Protocol: "tcp", Service: "redis", Product: ""},
	}
	for _, fp := range fps {
		if err := scanStore.SaveFingerprint("pivot-run", fp); err != nil {
			t.Fatalf("SaveFingerprint returned error: %v", err)
		}
	}
	if err := scanStore.SaveFinding("pivot-run", report.Finding{IP: "192.0.2.12", Port: 6379, Source: "nuclei", ID: "redis-logins", Severity: "high", Summary: "Redis default login", Target: "192.0.2.12:6379"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	return handler, "pivot-run"
}

func TestReportPageExposesPivotsAndMatrix(t *testing.T) {
	handler, runID := seedPivotRun(t)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/"+runID, nil))
	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	body := res.Body.String()
	unescaped := html.UnescapeString(body)
	// Pivot facets JSON is emitted for the Vue mount point.
	if !strings.Contains(body, `data-pivot-facets=`) {
		t.Fatalf("expected pivot facets data attribute on the report mount point")
	}
	if !strings.Contains(body, `data-service-matrix=`) {
		t.Fatalf("expected service matrix data attribute on the report mount point")
	}
	// Each dimension appears in the facet payload (assert on the unescaped JSON).
	for _, want := range []string{`"dimension":"host"`, `"dimension":"port"`, `"dimension":"service"`, `"dimension":"product"`, `"dimension":"vulnerability"`} {
		if !strings.Contains(unescaped, want) {
			t.Fatalf("expected pivot dimension %s in body", want)
		}
	}
	// Dedup: nginx product counts two endpoints (443 + 80), not duplicated.
	if !strings.Contains(unescaped, `"dimension":"product","raw_value":"nginx","label":"nginx","count":2`) {
		t.Fatalf("expected nginx product facet with deduped count 2")
	}
}

func TestReportPageProductFilter(t *testing.T) {
	handler, runID := seedPivotRun(t)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/"+runID+"?view=ports&product=nginx", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	body := res.Body.String()
	// nginx endpoints (192.0.2.10:443 and :80) remain; redis/ssh excluded.
	if !strings.Contains(body, "192.0.2.10") {
		t.Fatalf("expected nginx host 192.0.2.10 under product filter")
	}
	if strings.Contains(body, "192.0.2.11") || strings.Contains(body, "192.0.2.12") {
		t.Fatalf("product=nginx filter should exclude non-nginx hosts")
	}
}
