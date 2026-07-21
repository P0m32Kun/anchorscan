package web

import (
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

	form := "name=Local+Lab&description=Test&client_unit=Client+A&test_object=internal&start_date=2026-07-01&end_date=2026-07-15&testers=Alice%2C+Bob"
	req := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/projects/") {
		t.Fatalf("expected redirect to project detail, got %q", location)
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
	project := projects[0]
	if project.ClientUnit != "Client A" || project.TestObject != "internal" || project.Testers != "Alice, Bob" {
		t.Fatalf("unexpected project metadata: %#v", project)
	}
	if project.ReportTitle != "Client A内网安全检查报告" {
		t.Fatalf("expected default report title, got %q", project.ReportTitle)
	}
	if project.DefaultTargets != "" || project.DefaultPorts != "" || project.DefaultProfile != "" {
		t.Fatalf("legacy scan default fields should not be set for new project: %#v", project)
	}

	zones, err := scanStore.ListProjectZones(project.ID)
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 3 {
		t.Fatalf("expected 3 default zones, got %#v", zones)
	}
	for i, want := range []string{"I区", "II区", "III区"} {
		if zones[i].Name != want {
			t.Fatalf("zone %d = %q, want %q", i, zones[i].Name, want)
		}
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
	if _, err := scanStore.AcquireRunLease("run-1", "owner-1", time.Now(), time.Minute); err != nil {
		t.Fatalf("AcquireRunLease returned error: %v", err)
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

func TestProjectDetailRendersMetadataAndZones(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{
		ID:          "p1",
		Name:        "甘肃任务",
		ClientUnit:  "甘肃电力",
		ReportTitle: "甘肃电力内网安全检查报告",
		TestObject:  "信息内网",
		StartDate:   "2026-07-01",
		EndDate:     "2026-07-15",
		Testers:     "张三, 李四",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := scanStore.CreateProjectZone(store.ProjectZone{ProjectID: "p1", ZoneID: "dmz", Name: "DMZ", SortOrder: 3}); err != nil {
		t.Fatalf("CreateProjectZone returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"甘肃任务", "甘肃电力", "甘肃电力内网安全检查报告", "信息内网", "2026-07-01", "2026-07-15", "张三, 李四", "I区", "II区", "III区", "DMZ"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body, got %s", want, body)
		}
	}
}

func TestAddAndDeleteProjectZone(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	addReq := httptest.NewRequest(http.MethodPost, "/projects/p1/zones", strings.NewReader("name=DMZ"))
	addReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRes := httptest.NewRecorder()
	handler.ServeHTTP(addRes, addReq)
	if addRes.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect after adding zone, got %d body=%s", addRes.Code, addRes.Body.String())
	}

	zones, err := scanStore.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	var dmzID string
	for _, z := range zones {
		if z.Name == "DMZ" {
			dmzID = z.ZoneID
			break
		}
	}
	if dmzID == "" {
		t.Fatalf("DMZ zone not created: %#v", zones)
	}
	if len(zones) != 4 || zones[3].Name != "DMZ" {
		t.Fatalf("unexpected zone order: %#v", zones)
	}

	delReq := httptest.NewRequest(http.MethodPost, "/projects/p1/zones/"+dmzID, strings.NewReader("_method=delete"))
	delReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	delRes := httptest.NewRecorder()
	handler.ServeHTTP(delRes, delReq)
	if delRes.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect after deleting zone, got %d body=%s", delRes.Code, delRes.Body.String())
	}
	zones, err = scanStore.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 3 {
		t.Fatalf("expected 3 zones after deletion, got %#v", zones)
	}
}

func TestDeleteProjectZoneRejectsWhenRunsExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:      "run-1",
		ProjectID:  "p1",
		ZoneID:     "I",
		Target:     "127.0.0.1",
		Ports:      "80",
		Profile:    "normal",
		Status:     "completed",
		StartedAt:  now,
		FinishedAt: now,
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/projects/p1/zones/I", strings.NewReader("_method=delete"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", res.Code, res.Body.String())
	}
	zones, err := scanStore.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 3 {
		t.Fatalf("expected zone not deleted, got %#v", zones)
	}
}

func TestOldProjectFieldsRemainReadable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	oldProject := store.Project{
		ID:             "legacy",
		Name:           "Legacy",
		DefaultTargets: "192.168.1.1",
		DefaultPorts:   "8080",
		ExcludeTargets: "192.168.1.2",
		ExcludePorts:   "22",
		DefaultProfile: "normal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := scanStore.SaveProject(oldProject); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/legacy/edit", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, "Legacy") {
		t.Fatalf("expected old project name in edit form, got %s", body)
	}
	// Legacy scan default fields are no longer editable, but must remain stored.
	for _, mustNotContain := range []string{"default_targets", "default_ports", "exclude_targets"} {
		if strings.Contains(body, mustNotContain) {
			t.Fatalf("edit form should not contain legacy field %q", mustNotContain)
		}
	}

	update := "name=Legacy+Updated\u0026description=\u0026client_unit=Unit\u0026report_title=\u0026test_object=\u0026start_date=\u0026end_date=\u0026testers="
	updateReq := httptest.NewRequest(http.MethodPost, "/projects/legacy", strings.NewReader(update))
	updateReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updateRes := httptest.NewRecorder()
	handler.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect after update, got %d body=%s", updateRes.Code, updateRes.Body.String())
	}
	updated, err := scanStore.GetProject("legacy")
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	if updated.DefaultTargets != "192.168.1.1" || updated.DefaultPorts != "8080" || updated.DefaultProfile != "normal" {
		t.Fatalf("old default fields overwritten: %#v", updated)
	}
	if updated.ClientUnit != "Unit" {
		t.Fatalf("new metadata not set: %#v", updated)
	}
}
