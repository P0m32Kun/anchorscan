package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func TestOpenConfiguresSQLiteForConcurrentScanWrites(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	var timeout int
	if err := store.db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatalf("busy_timeout query returned error: %v", err)
	}
	if timeout < 5000 {
		t.Fatalf("expected busy_timeout >= 5000, got %d", timeout)
	}

	var journalMode string
	if err := store.db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode query returned error: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("expected WAL journal mode, got %q", journalMode)
	}
}

func TestSQLiteStoreSavesAndListsFingerprints(t *testing.T) {
	db := t.TempDir() + "/scan.db"
	store, err := Open(db)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	fp := fingerprint.ServiceFingerprint{
		IP:         "192.168.1.10",
		Port:       6379,
		Service:    "redis",
		Product:    "redis",
		Version:    "7.2.0",
		Normalized: "redis",
	}
	if err := store.SaveFingerprint("run-1", fp); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}

	got, err := store.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(got) != 1 || got[0].Port != 6379 || got[0].Version != "7.2.0" {
		t.Fatalf("unexpected rows: %#v", got)
	}
}

func TestSQLiteStoreSavesAndListsFindings(t *testing.T) {
	db := t.TempDir() + "/scan.db"
	store, err := Open(db)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	finding := report.Finding{
		IP:       "192.168.1.10",
		Port:     6379,
		Source:   "nuclei",
		ID:       "redis-detect",
		Severity: "info",
		Summary:  "Redis Detect",
		Target:   "192.168.1.10:6379",
		Output:   "matched",
	}
	if err := store.SaveFinding("run-1", finding); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}

	got, err := store.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "redis-detect" {
		t.Fatalf("unexpected findings: %#v", got)
	}
}

func TestStoreProjectCRUD(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	project := Project{
		ID:             "p1",
		Name:           "Local Lab",
		Description:    "Tomcat Redis",
		DefaultTargets: "127.0.0.1",
		DefaultPorts:   "8080,6379",
		ExcludeTargets: "127.0.0.2",
		ExcludePorts:   "22,3306",
		DefaultProfile: "normal",
		CreatedAt:      time.Unix(1, 0),
		UpdatedAt:      time.Unix(1, 0),
	}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Local Lab" {
		t.Fatalf("unexpected projects: %#v", projects)
	}

	project.Name = "Updated Lab"
	project.UpdatedAt = time.Unix(2, 0)
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("SaveProject update returned error: %v", err)
	}

	got, err := store.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	if got.Name != "Updated Lab" {
		t.Fatalf("project name mismatch: %#v", got)
	}
	if got.ExcludeTargets != "127.0.0.2" || got.ExcludePorts != "22,3306" {
		t.Fatalf("project exclude fields mismatch: %#v", got)
	}

	if err := store.DeleteProject("p1"); err != nil {
		t.Fatalf("DeleteProject returned error: %v", err)
	}

	projects, err = store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected no projects, got %#v", projects)
	}
}

func TestDeleteProjectCascadeRemovesRunsAndArtifacts(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	project := Project{
		ID:             "p1",
		Name:           "Local Lab",
		DefaultTargets: "127.0.0.1",
		DefaultPorts:   "6379",
		DefaultProfile: "normal",
		CreatedAt:      time.Unix(1, 0),
		UpdatedAt:      time.Unix(1, 0),
	}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	run := ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "6379",
		Profile:        "normal",
		Status:         "completed",
		ConfigSnapshot: "profile: normal",
		StartedAt:      time.Unix(2, 0),
		FinishedAt:     time.Unix(3, 0),
	}
	if err := store.SaveScanRun(run); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := store.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := store.SaveFinding("run-1", report.Finding{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	if err := store.AppendScanEvent(ScanEvent{RunID: "run-1", Time: time.Unix(4, 0), Level: "info", Stage: "nmap", Message: "done"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}

	if err := store.DeleteProjectCascade("p1"); err != nil {
		t.Fatalf("DeleteProjectCascade returned error: %v", err)
	}

	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected no projects, got %#v", projects)
	}
	runs, err := store.ListProjectScanRuns("p1", 10)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no runs, got %#v", runs)
	}
	fps, err := store.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 0 {
		t.Fatalf("expected no fingerprints, got %#v", fps)
	}
	findings, err := store.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
	events, err := store.ListScanEvents("run-1", 10)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %#v", events)
	}
}

