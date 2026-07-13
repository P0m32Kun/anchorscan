package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/version"
)

func TestHomePageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", ProjectID: "p1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(2, 0), FinishedAt: time.Unix(3, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "missing.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "AnchorScan") || !strings.Contains(res.Body.String(), "Local Lab") {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "项目") || !strings.Contains(res.Body.String(), "最近扫描") {
		t.Fatalf("expected chinese ui copy: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `/projects/p1`) || !strings.Contains(res.Body.String(), `/runs/run-1`) {
		t.Fatalf("expected home links in body: %s", res.Body.String())
	}
	// footer version is rendered from the version package, not hardcoded
	if !strings.Contains(res.Body.String(), "AnchorScan Console v"+version.Version) {
		t.Fatalf("expected versioned footer in body: %s", res.Body.String())
	}
}

func TestCreateProjectFromWeb(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{
		"name":            "Local Lab",
		"description":     "Test",
		"default_targets": "127.0.0.1\n127.0.0.2",
		"default_ports":   "top1000",
		"exclude_targets": "127.0.0.2",
		"exclude_ports":   "22,3306",
		"default_profile": "normal",
	} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField returned error: %v", err)
		}
	}
	fileWriter, err := writer.CreateFormFile("targets_file", "targets.txt")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte("192.168.1.0/24\n")); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d", res.Code)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Local Lab" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
	if !strings.Contains(projects[0].DefaultTargets, "192.168.1.0/24") || projects[0].ExcludeTargets != "127.0.0.2" || projects[0].ExcludePorts != "22,3306" {
		t.Fatalf("unexpected saved project fields: %#v", projects[0])
	}
}

func TestDeleteProjectRemovesManagedFilesAndDatabaseRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{
		ID:             "p1",
		Name:           "Local Lab",
		DefaultTargets: "127.0.0.1",
		DefaultPorts:   "6379",
		DefaultProfile: "normal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "6379",
		Profile:        "normal",
		Status:         "completed",
		ArtifactDir:    filepath.Join(dir, "artifacts", "run-1"),
		ConfigSnapshot: "profile: normal",
		StartedAt:      now,
		FinishedAt:     now.Add(time.Second),
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := scanStore.SaveFinding("run-1", report.Finding{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	if err := scanStore.AppendScanEvent(store.ScanEvent{RunID: "run-1", Time: now.Add(2 * time.Second), Level: "info", Stage: "nmap", Message: "done"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}
	reportPath := filepath.Join(dir, "projects", "p1", "runs", "run-1", "report.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	artifactPath := filepath.Join(dir, "artifacts", "run-1", "rustscan-127.0.0.1.txt")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("127.0.0.1 -> [6379]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("_method=delete")
	req := httptest.NewRequest(http.MethodPost, "/projects/p1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}

	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected no projects, got %#v", projects)
	}
	runs, err := scanStore.ListProjectScanRuns("p1", 10)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no runs, got %#v", runs)
	}
	fps, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 0 {
		t.Fatalf("expected no fingerprints, got %#v", fps)
	}
	findings, err := scanStore.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
	events, err := scanStore.ListScanEvents("run-1", 10)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %#v", events)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "p1")); !os.IsNotExist(err) {
		t.Fatalf("expected project dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "artifacts", "run-1")); !os.IsNotExist(err) {
		t.Fatalf("expected artifact dir to be removed, got err=%v", err)
	}
}

func TestDeleteProjectRejectsRunningRuns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{
		ID:             "p1",
		Name:           "Local Lab",
		DefaultTargets: "127.0.0.1",
		DefaultPorts:   "6379",
		DefaultProfile: "normal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "6379",
		Profile:        "normal",
		Status:         "running",
		ConfigSnapshot: "profile: normal",
		StartedAt:      now,
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("_method=delete")
	req := httptest.NewRequest(http.MethodPost, "/projects/p1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected project to remain, got %#v", projects)
	}
}
