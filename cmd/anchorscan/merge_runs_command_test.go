package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func seedMergeRunsFixtures(t *testing.T, dbPath string) {
	t.Helper()
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	// Four source projects matching the Gansu shape.
	for _, p := range []store.Project{
		{ID: "gansu-i-a", Name: "甘肃I区A", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)},
		{ID: "gansu-i-b", Name: "甘肃I区B", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)},
		{ID: "gansu-i-c", Name: "甘肃I区C", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)},
		{ID: "gansu-iii", Name: "甘肃III区", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)},
	} {
		if err := st.SaveProject(p); err != nil {
			t.Fatalf("SaveProject returned error: %v", err)
		}
	}
	// Target task project with I/III zones.
	if err := st.SaveProject(store.Project{ID: "gansu-task", Name: "甘肃电力内网安全检查任务", ClientUnit: "甘肃电力", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject target returned error: %v", err)
	}
	if err := st.CreateDefaultProjectZones("gansu-task"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	runs := []struct {
		runID, projectID string
	}{
		{"run-i-a", "gansu-i-a"},
		{"run-i-b", "gansu-i-b"},
		{"run-i-c", "gansu-i-c"},
		{"run-iii", "gansu-iii"},
	}
	for _, r := range runs {
		if err := st.SaveScanRun(store.ScanRun{
			RunID: r.runID, ProjectID: r.projectID, Target: "10.0.0.1", Ports: "80", Profile: "normal",
			Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0),
		}); err != nil {
			t.Fatalf("SaveScanRun returned error: %v", err)
		}
		// Persist a finding per run so aggregate counts can be compared.
		if err := st.SaveFinding(r.runID, report.Finding{IP: "10.0.0.1", Port: 80, Source: "nuclei", ID: r.runID, Severity: "high", Summary: r.runID}); err != nil {
			t.Fatalf("SaveFinding returned error: %v", err)
		}
	}
}

func TestMergeRunsDryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scans.sqlite")
	seedMergeRunsFixtures(t, dbPath)

	var stdout, stderr bytes.Buffer
	err := runMergeRuns([]string{
		"--db", dbPath,
		"--to-project", "gansu-task",
		"--run", "run-i-a@I",
		"--run", "run-i-b@I",
		"--run", "run-i-c@I",
		"--run", "run-iii@III",
	}, &stdout, &stderr, cliDeps{})
	if err != nil {
		t.Fatalf("runMergeRuns returned error: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"dry-run only", "run run-i-a", "run run-iii", "zone:     -> III", "target project: gansu-task"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in dry-run output: %s", want, out)
		}
	}

	// Nothing mutated: the four runs still belong to their original projects.
	st, _ := store.Open(dbPath)
	defer st.Close()
	for _, r := range []struct{ runID, want string }{
		{"run-i-a", "gansu-i-a"},
		{"run-iii", "gansu-iii"},
	} {
		run, err := st.GetScanRun(r.runID)
		if err != nil {
			t.Fatalf("GetScanRun returned error: %v", err)
		}
		if run.ProjectID != r.want {
			t.Fatalf("dry-run mutated run %s project: got %s want %s", r.runID, run.ProjectID, r.want)
		}
	}
}

func TestMergeRunsApplyReassignsRuns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scans.sqlite")
	seedMergeRunsFixtures(t, dbPath)
	backupDir := filepath.Join(dir, "backup")

	var stdout, stderr bytes.Buffer
	err := runMergeRuns([]string{
		"--db", dbPath,
		"--to-project", "gansu-task",
		"--run", "run-i-a@I",
		"--run", "run-i-b@I",
		"--run", "run-i-c@I",
		"--run", "run-iii@III",
		"--apply",
		"--backup-dir", backupDir,
		"--delete-empty-source-projects",
	}, &stdout, &stderr, cliDeps{})
	if err != nil {
		t.Fatalf("runMergeRuns returned error: %v", err)
	}

	st, _ := store.Open(dbPath)
	defer st.Close()
	targetRuns, err := st.ListProjectScanRuns("gansu-task", 100)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(targetRuns) != 4 {
		t.Fatalf("expected 4 runs in target project, got %d", len(targetRuns))
	}
	zoneByRun := map[string]string{}
	for _, r := range targetRuns {
		zoneByRun[r.RunID] = r.ZoneID
	}
	if zoneByRun["run-i-a"] != "I" || zoneByRun["run-iii"] != "III" {
		t.Fatalf("unexpected zone assignment: %#v", zoneByRun)
	}

	// Backup database exists.
	if _, err := store.Open(filepath.Join(backupDir, "scans.sqlite")); err != nil {
		t.Fatalf("backup db not readable: %v", err)
	}

	// Source projects deleted because they are now empty.
	for _, projectID := range []string{"gansu-i-a", "gansu-i-b", "gansu-i-c", "gansu-iii"} {
		if _, err := st.GetProject(projectID); err == nil {
			t.Fatalf("expected source project %s to be deleted", projectID)
		}
	}
}

func TestMergeRunsApplyRequiresBackupDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scans.sqlite")
	seedMergeRunsFixtures(t, dbPath)

	var stdout, stderr bytes.Buffer
	err := runMergeRuns([]string{
		"--db", dbPath,
		"--to-project", "gansu-task",
		"--run", "run-i-a@I",
		"--apply",
	}, &stdout, &stderr, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "backup-dir") {
		t.Fatalf("expected backup-dir error, got %v", err)
	}
}
