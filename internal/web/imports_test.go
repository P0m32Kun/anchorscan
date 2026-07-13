package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestImportNmapFormRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/import/nmap", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "导入 Nmap XML") || !strings.Contains(body, `name="xml_file"`) {
		t.Fatalf("expected import form, got: %s", body)
	}
	if !strings.Contains(body, `name="project_id"`) || !strings.Contains(body, "Local Lab") {
		t.Fatalf("expected project selector with project name, got: %s", body)
	}
}

func TestImportNmapRunRedirectsToRun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "scan.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte(`<nmaprun>
  <host>
    <address addr="10.0.0.53"/>
    <ports>
      <port protocol="tcp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
    </ports>
  </host>
</nmaprun>`)); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", res.Code)
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/runs/") {
		t.Fatalf("expected redirect to /runs/, got %q", location)
	}

	// 验证 DB 有完成态 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("expected one completed run, got %d err=%v", len(runs), err)
	}
}

func TestImportNmapRunEmptyFileRendersFormError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "empty.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte("")); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 (form re-render), got %d", res.Code)
	}
	pageBody := res.Body.String()
	if !strings.Contains(pageBody, "XML 文件为空") {
		t.Fatalf("expected error banner, got: %s", pageBody)
	}

	// 验证 DB 无新增 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}

func TestImportNmapRunNonNmaprunRendersFormError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "foo.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte(`<foo><bar/></foo>`)); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %s", res.Body.String())
	}

	// 验证 DB 无新增 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}
