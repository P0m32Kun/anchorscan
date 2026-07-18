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
