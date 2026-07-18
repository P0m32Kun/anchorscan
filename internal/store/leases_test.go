package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func TestRunLeaseRejectsFreshOwnerAndAllowsExpiry(t *testing.T) {
	scanStore, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()
	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	if _, err := scanStore.AcquireRunLease("run-1", "owner-1", now, time.Minute); err != nil {
		t.Fatal(err)
	}
	held, err := scanStore.AcquireRunLease("run-2", "owner-2", now.Add(30*time.Second), time.Minute)
	if !errors.Is(err, ErrRunLeaseHeld) || held.RunID != "run-1" {
		t.Fatalf("expected fresh lease conflict, got %#v %v", held, err)
	}
	if _, err := scanStore.AcquireRunLease("run-2", "owner-2", now.Add(2*time.Minute), time.Minute); err != nil {
		t.Fatal(err)
	}
	if released, err := scanStore.ReleaseRunLease("run-1", "owner-1"); err != nil || released {
		t.Fatalf("old owner release = %t, %v", released, err)
	}
}

func TestRunLeaseDoesNotExpireFreshFractionalSecondHeartbeat(t *testing.T) {
	scanStore, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()
	now := time.Date(2026, 7, 18, 0, 0, 0, 100_000_000, time.UTC)
	if _, err := scanStore.AcquireRunLease("run-1", "owner-1", now, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := scanStore.AcquireRunLease("run-2", "owner-2", now.Add(900*time.Millisecond), time.Second); !errors.Is(err, ErrRunLeaseHeld) {
		t.Fatalf("expected fresh fractional heartbeat to hold lease, got %v", err)
	}
}

func TestRunLeaseHonorsLegacyHeartbeatDuringUpgrade(t *testing.T) {
	scanStore, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()
	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	if _, err := scanStore.AcquireRunLease("run-1", "owner-1", now, time.Second); err != nil {
		t.Fatal(err)
	}
	legacyHeartbeat := now.Add(1500 * time.Millisecond)
	if _, err := scanStore.db.Exec(`UPDATE run_leases SET heartbeat_at = ? WHERE scope = ?`, legacyHeartbeat.Format(time.RFC3339Nano), globalRunLeaseScope); err != nil {
		t.Fatal(err)
	}
	if _, err := scanStore.AcquireRunLease("run-2", "owner-2", now.Add(2*time.Second), time.Second); !errors.Is(err, ErrRunLeaseHeld) {
		t.Fatalf("expected legacy heartbeat to retain lease, got %v", err)
	}
}

func TestReconcileInterruptedRunsKeepsFreshLease(t *testing.T) {
	scanStore, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()

	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	if err := scanStore.SaveScanRun(ScanRun{RunID: "run-1", Status: "running", StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := scanStore.AcquireRunLease("run-1", "owner-1", now, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := scanStore.ReconcileInterruptedRuns(now.Add(30*time.Second), time.Minute); err != nil {
		t.Fatal(err)
	}
	run, err := scanStore.GetScanRun("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "running" {
		t.Fatalf("fresh lease changed status to %q", run.Status)
	}
}

func TestReconcileInterruptedRunsClosesExpiredOrMissingRunsOnce(t *testing.T) {
	scanStore, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()

	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	for _, runID := range []string{"expired", "missing"} {
		if err := scanStore.SaveScanRun(ScanRun{RunID: runID, Status: "running", ConfigSnapshot: "original", ArtifactDir: "/tmp/artifacts/" + runID, StartedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanStore.SaveFingerprint("expired", fingerprint.ServiceFingerprint{IP: "198.51.100.10", Port: 443, Service: "https"}); err != nil {
		t.Fatal(err)
	}
	if err := scanStore.SaveFinding("expired", report.Finding{IP: "198.51.100.10", Port: 443, ID: "test", Severity: "info"}); err != nil {
		t.Fatal(err)
	}
	if _, err := scanStore.AcquireRunLease("expired", "owner-1", now, time.Minute); err != nil {
		t.Fatal(err)
	}
	reconciledAt := now.Add(2 * time.Minute)
	if err := scanStore.ReconcileInterruptedRuns(reconciledAt, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := scanStore.ReconcileInterruptedRuns(reconciledAt.Add(time.Minute), time.Minute); err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		runID string
		error string
	}{
		{runID: "expired", error: "run lease expired"},
		{runID: "missing", error: "run lease missing"},
	} {
		run, err := scanStore.GetScanRun(want.runID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != "interrupted" || run.Error != want.error || !run.FinishedAt.Equal(reconciledAt) {
			t.Fatalf("run %q = %#v", want.runID, run)
		}
	}
	fingerprints, err := scanStore.ListFingerprints("expired")
	if err != nil || len(fingerprints) != 1 {
		t.Fatalf("reconciliation lost fingerprints: %#v %v", fingerprints, err)
	}
	findings, err := scanStore.ListFindings("expired")
	if err != nil || len(findings) != 1 {
		t.Fatalf("reconciliation lost findings: %#v %v", findings, err)
	}
}
