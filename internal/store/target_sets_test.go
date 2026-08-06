package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// Ticket 07: Target Set persistence must upsert per project, survive the
// migration, and be cleaned up when the project is deleted.

func TestTargetSetSaveGetUpsert(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if err := s.SaveProject(Project{ID: "ts-p1", Name: "TS Project", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	first, err := s.SaveTargetSet("ts-p1", "内网资产", "192.0.2.10\n198.51.100.0/24")
	if err != nil {
		t.Fatalf("SaveTargetSet returned error: %v", err)
	}
	if first.ProjectID != "ts-p1" || len(first.TargetList()) != 2 {
		t.Fatalf("unexpected first save: %+v", first)
	}

	// Upsert replaces targets and preserves a single row per project.
	second, err := s.SaveTargetSet("ts-p1", "内网资产", "192.0.2.10\n2001:db8::1")
	if err != nil {
		t.Fatalf("second SaveTargetSet returned error: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("upsert must keep the same row id: %s != %s", second.ID, first.ID)
	}
	list := second.TargetList()
	if len(list) != 2 || list[0] != "192.0.2.10" || list[1] != "2001:db8::1" {
		t.Fatalf("unexpected replaced targets: %v", list)
	}

	got, err := s.GetTargetSet("ts-p1")
	if err != nil {
		t.Fatalf("GetTargetSet returned error: %v", err)
	}
	if got.Name != "内网资产" || got.Targets != "192.0.2.10\n2001:db8::1" {
		t.Fatalf("unexpected stored targets: %+v", got)
	}
}

func TestTargetSetAbsentReturnsNoRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if _, err := s.GetTargetSet("missing"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestTargetSetDeletedWithProject(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if err := s.SaveProject(Project{ID: "ts-p2", Name: "Cascade", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if _, err := s.SaveTargetSet("ts-p2", "", "192.0.2.1"); err != nil {
		t.Fatalf("SaveTargetSet returned error: %v", err)
	}
	if err := s.DeleteProjectCascade("ts-p2"); err != nil {
		t.Fatalf("DeleteProjectCascade returned error: %v", err)
	}
	if _, err := s.GetTargetSet("ts-p2"); err != sql.ErrNoRows {
		t.Fatalf("target set must be removed with the project, got %v", err)
	}
}
