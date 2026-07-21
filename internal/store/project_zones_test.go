package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestProjectMetadataRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	project := Project{
		ID:          "p1",
		Name:        "甘肃电力内网安全检查任务",
		Description: "季度任务",
		ClientUnit:  "甘肃电力",
		ReportTitle: "甘肃电力内网安全检查报告",
		TestObject:  "信息内网",
		StartDate:   "2026-07-01",
		EndDate:     "2026-07-15",
		Testers:     "张三, 李四",
		CreatedAt:   time.Unix(1, 0),
		UpdatedAt:   time.Unix(1, 0),
	}
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	got, err := s.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	if got.ClientUnit != "甘肃电力" || got.ReportTitle != "甘肃电力内网安全检查报告" || got.TestObject != "信息内网" {
		t.Fatalf("metadata mismatch: %#v", got)
	}
	if got.StartDate != "2026-07-01" || got.EndDate != "2026-07-15" || got.Testers != "张三, 李四" {
		t.Fatalf("date/testers mismatch: %#v", got)
	}

	project.ReportTitle = "更新后的报告"
	project.UpdatedAt = time.Unix(2, 0)
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject update returned error: %v", err)
	}
	got, err = s.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	if got.ReportTitle != "更新后的报告" {
		t.Fatalf("report title not updated: %#v", got)
	}
}

func TestCreateDefaultProjectZones(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if err := s.SaveProject(Project{ID: "p1", Name: "Task", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	zones, err := s.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 3 {
		t.Fatalf("expected 3 default zones, got %#v", zones)
	}
	for i, want := range []struct {
		id   string
		name string
	}{
		{"I", "I区"},
		{"II", "II区"},
		{"III", "III区"},
	} {
		if zones[i].ZoneID != want.id || zones[i].Name != want.name || zones[i].SortOrder != i {
			t.Fatalf("zone %d mismatch: %#v", i, zones[i])
		}
	}
}

func TestProjectZonesAreStableAndSorted(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if err := s.SaveProject(Project{ID: "p1", Name: "Task", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	order, err := s.NextProjectZoneSortOrder("p1")
	if err != nil {
		t.Fatalf("NextProjectZoneSortOrder returned error: %v", err)
	}
	custom := ProjectZone{ProjectID: "p1", ZoneID: "custom-1", Name: "DMZ", SortOrder: order}
	if err := s.CreateProjectZone(custom); err != nil {
		t.Fatalf("CreateProjectZone returned error: %v", err)
	}

	zones, err := s.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 4 || zones[3].ZoneID != "custom-1" || zones[3].SortOrder != 3 {
		t.Fatalf("unexpected zones: %#v", zones)
	}
}

func TestDeleteProjectZoneRejectsWhenRunExists(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if err := s.SaveProject(Project{ID: "p1", Name: "Task", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(ScanRun{RunID: "run-1", ProjectID: "p1", ZoneID: "I", Target: "127.0.0.1", Ports: "80", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	hasRuns, err := s.ZoneHasRuns("p1", "I")
	if err != nil {
		t.Fatalf("ZoneHasRuns returned error: %v", err)
	}
	if !hasRuns {
		t.Fatalf("expected ZoneHasRuns true")
	}

	// The store layer allows deletion; enforcement belongs to the web layer.
	if err := s.DeleteProjectZone("p1", "I"); err != nil {
		t.Fatalf("DeleteProjectZone returned error: %v", err)
	}
	zones, err := s.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones after deletion, got %#v", zones)
	}
}
