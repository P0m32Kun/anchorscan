package web

import (
	"encoding/json"
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
)

func TestToolRunDetailShowsReturnAndEvidenceLinks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:           "tool-nmap-20260701-120000.000000000",
		ProjectID:       "p1",
		ZoneID:          "I",
		Kind:            "tool",
		Profile:         "tool:nmap",
		Target:          "192.0.2.10",
		Status:          "completed",
		ConfigSnapshot:  `{"tool":"nmap","mode":"alive","target":"192.0.2.10","verification_id":"v1"}`,
		StartedAt:       time.Unix(1, 0),
		FinishedAt:      time.Unix(2, 0),
		IncludeInReport: false,
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/runs/tool-nmap-20260701-120000.000000000?return=/projects/p1/workbench", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{
		"返回工作台",
		"上传证据",
		"/projects/p1/workbench",
		"/projects/p1/verifications/v1/evidence",
		"复制输出",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body: %s", want, body)
		}
	}
}

func TestRunsPageShowsProjectID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{
		ID:        "p1",
		Name:      "Local Lab",
		CreatedAt: time.Unix(1, 0),
		UpdatedAt: time.Unix(1, 0),
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:     "run-1",
		ProjectID: "p1",
		Target:    "127.0.0.1",
		Ports:     "80",
		Profile:   "normal",
		Status:    "completed",
		StartedAt: time.Unix(1, 0),
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/runs", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `/projects/p1`) || !strings.Contains(body, "Local Lab") {
		t.Fatalf("expected project name and link in runs page: %s", body)
	}
	if strings.Contains(body, `>p1</a>`) {
		t.Fatalf("expected project name instead of project ID: %s", body)
	}
}

func TestDeleteScanRunRemovesManagedFilesAndDatabaseRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
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
	artifactPath := filepath.Join(dir, "artifacts", "run-1", "nmap-127.0.0.1.xml")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("<nmaprun/>"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("_method=delete")
	req := httptest.NewRequest(http.MethodPost, "/runs/run-1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}

	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
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
	if _, err := os.Stat(filepath.Join(dir, "projects", "p1", "runs", "run-1")); !os.IsNotExist(err) {
		t.Fatalf("expected run dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "artifacts", "run-1")); !os.IsNotExist(err) {
		t.Fatalf("expected artifact dir to be removed, got err=%v", err)
	}
}

func TestListScanRunsFiltersByScanKind(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "scan-1", ProjectID: "p1", Kind: "scan", Status: "completed", StartedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "tool-1", ProjectID: "p1", Kind: "tool", Status: "completed", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "scan-1" {
		t.Fatalf("expected only scan runs, got %#v", runs)
	}

	projectRuns, err := scanStore.ListProjectScanRuns("p1", 10)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(projectRuns) != 1 || projectRuns[0].RunID != "scan-1" {
		t.Fatalf("expected only scan runs for project, got %#v", projectRuns)
	}
}

func TestRunsPageListsOnlyToolRunsWhenRequested(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	for _, run := range []store.ScanRun{
		{RunID: "scan-visible-only-on-default", Kind: "scan", Profile: "normal", Status: "completed", StartedAt: time.Unix(2, 0)},
		{RunID: "tool-visible-in-history", Kind: "tool", Profile: "tool:nuclei", Status: "completed", StartedAt: time.Unix(1, 0)},
	} {
		if err := scanStore.SaveScanRun(run); err != nil {
			t.Fatalf("SaveScanRun returned error: %v", err)
		}
	}
	if err := scanStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/runs?kind=tool", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, "工具运行历史") || !strings.Contains(body, "tool-visible-in-history") || strings.Contains(body, "scan-visible-only-on-default") {
		t.Fatalf("unexpected tool history page: %s", body)
	}
}

func TestToolPagesLinkToToolRunHistory(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	for _, path := range []string{"/tools/new", "/tools/nuclei"} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `href="/runs?kind=tool"`) {
			t.Fatalf("%s does not link to tool history: %d %s", path, res.Code, res.Body.String())
		}
	}
}

func TestRunEventsAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "running", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.AppendScanEvent(store.ScanEvent{RunID: "run-1", Time: time.Unix(2, 0), Level: "info", Stage: "nmap", Message: "still running"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/runs/run-1/events", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "still running") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
	var events []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &events); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(events) != 1 || events[0]["time"] == nil || events[0]["stage"] == nil || events[0]["message"] == nil {
		t.Fatalf("unexpected json fields: %#v", events)
	}
}

