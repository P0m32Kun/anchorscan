package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// tinyPNG returns a minimal valid 1x1 PNG for evidence fixtures.
func tinyPNG() []byte {
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x03, 0x00, 0x01, 0x5b, 0x70, 0x20, 0xd7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}
}

func newProjectReportStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	dbPath := filepath.Join(dir, "scan.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return st
}

func seedProjectReportFixtures(t *testing.T, st *store.Store) {
	t.Helper()
	if err := st.SaveProject(store.Project{
		ID:         "p1",
		Name:       "甘肃任务",
		ClientUnit: "示例电力有限公司",
		TestObject: "示例电力生产控制系统",
		StartDate:  "2026-07-01",
		EndDate:    "2026-07-05",
		Testers:    "张三、李四",
		CreatedAt:  time.Unix(1, 0),
		UpdatedAt:  time.Unix(1, 0),
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := st.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	zones, _ := st.ListProjectZones("p1")
	zoneI := zones[0].ZoneID
	if err := st.SaveScanRun(store.ScanRun{
		RunID: "r1", ProjectID: "p1", ZoneID: zoneI, Kind: "scan", Status: "completed", IncludeInReport: true,
		Label: "核心交换机", AccessPoint: "SW-01", TesterIP: "10.0.0.10", Target: "10.0.1.0/24", Notes: "夜间窗口",
		ConfigSnapshot: `{"exclude_targets":"10.0.1.99","exclude_ports":"22"}`,
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	// Confirmed verification with one PNG evidence.
	if err := st.CreateVerification(store.Verification{
		ID: "v1", ProjectID: "p1", ZoneID: zoneI, Outcome: "confirmed",
		Title: "弱口令", Severity: "high", Description: "发现可被爆破的弱口令",
		Remediation: "修改默认密码并启用强密码策略", Included: true, Position: 1,
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}, []store.VerificationAsset{{VerificationID: "v1", IP: "10.0.0.1", Port: 22, Position: 1}, {VerificationID: "v1", IP: "10.0.0.2", Port: 443, Position: 2}}, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}
	_, err := st.CreateEvidence("p1", store.CreateEvidenceInput{
		VerificationID: "v1", Data: tinyPNG(), Caption: "弱口令证据", Position: 0,
	})
	if err != nil {
		t.Fatalf("CreateEvidence returned error: %v", err)
	}

	// Not observed verification.
	if err := st.CreateVerification(store.Verification{
		ID: "v2", ProjectID: "p1", ZoneID: zoneI, Outcome: "not_observed",
		Title: "Redis未授权访问", Severity: "high", Included: true, Position: 2,
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}, []store.VerificationAsset{{VerificationID: "v2", IP: "10.0.0.2", Port: 6379, Position: 1}}, nil); err != nil {
		t.Fatalf("CreateVerification v2 returned error: %v", err)
	}
	_, err = st.CreateEvidence("p1", store.CreateEvidenceInput{
		VerificationID: "v2", Data: tinyPNG(), Position: 0,
	})
	if err != nil {
		t.Fatalf("CreateEvidence v2 returned error: %v", err)
	}

	// Non-included verification must not appear because outcome is inconclusive.
	if err := st.CreateVerification(store.Verification{
		ID: "v3", ProjectID: "p1", ZoneID: zoneI, Outcome: "inconclusive",
		Title: "不应出现的项", Severity: "high", Included: false, Position: 3,
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}, nil, nil); err != nil {
		t.Fatalf("CreateVerification v3 returned error: %v", err)
	}

	// Confirmed verification with Included=false should still be projected.
	if err := st.CreateVerification(store.Verification{
		ID: "v4", ProjectID: "p1", ZoneID: zoneI, Outcome: "confirmed",
		Title: "自动纳入的确认项", Severity: "medium", Description: "即使 Included=false 也会自动纳入", Remediation: "修复建议", Included: false, Position: 4,
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}, []store.VerificationAsset{{VerificationID: "v4", IP: "10.0.0.5", Port: 8080, Position: 1}}, nil); err != nil {
		t.Fatalf("CreateVerification v4 returned error: %v", err)
	}
	_, err = st.CreateEvidence("p1", store.CreateEvidenceInput{
		VerificationID: "v4", Data: tinyPNG(), Caption: "自动纳入证据", Position: 0,
	})
	if err != nil {
		t.Fatalf("CreateEvidence v4 returned error: %v", err)
	}
}

func TestProjectReportHTMLRendersIncludedVerificationsAndEvidence(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	seedProjectReportFixtures(t, st)
	st.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"), Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/report.html", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{
		"示例电力有限公司安全渗透测试分析报告",
		"测试对象：<span>示例电力生产控制系统",
		"弱口令",
		"修改默认密码并启用强密码策略",
		"10.0.0.1:22",
		"10.0.0.1:22, 10.0.0.2:443",
		"10.0.0.5:8080",
		"data:image/png;base64,",
		"弱口令证据",
		"Redis未授权访问",
		"自动纳入的确认项",
		"自动纳入证据",
		"SW-01",
		"10.0.0.10",
		"10.0.1.0/24",
		"排除范围",
		"目标：10.0.1.99；端口：22",
		"渗透测试概念",
		"Hscan",
		"测试过程介绍",
		"序号",
		"渗透测试结论",
		"高危漏洞1个",
		"Redis未授权访问不存在证明，端口（6379）",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body", want)
		}
	}
	for _, unwanted := range []string{"不应出现的项", "XX"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect %q in body", unwanted)
		}
	}
	if !strings.Contains(res.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content type = %s", res.Header().Get("Content-Type"))
	}
	results := strings.Index(body, "三、测试结果与分析")
	summary := strings.Index(body, "渗透测试结果汇总表")
	zone := strings.Index(body, "<h3>I区</h3>")
	conclusion := strings.Index(body, "四、渗透测试结论")
	if results < 0 || summary < results || zone < summary || conclusion < zone || strings.Contains(body, "五、渗透测试结论") {
		t.Fatalf("unexpected report section order")
	}
}

func TestProjectReportDOCXReturnsClearErrorWhenUnconfigured(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	seedProjectReportFixtures(t, st)
	st.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"), Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/report.docx", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when sidecar unconfigured, got %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "docxtpl") {
		t.Fatalf("expected docxtpl hint in body: %s", res.Body.String())
	}
}

func TestProjectReportDOCXRunsConfiguredSidecarScript(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	seedProjectReportFixtures(t, st)
	st.Close()

	sidecarDir := filepath.Join(dir, "tools", "docx-render")
	if err := os.MkdirAll(sidecarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	renderScript := filepath.Join(sidecarDir, "render_docx.py")
	writeFile(t, renderScript, "# renderer fixture\n")
	templatePath := filepath.Join(sidecarDir, "project-report.docx")
	writeFile(t, templatePath, "template")
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uvPath := filepath.Join(binDir, "uv")
	uvScript := `#!/bin/sh
if [ "$1" != "run" ] || [ "$2" != "--project" ] || [ "$4" != "python" ] || [ "$5" != "$3/render_docx.py" ]; then
  exit 42
fi
shift 10
printf 'fake-docx' > "$1"
`
	if err := os.WriteFile(uvPath, []byte(uvScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	handler, err := NewServer(ServerOptions{
		ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"),
		DocxRenderProject: sidecarDir, DocxTemplatePath: templatePath,
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/report.docx", nil))
	if res.Code != http.StatusOK || res.Body.String() != "fake-docx" {
		t.Fatalf("unexpected response: %d %q", res.Code, res.Body.String())
	}
}

func TestProjectReportHTMLRejectsMissingMetadata(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	if err := st.SaveProject(store.Project{ID: "p2", Name: "空项目", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	st.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"), Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p2/report.html", nil))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing metadata, got %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"被测单位", "测试对象", "测试开始日期", "测试结束日期", "测试人员"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in error body: %s", want, body)
		}
	}
}

func TestProjectReportHTMLRejectsIncompleteIncludedRun(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	seedProjectReportFixtures(t, st)
	run, err := st.GetScanRun("r1")
	if err != nil {
		t.Fatal(err)
	}
	run.TesterIP = ""
	if err := st.SaveScanRun(run); err != nil {
		t.Fatal(err)
	}
	st.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/report.html", nil))
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "测试设备 IP") {
		t.Fatalf("expected run validation error, got %d %s", res.Code, res.Body.String())
	}
}

func TestProjectReportHTMLRejectsCriticalUntilConclusionWordingIsApproved(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	seedProjectReportFixtures(t, st)
	full, err := st.GetVerification("v1")
	if err != nil {
		t.Fatal(err)
	}
	full.Verification.Severity = "critical"
	if err := st.UpdateVerification(full.Verification); err != nil {
		t.Fatal(err)
	}
	st.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/report.html", nil))
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "critical") {
		t.Fatalf("expected critical validation error, got %d %s", res.Code, res.Body.String())
	}
}

func TestProjectReportHTMLRejectsIncludedVerificationWithoutEvidence(t *testing.T) {
	dir := t.TempDir()
	st := newProjectReportStore(t, dir)
	seedProjectReportFixtures(t, st)
	evidence, err := st.ListVerificationEvidence("v1")
	if err != nil || len(evidence) != 1 {
		t.Fatalf("ListVerificationEvidence = %#v, %v", evidence, err)
	}
	if err := st.DeleteEvidence(evidence[0].ID); err != nil {
		t.Fatalf("DeleteEvidence returned error: %v", err)
	}
	st.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"), Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/report.html", nil))
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "缺少证据") {
		t.Fatalf("expected missing-evidence error, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestReportTitleFallsBackToClientUnit(t *testing.T) {
	if got := reportTitle(store.Project{ClientUnit: "甘肃电力"}); got != "甘肃电力安全渗透测试分析报告" {
		t.Fatalf("expected fallback title, got %q", got)
	}
}

func TestReportTitleUsesExistingReportTitle(t *testing.T) {
	if got := reportTitle(store.Project{ReportTitle: "自定义标题", ClientUnit: "甘肃电力"}); got != "自定义标题" {
		t.Fatalf("expected existing title, got %q", got)
	}
}

func TestReportTitleFallsBackToName(t *testing.T) {
	if got := reportTitle(store.Project{Name: "某任务", ClientUnit: ""}); got != "某任务安全渗透测试分析报告" {
		t.Fatalf("expected name fallback title, got %q", got)
	}
}

func TestReportTitleBareFallback(t *testing.T) {
	if got := reportTitle(store.Project{}); got != "安全渗透测试分析报告" {
		t.Fatalf("expected bare fallback title, got %q", got)
	}
}
