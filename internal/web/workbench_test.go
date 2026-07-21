package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func writeTestCatalog(t *testing.T, dir string) string {
	t.Helper()
	content := `<!-- anchorscan-catalog version: 1 -->

### Redis 默认登录（高危）

<!-- anchorscan-entry
id: redis-default-login
aliases:
match:
  nuclei:
    - redis-default-logins
-->

#### 漏洞描述
Redis 服务未启用认证。

#### 验证命令

##### Nuclei
` + "```" + `
nuclei -t redis-default-logins -u {{host}}:{{port}}
` + "```" + `

##### Nmap NSE
` + "```" + `
nmap -p {{port}} --script redis-info {{host}}
` + "```" + `

##### MSF
` + "```" + `
use auxiliary/scanner/redis/redis_login
set RHOSTS {{host}}
set RPORT {{port}}
run
` + "```" + `

#### 修复建议
启用 Redis 认证。

---
`
	path := filepath.Join(dir, "kb.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()
	content := `knowledge_base:
  path: kb.md
`
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func setupWorkbenchProject(t *testing.T) (http.Handler, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := writeTestConfig(t, dir)
	_ = writeTestCatalog(t, dir)

	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	project := store.Project{
		ID:        "project-20260101-120000.000000000",
		Name:      "Workbench Test",
		CreatedAt: time.Unix(1, 0),
		UpdatedAt: time.Unix(1, 0),
	}
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones(project.ID); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	run := store.ScanRun{
		RunID:           "run-20260101-120000.000000000",
		ProjectID:       project.ID,
		ZoneID:          "I",
		Target:          "192.168.1.10",
		Ports:           "6379",
		Profile:         "normal",
		Status:          "completed",
		IncludeInReport: true,
		StartedAt:       time.Unix(1, 0),
		FinishedAt:      time.Unix(2, 0),
	}
	if err := s.SaveScanRun(run); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := s.SaveFingerprint(run.RunID, fingerprint.ServiceFingerprint{
		IP:       "192.168.1.10",
		Port:     6379,
		Protocol: "tcp",
		Service:  "redis",
	}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := s.SaveFinding(run.RunID, report.Finding{
		IP:       "192.168.1.10",
		Port:     6379,
		Protocol: "tcp",
		Source:   "nuclei",
		ID:       "redis-default-logins",
		Severity: "high",
		Summary:  "Redis 默认登录",
		Target:   "192.168.1.10:6379",
	}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}

	return handler, project.ID, run.RunID, dbPath
}

func TestWorkbenchPageListsPositiveCandidate(t *testing.T) {
	handler, projectID, _, _ := setupWorkbenchProject(t)

	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, "Redis 默认登录") {
		t.Fatalf("expected candidate title in body, got: %s", body)
	}
	if !strings.Contains(body, "192.168.1.10:6379") {
		t.Fatalf("expected asset in body, got: %s", body)
	}
}

func TestWorkbenchCandidateCommandGeneratesToolLink(t *testing.T) {
	handler, projectID, _, _ := setupWorkbenchProject(t)

	endpoint := "/projects/" + projectID + "/candidates/redis-default-login/commands"
	req := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader("tool=nuclei"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	commands, ok := payload["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("expected commands array, got: %#v", payload)
	}
	first := commands[0].(map[string]any)
	if !strings.Contains(first["full_command"].(string), "nuclei") {
		t.Fatalf("expected nuclei command, got: %s", first["full_command"])
	}
	if payload["tool_link"] == "" {
		t.Fatalf("expected tool_link, got empty: %#v", payload)
	}
}

func TestWorkbenchCannotIncludeConfirmedWithoutEvidence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := writeTestConfig(t, dir)
	_ = writeTestCatalog(t, dir)

	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	project := store.Project{
		ID:        "project-20260101-120000.000000000",
		Name:      "Workbench Test",
		CreatedAt: time.Unix(1, 0),
		UpdatedAt: time.Unix(1, 0),
	}
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones(project.ID); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	payload := verificationCreateRequest{
		ZoneID:           "I",
		VulnerabilityKey: "some-key",
		Outcome:          "confirmed",
		Title:            "Confirmed without evidence",
		Severity:         "high",
		Included:         true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+project.ID+"/verifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for included without evidence, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWorkbenchCanIncludeConfirmedAfterEvidence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := writeTestConfig(t, dir)
	_ = writeTestCatalog(t, dir)

	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	project := store.Project{
		ID:        "project-20260101-120000.000000000",
		Name:      "Workbench Test",
		CreatedAt: time.Unix(1, 0),
		UpdatedAt: time.Unix(1, 0),
	}
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones(project.ID); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	payload := verificationCreateRequest{
		ZoneID:           "I",
		VulnerabilityKey: "some-key",
		Outcome:          "confirmed",
		Title:            "Confirmed with evidence",
		Severity:         "high",
		Included:         false,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+project.ID+"/verifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	var v store.Verification
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	uploadRes := uploadEvidence(t, handler, "/projects/"+project.ID+"/verifications/"+v.ID+"/evidence", generateTestPNG(t), "confirmed")
	if uploadRes.Code != http.StatusCreated {
		t.Fatalf("upload returned %d: %s", uploadRes.Code, uploadRes.Body.String())
	}

	update := verificationUpdateRequest{Included: true, Outcome: "confirmed", Title: v.Title, Severity: v.Severity}
	updateBody, _ := json.Marshal(update)
	updateReq := httptest.NewRequest(http.MethodPost, "/projects/"+project.ID+"/verifications/"+v.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRes := httptest.NewRecorder()
	handler.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d: %s", updateRes.Code, updateRes.Body.String())
	}
	var updated store.Verification
	if err := json.Unmarshal(updateRes.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if !updated.Included {
		t.Fatalf("expected verification to be included")
	}
}