func TestRunEventsAPIEmptyListReturnsJSONArray(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Status: "completed", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/runs/run-1/events", nil))
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("empty events response: %d %s", res.Code, res.Body.String())
	}
}

func TestRunStatusAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/runs/run-1/status", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Status          string         `json:"status"`
		DetectionChecks map[string]int `json:"detection_checks"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if body.Status != "completed" || body.DetectionChecks["completed"] != 0 || body.DetectionChecks["interrupted"] != 0 {
		t.Fatalf("unexpected status response: %#v", body)
	}
}

func TestRunPageLoadsStatusPolling(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Status: "running", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/runs/run-1", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	appScript := strings.Index(body, `<script src="/static/app.js" defer></script>`)
	runStatusScript := strings.Index(body, `<script src="/static/run-status.js" defer></script>`)
	if appScript == -1 || runStatusScript == -1 || appScript > runStatusScript {
		t.Fatalf("expected app.js before run-status.js: %s", body)
	}
}

func TestInterruptedRunShowsHistoryAndPrefilledRerunFormWithoutStarting(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		ZoneID:         "I",
		Target:         "198.51.100.10",
		Ports:          "80,443",
		Profile:        "normal",
		Status:         "interrupted",
		ConfigSnapshot: `{"zone_id":"I","target":"198.51.100.10","exclude_targets":"198.51.100.20","ports":"80,443","exclude_ports":"22","profile":"fast","rustscan_args":"--ulimit 5000","nmap_args":"-sV"}`,
		StartedAt:      time.Unix(1, 0),
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "198.51.100.10", Port: 443, Service: "https"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/runs/run-1", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "status-interrupted") || !strings.Contains(detail.Body.String(), "/projects/p1/scans/new?rerun=run-1") {
		t.Fatalf("unexpected run detail: %d %s", detail.Code, detail.Body.String())
	}

	rerun := httptest.NewRecorder()
	handler.ServeHTTP(rerun, httptest.NewRequest(http.MethodGet, "/projects/p1/scans/new?rerun=run-1", nil))
	if rerun.Code != http.StatusOK {
		t.Fatalf("rerun page status: %d %s", rerun.Code, rerun.Body.String())
	}
	body := rerun.Body.String()
	for _, want := range []string{`data-scan-create-props=`, "isRerun", "zone_id", "198.51.100.10", "exclude_targets", "198.51.100.20", "ports", "80,443", "exclude_ports", "22", "fast", "--ulimit 5000", "-sV"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rerun page missing %q: %s", want, body)
		}
	}
	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run-1" {
		t.Fatalf("opening rerun page changed history: %#v", runs)
	}
}

func TestCompletedWithErrorsRunCanBeRerun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatal(err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-errors", ProjectID: "p1", ZoneID: "I", Target: "198.51.100.10", Ports: "443", Profile: "normal", Status: "completed_with_errors", ConfigSnapshot: `{"target":"198.51.100.10","ports":"443","profile":"normal"}`, StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/runs/run-errors", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "/projects/p1/scans/new?rerun=run-errors") {
		t.Fatalf("run detail = %d %s", detail.Code, detail.Body.String())
	}
	rerun := httptest.NewRecorder()
	handler.ServeHTTP(rerun, httptest.NewRequest(http.MethodGet, "/projects/p1/scans/new?rerun=run-errors", nil))
	if rerun.Code != http.StatusOK || !strings.Contains(rerun.Body.String(), "198.51.100.10") {
		t.Fatalf("rerun page = %d %s", rerun.Code, rerun.Body.String())
	}
}

func TestInterruptedLegacyProjectScanPrefillsPersistedFields(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "legacy", ProjectID: "p1", ZoneID: "I", Target: "198.51.100.20", Ports: "443", Profile: "normal", Status: "interrupted", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/scans/new?rerun=legacy", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `data-scan-create-props=`) || !strings.Contains(res.Body.String(), "198.51.100.20") || !strings.Contains(res.Body.String(), "443") || !strings.Contains(res.Body.String(), "normal") {
		t.Fatalf("legacy rerun page: %d %s", res.Code, res.Body.String())
	}
}
