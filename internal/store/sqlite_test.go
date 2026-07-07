package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

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
		Normalized: "redis",
	}
	if err := store.SaveFingerprint("run-1", fp); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}

	got, err := store.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(got) != 1 || got[0].Port != 6379 {
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