func TestStoreScanRunPersistsArtifactDir(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	wantArtifactDir := filepath.Join(dir, "artifacts", "run-1")
	run := ScanRun{
		RunID:       "run-1",
		ProjectID:   "project-1",
		Target:      "127.0.0.1",
		Ports:       "80",
		Profile:     "normal",
		Status:      "completed",
		StartedAt:   time.Unix(1, 0),
		FinishedAt:  time.Unix(2, 0),
		ArtifactDir: wantArtifactDir,
	}
	if err := store.SaveScanRun(run); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	got, err := store.GetScanRun("run-1")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if got.ArtifactDir != wantArtifactDir {
		t.Fatalf("artifact dir mismatch: got %q want %q", got.ArtifactDir, wantArtifactDir)
	}

	dirs, err := store.ListProjectArtifactDirs("project-1")
	if err != nil {
		t.Fatalf("ListProjectArtifactDirs returned error: %v", err)
	}
	if len(dirs) != 1 || dirs[0] != wantArtifactDir {
		t.Fatalf("artifact dirs mismatch: got %#v want %#v", dirs, []string{wantArtifactDir})
	}
}

func TestProjectHasRunningRuns(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if err := store.SaveScanRun(ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "80",
		Profile:        "normal",
		Status:         "running",
		ConfigSnapshot: "profile: normal",
		StartedAt:      time.Unix(1, 0),
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	hasRunning, err := store.ProjectHasRunningRuns("p1")
	if err != nil {
		t.Fatalf("ProjectHasRunningRuns returned error: %v", err)
	}
	if !hasRunning {
		t.Fatalf("expected running run for project p1")
	}

	hasRunning, err = store.ProjectHasRunningRuns("p2")
	if err != nil {
		t.Fatalf("ProjectHasRunningRuns returned error: %v", err)
	}
	if hasRunning {
		t.Fatalf("expected no running runs for project p2")
	}
}

func TestStoreScanRunsAndEvents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	run := ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "8080,6379",
		Profile:        "normal",
		Status:         "queued",
		ConfigSnapshot: "profile: normal",
		StartedAt:      time.Unix(1, 0),
	}
	if err := store.SaveScanRun(run); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := store.UpdateScanRunStatus("run-1", "running", "", time.Time{}); err != nil {
		t.Fatalf("UpdateScanRunStatus returned error: %v", err)
	}
	if err := store.AppendScanEvent(ScanEvent{
		RunID:   "run-1",
		Time:    time.Unix(2, 0),
		Level:   "info",
		Stage:   "nmap",
		Message: "nmap still running",
	}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}

	runs, err := store.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "running" {
		t.Fatalf("unexpected runs: %#v", runs)
	}

	gotRun, err := store.GetScanRun("run-1")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if gotRun.ProjectID != "p1" || gotRun.Status != "running" {
		t.Fatalf("unexpected run: %#v", gotRun)
	}

	projectRuns, err := store.ListProjectScanRuns("p1", 10)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(projectRuns) != 1 || projectRuns[0].RunID != "run-1" {
		t.Fatalf("unexpected project runs: %#v", projectRuns)
	}

	events, err := store.ListScanEvents("run-1", 10)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].Stage != "nmap" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestStoreListsScanRunsByChronologicalStartedAtWithMixedPrecision(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	projectID := "p1"
	earlier := ScanRun{
		RunID:          "run-whole-second",
		ProjectID:      projectID,
		Target:         "127.0.0.1",
		Ports:          "80",
		Profile:        "normal",
		Status:         "queued",
		ConfigSnapshot: "profile: normal",
		StartedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	later := ScanRun{
		RunID:          "run-fractional",
		ProjectID:      projectID,
		Target:         "127.0.0.2",
		Ports:          "443",
		Profile:        "normal",
		Status:         "queued",
		ConfigSnapshot: "profile: normal",
		StartedAt:      time.Date(2026, 1, 1, 0, 0, 0, 100000000, time.UTC),
	}

	if err := store.SaveScanRun(earlier); err != nil {
		t.Fatalf("SaveScanRun earlier returned error: %v", err)
	}
	if err := store.SaveScanRun(later); err != nil {
		t.Fatalf("SaveScanRun later returned error: %v", err)
	}

	runs, err := store.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %#v", runs)
	}
	if runs[0].RunID != later.RunID || runs[1].RunID != earlier.RunID {
		t.Fatalf("unexpected global run order: %#v", runs)
	}

	projectRuns, err := store.ListProjectScanRuns(projectID, 10)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(projectRuns) != 2 {
		t.Fatalf("expected 2 project runs, got %#v", projectRuns)
	}
	if projectRuns[0].RunID != later.RunID || projectRuns[1].RunID != earlier.RunID {
		t.Fatalf("unexpected project run order: %#v", projectRuns)
	}
}
