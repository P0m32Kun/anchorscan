package store

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerificationRoundTripWithAssetsAndSources(t *testing.T) {
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

	now := time.Unix(2, 0)
	v := Verification{
		ID:               "v1",
		ProjectID:        "p1",
		ZoneID:           "I",
		VulnerabilityKey: "redis-default-login",
		Outcome:          "confirmed",
		Title:            "Redis 默认登录",
		Severity:         "high",
		Description:      "未启用认证",
		Remediation:      "启用认证",
		Notes:            "已复核",
		Included:         true,
		Position:         1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	assets := []VerificationAsset{
		{VerificationID: "v1", IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Position: 0},
		{VerificationID: "v1", IP: "10.0.0.2", Port: 6379, Protocol: "tcp", Position: 1},
	}
	sources := []VerificationSource{
		{VerificationID: "v1", RunID: "run1", Source: "nuclei", FindingID: "redis-default-logins", IP: "10.0.0.1", Port: 6379, Protocol: "tcp"},
	}

	if err := s.CreateVerification(v, assets, sources); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	got, err := s.GetVerification("v1")
	if err != nil {
		t.Fatalf("GetVerification returned error: %v", err)
	}
	if got.Verification.Title != "Redis 默认登录" || got.Verification.Outcome != "confirmed" {
		t.Fatalf("verification mismatch: %#v", got.Verification)
	}
	if len(got.Assets) != 2 || got.Assets[0].IP != "10.0.0.1" {
		t.Fatalf("assets mismatch: %#v", got.Assets)
	}
	if len(got.Sources) != 1 || got.Sources[0].RunID != "run1" {
		t.Fatalf("sources mismatch: %#v", got.Sources)
	}

	list, err := s.ListProjectVerifications("p1")
	if err != nil {
		t.Fatalf("ListProjectVerifications returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != "v1" {
		t.Fatalf("unexpected project verifications: %#v", list)
	}

	zoneList, err := s.ListZoneVerifications("p1", "I")
	if err != nil {
		t.Fatalf("ListZoneVerifications returned error: %v", err)
	}
	if len(zoneList) != 1 {
		t.Fatalf("expected 1 zone verification, got %d", len(zoneList))
	}

	zoneListII, err := s.ListZoneVerifications("p1", "II")
	if err != nil {
		t.Fatalf("ListZoneVerifications returned error: %v", err)
	}
	if len(zoneListII) != 0 {
		t.Fatalf("expected 0 zone II verifications, got %d", len(zoneListII))
	}
}

func TestCreateVerificationRejectsInvalidOutcome(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	v := Verification{
		ID:        "v1",
		ProjectID: "p1",
		Outcome:   "fixed",
		Title:     "X",
	}
	if err := s.CreateVerification(v, nil, nil); err == nil {
		t.Fatalf("expected invalid outcome to be rejected")
	}
}

func TestUpdateVerification(t *testing.T) {
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

	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "inconclusive", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	v.Outcome = "confirmed"
	v.Title = "Confirmed Title"
	v.UpdatedAt = time.Unix(2, 0)
	if err := s.UpdateVerification(v); err != nil {
		t.Fatalf("UpdateVerification returned error: %v", err)
	}

	got, err := s.GetVerification("v1")
	if err != nil {
		t.Fatalf("GetVerification returned error: %v", err)
	}
	if got.Verification.Outcome != "confirmed" || got.Verification.Title != "Confirmed Title" {
		t.Fatalf("update not applied: %#v", got.Verification)
	}
}

func TestSetVerificationAssetsAndSources(t *testing.T) {
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

	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, []VerificationAsset{{VerificationID: "v1", IP: "1.1.1.1", Port: 1, Protocol: "tcp"}}, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	if err := s.SetVerificationAssets("v1", []VerificationAsset{{VerificationID: "v1", IP: "2.2.2.2", Port: 2, Protocol: "udp"}}); err != nil {
		t.Fatalf("SetVerificationAssets returned error: %v", err)
	}
	if err := s.SetVerificationSources("v1", []VerificationSource{{VerificationID: "v1", RunID: "r1", Source: "nuclei", FindingID: "x", IP: "2.2.2.2", Port: 2, Protocol: "udp"}}); err != nil {
		t.Fatalf("SetVerificationSources returned error: %v", err)
	}

	got, err := s.GetVerification("v1")
	if err != nil {
		t.Fatalf("GetVerification returned error: %v", err)
	}
	if len(got.Assets) != 1 || got.Assets[0].IP != "2.2.2.2" {
		t.Fatalf("assets not replaced: %#v", got.Assets)
	}
	if len(got.Sources) != 1 || got.Sources[0].RunID != "r1" {
		t.Fatalf("sources not replaced: %#v", got.Sources)
	}
}

func TestZoneHasVerifications(t *testing.T) {
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	has, err := s.ZoneHasVerifications("p1", "I")
	if err != nil {
		t.Fatalf("ZoneHasVerifications returned error: %v", err)
	}
	if !has {
		t.Fatalf("expected ZoneHasVerifications true")
	}
	has, err = s.ZoneHasVerifications("p1", "II")
	if err != nil {
		t.Fatalf("ZoneHasVerifications returned error: %v", err)
	}
	if has {
		t.Fatalf("expected ZoneHasVerifications false for II")
	}
}

func TestDeleteVerificationCascadeRemovesEvidenceFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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

	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	ev, err := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: generatePNG(t), Caption: "c1", Position: 0})
	if err != nil {
		t.Fatalf("CreateEvidence returned error: %v", err)
	}
	absPath := s.EvidenceFilePath(ev, "p1")
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("evidence file not written: %v", err)
	}

	if err := s.DeleteVerificationCascade("v1"); err != nil {
		t.Fatalf("DeleteVerificationCascade returned error: %v", err)
	}

	if _, err := s.GetVerification("v1"); err == nil {
		t.Fatalf("expected verification to be deleted")
	}
	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Fatalf("expected evidence file to be removed, got %v", err)
	}
}

