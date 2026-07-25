package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestBuildProjectDeliverableAssemblesEvidence(t *testing.T) {
	st := newProjectDeliverableStore(t)
	defer st.Close()
	seedProjectDeliverable(t, st)

	deliverable, err := BuildProjectDeliverable(st, "p1", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("BuildProjectDeliverable returned error: %v", err)
	}
	if deliverable.Project.Name != "甘肃任务" {
		t.Fatalf("project name = %q", deliverable.Project.Name)
	}
	if len(deliverable.Zones) != 1 || len(deliverable.Zones[0].Confirmed) != 1 {
		t.Fatalf("deliverable zones = %#v", deliverable.Zones)
	}
	evidence := deliverable.Zones[0].Confirmed[0].Evidence
	if len(evidence) != 1 || !strings.HasPrefix(evidence[0].DataURI, "data:image/png;base64,") || evidence[0].FilePath == "" {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestBuildProjectDeliverableReturnsNotFoundCode(t *testing.T) {
	st := newProjectDeliverableStore(t)
	defer st.Close()

	_, err := BuildProjectDeliverable(st, "missing", time.Unix(100, 0))
	var perr *ProjectReportError
	if !errors.As(err, &perr) || perr.Code != CodeProjectNotFound {
		t.Fatalf("expected CodeProjectNotFound, got %v", err)
	}
	if !strings.Contains(perr.Message, "不存在") {
		t.Fatalf("expected user-facing not-found message, got %q", perr.Message)
	}
}

func TestBuildProjectDeliverableReturnsInvalidCodeForMissingMetadata(t *testing.T) {
	st := newProjectDeliverableStore(t)
	defer st.Close()
	if err := st.SaveProject(store.Project{ID: "p1", Name: "空项目"}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	_, err := BuildProjectDeliverable(st, "p1", time.Unix(100, 0))
	var perr *ProjectReportError
	if !errors.As(err, &perr) || perr.Code != CodeProjectReportInvalid {
		t.Fatalf("expected CodeProjectReportInvalid, got %v", err)
	}
	if !strings.Contains(perr.Message, "不完整") || !strings.Contains(perr.Message, "项目工作台") {
		t.Fatalf("expected user-action hint in message, got %q", perr.Message)
	}
}

func TestBuildProjectDeliverableReturnsUnavailableCodeForEvidenceReadFailure(t *testing.T) {
	st := newProjectDeliverableStore(t)
	defer st.Close()
	seedProjectDeliverable(t, st)
	evidence, err := st.ListVerificationEvidence("v1")
	if err != nil || len(evidence) != 1 {
		t.Fatalf("ListVerificationEvidence = %#v, %v", evidence, err)
	}
	if err := os.Remove(st.EvidenceFilePath(evidence[0], "p1")); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	_, err = BuildProjectDeliverable(st, "p1", time.Unix(100, 0))
	var perr *ProjectReportError
	if !errors.As(err, &perr) || perr.Code != CodeProjectReportUnavailable {
		t.Fatalf("expected CodeProjectReportUnavailable, got %v", err)
	}
	if !strings.Contains(perr.Message, "暂时无法生成") {
		t.Fatalf("expected user-facing unavailable message, got %q", perr.Message)
	}
}

func newProjectDeliverableStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return st
}

func seedProjectDeliverable(t *testing.T, st *store.Store) {
	t.Helper()
	createdAt := time.Unix(1, 0)
	if err := st.SaveProject(store.Project{
		ID: "p1", Name: "甘肃任务", ClientUnit: "示例电力", TestObject: "生产系统",
		StartDate: "2026-07-01", EndDate: "2026-07-05", Testers: "张三", CreatedAt: createdAt, UpdatedAt: createdAt,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := st.CreateProjectZone(store.ProjectZone{ProjectID: "p1", ZoneID: "I", Name: "I区"}); err != nil {
		t.Fatalf("CreateProjectZone returned error: %v", err)
	}
	if err := st.CreateVerification(store.Verification{
		ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "弱口令", Severity: "high",
		Description: "弱口令", Remediation: "修改密码", Position: 1, CreatedAt: createdAt, UpdatedAt: createdAt,
	}, []store.VerificationAsset{{VerificationID: "v1", IP: "10.0.0.1", Port: 22}}, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}
	if _, err := st.CreateEvidence("p1", store.CreateEvidenceInput{VerificationID: "v1", Data: projectDeliverablePNG(), Position: 0}); err != nil {
		t.Fatalf("CreateEvidence returned error: %v", err)
	}
}

func projectDeliverablePNG() []byte {
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x03, 0x00, 0x01, 0x5b, 0x70, 0x20, 0xd7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}
}
