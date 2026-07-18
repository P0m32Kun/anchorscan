package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
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