func TestDeleteProjectCascadeRemovesVerificationsAndZones(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	if err := s.DeleteProjectCascade("p1"); err != nil {
		t.Fatalf("DeleteProjectCascade returned error: %v", err)
	}

	if _, err := s.GetProject("p1"); err == nil {
		t.Fatalf("expected project to be deleted")
	}
	zones, err := s.ListProjectZones("p1")
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) != 0 {
		t.Fatalf("expected zones to be deleted, got %#v", zones)
	}
	verifications, err := s.ListProjectVerifications("p1")
	if err != nil {
		t.Fatalf("ListProjectVerifications returned error: %v", err)
	}
	if len(verifications) != 0 {
		t.Fatalf("expected verifications to be deleted, got %#v", verifications)
	}
}

func TestCreateEvidenceAcceptsPNGAndJPEG(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	evPNG, err := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: generatePNG(t), Caption: "png", Position: 0})
	if err != nil {
		t.Fatalf("CreateEvidence PNG returned error: %v", err)
	}
	if evPNG.MediaType != "image/png" || evPNG.Width != 2 || evPNG.Height != 3 {
		t.Fatalf("unexpected PNG evidence: %#v", evPNG)
	}

	evJPEG, err := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: generateJPEG(t), Caption: "jpg", Position: 1})
	if err != nil {
		t.Fatalf("CreateEvidence JPEG returned error: %v", err)
	}
	if evJPEG.MediaType != "image/jpeg" {
		t.Fatalf("unexpected JPEG media type: %s", evJPEG.MediaType)
	}
}

func TestCreateEvidenceRejectsInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	if _, err := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: []byte("not an image"), Caption: "x", Position: 0}); err == nil {
		t.Fatalf("expected invalid image to be rejected")
	}
}

func TestCreateEvidenceRejectsWrongProject(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	if _, err := s.CreateEvidence("p2", CreateEvidenceInput{VerificationID: "v1", Data: generatePNG(t), Caption: "x", Position: 0}); err == nil {
		t.Fatalf("expected wrong project to be rejected")
	}
}

func TestEvidenceUploadFailureLeavesNoOrphan(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	// Invalid data should not write any file or DB row.
	if _, err := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: []byte{0xFF, 0xD8}, Caption: "x", Position: 0}); err == nil {
		t.Fatalf("expected incomplete JPEG to be rejected")
	}

	list, err := s.ListVerificationEvidence("v1")
	if err != nil {
		t.Fatalf("ListVerificationEvidence returned error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no evidence rows after failed upload, got %d", len(list))
	}
	matches, err := filepath.Glob(filepath.Join(dir, "projects", "p1", "evidence", "v1", "*"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no evidence files after failed upload, got %v", matches)
	}
}

func TestEvidenceCaptionAndOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "scan.db"))
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
	v := Verification{ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "T", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}

	ev1, _ := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: generatePNG(t), Caption: "first", Position: 0})
	ev2, _ := s.CreateEvidence("p1", CreateEvidenceInput{VerificationID: "v1", Data: generatePNG(t), Caption: "second", Position: 1})

	if err := s.UpdateEvidenceCaption(ev1.ID, "first updated"); err != nil {
		t.Fatalf("UpdateEvidenceCaption returned error: %v", err)
	}
	if err := s.ReorderEvidence("v1", []string{ev2.ID, ev1.ID}); err != nil {
		t.Fatalf("ReorderEvidence returned error: %v", err)
	}

	list, _ := s.ListVerificationEvidence("v1")
	if len(list) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(list))
	}
	if list[0].ID != ev2.ID || list[0].Position != 0 {
		t.Fatalf("expected ev2 first, got %#v", list[0])
	}
	if list[1].Caption != "first updated" || list[1].Position != 1 {
		t.Fatalf("expected ev1 caption updated and position 1, got %#v", list[1])
	}

	if err := s.DeleteEvidence(ev1.ID); err != nil {
		t.Fatalf("DeleteEvidence returned error: %v", err)
	}
	list, _ = s.ListVerificationEvidence("v1")
	if len(list) != 1 {
		t.Fatalf("expected 1 evidence item after delete, got %d", len(list))
	}
}

func generatePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode returned error: %v", err)
	}
	return buf.Bytes()
}

func generateJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 5))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode returned error: %v", err)
	}
	return buf.Bytes()
}
