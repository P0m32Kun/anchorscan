package store

import (
	"testing"

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
