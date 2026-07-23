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
	"github.com/P0m32Kun/anchorscan/internal/vuln"
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

func TestWorkbenchAPIReturnsWorkbenchData(t *testing.T) {
	handler, projectID, _, _ := setupWorkbenchProject(t)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if payload["project_id"] != projectID {
		t.Fatalf("expected project_id %s, got %v", projectID, payload["project_id"])
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		t.Fatalf("expected candidates array, got %#v", payload["candidates"])
	}
	first := candidates[0].(map[string]any)
	if first["Title"] != "Redis 默认登录" {
		t.Fatalf("expected Redis candidate title, got %v", first["Title"])
	}
	counts := payload["counts"].(map[string]any)
	if counts["positive"] != float64(1) {
		t.Fatalf("expected positive count 1, got %v", counts["positive"])
	}
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
	if !strings.Contains(body, `data-workbench`) {
		t.Fatalf("expected vue mount point in body, got: %s", body)
	}
	if !strings.Contains(body, "Redis 默认登录") {
		t.Fatalf("expected candidate title in serialized props, got: %s", body)
	}
	if !strings.Contains(body, "192.168.1.10:6379") {
		t.Fatalf("expected asset in serialized props, got: %s", body)
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
	toolLink := payload["tool_link"].(string)
	if !strings.Contains(toolLink, "project_id=") || !strings.Contains(toolLink, "zone_id=") {
		t.Fatalf("tool link must carry project context: %s", toolLink)
	}
	if !strings.Contains(toolLink, "raw_args=") || !strings.Contains(toolLink, "return=") {
		t.Fatalf("tool link must carry raw_args and return: %s", toolLink)
	}
}

func TestWorkbenchCreateConfirmedAutoIncludes(t *testing.T) {
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
		Title:            "Confirmed auto included",
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
	if !v.Included {
		t.Fatalf("expected confirmed verification to be auto-included, got Included=%v", v.Included)
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

// setupNegativeWorkbenchProject creates a project with one fingerprint endpoint
// that has NSE+nuclei both completed and no non-info finding — qualifying as a
// negative candidate. Also creates a second fingerprint with a failed nuclei
// check, which should appear as an incomplete check.
func setupNegativeWorkbenchProject(t *testing.T) (http.Handler, string, string) {
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

	now := time.Unix(1, 0)
	project := store.Project{
		ID:        "project-20260101-120000.000000001",
		Name:      "Negative Workbench Test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones(project.ID); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}

	run := store.ScanRun{
		RunID:           "run-neg-20260101-120000.000000001",
		ProjectID:       project.ID,
		ZoneID:          "I",
		Target:          "10.0.0.1",
		Ports:           "80,443",
		Profile:         "normal",
		Status:          "completed",
		IncludeInReport: true,
		StartedAt:       now,
		FinishedAt:      now,
	}
	if err := s.SaveScanRun(run); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	// Port 80: fingerprint + both checks completed + no finding → negative candidate
	if err := s.SaveFingerprint(run.RunID, fingerprint.ServiceFingerprint{
		IP: "10.0.0.1", Port: 80, Protocol: "tcp", Service: "http", Product: "nginx", Normalized: "http",
	}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{
		RunID: run.RunID, IP: "10.0.0.1", Port: 80, Protocol: "tcp",
		Engine: "nse", Status: "completed", StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("UpsertDetectionCheck nse returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{
		RunID: run.RunID, IP: "10.0.0.1", Port: 80, Protocol: "tcp",
		Engine: "nuclei", Status: "completed", StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("UpsertDetectionCheck nuclei returned error: %v", err)
	}

	// Port 443: fingerprint + nuclei failed → incomplete check
	if err := s.SaveFingerprint(run.RunID, fingerprint.ServiceFingerprint{
		IP: "10.0.0.1", Port: 443, Protocol: "tcp", Service: "https", Product: "nginx", Normalized: "https",
	}); err != nil {
		t.Fatalf("SaveFingerprint 443 returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{
		RunID: run.RunID, IP: "10.0.0.1", Port: 443, Protocol: "tcp",
		Engine: "nse", Status: "completed", StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("UpsertDetectionCheck nse 443 returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{
		RunID: run.RunID, IP: "10.0.0.1", Port: 443, Protocol: "tcp",
		Engine: "nuclei", Status: "failed", StartedAt: now, FinishedAt: now,
	}); err != nil {
		t.Fatalf("UpsertDetectionCheck nuclei 443 returned error: %v", err)
	}

	return handler, project.ID, dbPath
}

// TestWorkbenchPageRendersNegativeAndIncomplete verifies that the workbench page
// includes the serialized negative candidate and incomplete check data in the
// Vue mount point props.
func TestWorkbenchPageRendersNegativeAndIncomplete(t *testing.T) {
	handler, projectID, _ := setupNegativeWorkbenchProject(t)

	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `data-workbench`) {
		t.Fatalf("expected vue mount point in body")
	}

	// Assert via the JSON API that both queues contain the expected endpoints.
	apiReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/workbench", nil)
	apiRes := httptest.NewRecorder()
	handler.ServeHTTP(apiRes, apiReq)
	if apiRes.Code != http.StatusOK {
		t.Fatalf("expected api 200, got %d: %s", apiRes.Code, apiRes.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(apiRes.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal api returned error: %v", err)
	}
	negGroups, ok := payload["negative_groups"].([]any)
	if !ok || len(negGroups) == 0 {
		t.Fatalf("expected negative groups, got %#v", payload["negative_groups"])
	}
	negAssets := negGroups[0].(map[string]any)["Assets"].([]any)
	if len(negAssets) == 0 || negAssets[0].(map[string]any)["IP"] != "10.0.0.1" || int(negAssets[0].(map[string]any)["Port"].(float64)) != 80 {
		t.Fatalf("expected negative candidate 10.0.0.1:80, got %#v", negAssets)
	}
	incItems, ok := payload["incomplete_checks"].([]any)
	if !ok || len(incItems) == 0 {
		t.Fatalf("expected incomplete items, got %#v", payload["incomplete_checks"])
	}
	incAsset := incItems[0].(map[string]any)["Asset"].(map[string]any)
	if incAsset["IP"] != "10.0.0.1" || int(incAsset["Port"].(float64)) != 443 {
		t.Fatalf("expected incomplete check 10.0.0.1:443, got %#v", incAsset)
	}
}

// TestWorkbenchNegativeWorkbenchCandidateNotAutoCreated verifies that visiting
// the workbench page does not automatically create any not_observed Verification —
// candidates must be explicitly submitted by the user.
func TestWorkbenchNegativeCandidateNotAutoCreated(t *testing.T) {
	handler, projectID, dbPath := setupNegativeWorkbenchProject(t)

	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	// Confirm no verification was auto-created by reading the DB directly.
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	verifications, err := s.ListProjectVerifications(projectID)
	if err != nil {
		t.Fatalf("ListProjectVerifications returned error: %v", err)
	}
	for _, v := range verifications {
		if v.Outcome == "not_observed" {
			t.Fatalf("auto-created not_observed verification: %#v", v)
		}
	}
}

// TestWorkbenchSubmitNegativeVerification tests the full flow: POST a
// not_observed verification with assets, then upload evidence and confirm the
// verification is stored correctly.
func TestWorkbenchSubmitNegativeVerification(t *testing.T) {
	handler, projectID, dbPath := setupNegativeWorkbenchProject(t)

	// Step 1: Create the not_observed verification (not yet included, no evidence).
	payload := verificationCreateRequest{
		ZoneID:           "I",
		VulnerabilityKey: "neg:http-service-no-known-vuln",
		Outcome:          "not_observed",
		Title:            "HTTP 服务未检出已知漏洞",
		Severity:         "low",
		Description:      "本次验证执行了 NSE 与 nuclei 扫描，均未发现已知漏洞。",
		Included:         false,
		Assets: []store.VerificationAsset{
			{IP: "10.0.0.1", Port: 80, Protocol: "tcp", AssetName: "10.0.0.1:80", Position: 0},
		},
	}
	body, _ := json.Marshal(payload)
	createReq := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/verifications", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRes.Code, createRes.Body.String())
	}
	var created store.Verification
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if created.Outcome != "not_observed" {
		t.Fatalf("expected not_observed outcome, got %s", created.Outcome)
	}

	// Step 2: Upload evidence.
	evidenceURL := "/projects/" + projectID + "/verifications/" + created.ID + "/evidence"
	uploadRes := uploadEvidence(t, handler, evidenceURL, generateTestPNG(t), "执行 nuclei 扫描截图，覆盖端点 10.0.0.1:80")
	if uploadRes.Code != http.StatusCreated {
		t.Fatalf("evidence upload returned %d: %s", uploadRes.Code, uploadRes.Body.String())
	}

	// Step 3: Confirm via DB that assets are saved and evidence exists.
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	full, err := s.GetVerification(created.ID)
	if err != nil {
		t.Fatalf("GetVerification returned error: %v", err)
	}
	if full.Verification.Outcome != "not_observed" {
		t.Fatalf("expected not_observed, got %s", full.Verification.Outcome)
	}
	if len(full.Assets) != 1 || full.Assets[0].IP != "10.0.0.1" || full.Assets[0].Port != 80 {
		t.Fatalf("unexpected assets: %#v", full.Assets)
	}
	if len(full.Evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(full.Evidence))
	}
}

// TestWorkbenchIncompleteCheckExcludedFromNegative verifies that the
// fingerprint endpoint with a failed nuclei check (port 443) does NOT appear
// in the negative-data JSON block but DOES appear in the incomplete-data block.
func TestWorkbenchIncompleteCheckExcludedFromNegative(t *testing.T) {
	handler, projectID, _ := setupNegativeWorkbenchProject(t)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal api returned error: %v", err)
	}
	negGroups := payload["negative_groups"].([]any)
	incItems := payload["incomplete_checks"].([]any)

	// Negative queue: only port 80 (both completed) grouped by fingerprint
	if len(negGroups) != 1 {
		t.Fatalf("expected 1 negative group, got %d", len(negGroups))
	}
	g := negGroups[0].(map[string]any)
	assets := g["Assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset in negative group, got %d", len(assets))
	}
	asset := assets[0].(map[string]any)
	if int(asset["Port"].(float64)) != 80 {
		t.Fatalf("expected port 80 in negative group, got %v", asset["Port"])
	}

	// Incomplete queue: only port 443 (nuclei failed)
	if len(incItems) != 1 {
		t.Fatalf("expected 1 incomplete item, got %d", len(incItems))
	}
	inc := incItems[0].(map[string]any)
	incAsset := inc["Asset"].(map[string]any)
	if int(incAsset["Port"].(float64)) != 443 {
		t.Fatalf("expected port 443 in incomplete queue, got %v", incAsset["Port"])
	}
}

func TestGroupNegativeCandidatesByFingerprint(t *testing.T) {
	negatives := []report.ProjectNegativeCandidate{
		{
			Asset:       report.ProjectAsset{IP: "10.0.0.1", Port: 6379, Protocol: "tcp"},
			Fingerprint: fingerprint.ServiceFingerprint{Service: "redis"},
			ZoneID:      "I",
		},
		{
			Asset:       report.ProjectAsset{IP: "10.0.0.2", Port: 6380, Protocol: "tcp"},
			Fingerprint: fingerprint.ServiceFingerprint{Service: "redis"},
			ZoneID:      "I",
		},
		{
			Asset:       report.ProjectAsset{IP: "10.0.0.3", Port: 443, Protocol: "tcp"},
			Fingerprint: fingerprint.ServiceFingerprint{Service: "https", Product: "nginx"},
			ZoneID:      "I",
		},
	}
	nseRules := map[string][]string{"redis": {"redis-info"}}
	tagRules := []vuln.TagRule{{Name: "redis", Service: []string{"redis"}, NucleiTags: []string{"redis"}, Target: "hostport"}}
	groups := groupNegativeCandidates(negatives, nseRules, tagRules)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %#v", len(groups), groups)
	}
	redisGroup := groups[0]
	if groups[1].Service == "redis" {
		redisGroup = groups[1]
	}
	if len(redisGroup.Assets) != 2 {
		t.Fatalf("expected 2 assets in redis group, got %d", len(redisGroup.Assets))
	}
	if !strings.Contains(redisGroup.NmapCommand, "redis-info") {
		t.Fatalf("expected NmapCommand to use redis-info, got %q", redisGroup.NmapCommand)
	}
	if !strings.Contains(redisGroup.NmapCommand, "-p 6379,6380") || !strings.Contains(redisGroup.NmapCommand, "10.0.0.1 10.0.0.2") {
		t.Fatalf("expected NmapCommand to cover every port and IP in the group, got %q", redisGroup.NmapCommand)
	}
	if !strings.Contains(redisGroup.NucleiCommand, "-tags redis") {
		t.Fatalf("expected NucleiCommand to use -tags redis, got %q", redisGroup.NucleiCommand)
	}
}

func TestGroupNegativeCandidatesUsesNormalizedSambaRules(t *testing.T) {
	negatives := []report.ProjectNegativeCandidate{{
		Asset:       report.ProjectAsset{IP: "10.0.0.5", Port: 139, Protocol: "tcp"},
		Fingerprint: fingerprint.ServiceFingerprint{Service: "netbios-ssn", Product: "Samba smbd"},
		ZoneID:      "I",
	}}
	nseRules := map[string][]string{"smb": {"smb-protocols"}}
	tagRules := []vuln.TagRule{{Name: "smb", Service: []string{"smb"}, NucleiTags: []string{"smb"}, Target: "hostport"}}

	groups := groupNegativeCandidates(negatives, nseRules, tagRules)
	if len(groups) != 1 {
		t.Fatalf("expected 1 Samba group, got %d", len(groups))
	}
	if !strings.Contains(groups[0].NmapCommand, "smb-protocols") {
		t.Fatalf("expected Samba Nmap command, got %q", groups[0].NmapCommand)
	}
	if !strings.Contains(groups[0].NucleiCommand, "-tags smb") {
		t.Fatalf("expected Samba nuclei command, got %q", groups[0].NucleiCommand)
	}
	if groups[0].Title != "netbios-ssn / Samba smbd" || groups[0].PortsText != "139" {
		t.Fatalf("unexpected negative proof placeholders: %#v", groups[0])
	}
}

func TestGroupNegativeCandidatesKeepsZonesSeparate(t *testing.T) {
	negatives := []report.ProjectNegativeCandidate{
		{Asset: report.ProjectAsset{IP: "10.0.0.1", Port: 6379, Protocol: "tcp"}, Fingerprint: fingerprint.ServiceFingerprint{Service: "redis"}, ZoneID: "I"},
		{Asset: report.ProjectAsset{IP: "10.0.1.1", Port: 6379, Protocol: "tcp"}, Fingerprint: fingerprint.ServiceFingerprint{Service: "redis"}, ZoneID: "E"},
	}

	groups := groupNegativeCandidates(negatives, nil, nil)
	if len(groups) != 2 {
		t.Fatalf("expected separate groups for each zone, got %d: %#v", len(groups), groups)
	}
	if groups[0].ZoneID == groups[1].ZoneID || len(groups[0].Assets) != 1 || len(groups[1].Assets) != 1 {
		t.Fatalf("unexpected zone grouping: %#v", groups)
	}
}

func TestWorkbenchJSONUsesFieldNamesConsumedByJavaScript(t *testing.T) {
	candidateJSON, err := json.Marshal(workbenchCandidate{ZoneID: "I"})
	if err != nil {
		t.Fatal(err)
	}
	groupJSON, err := json.Marshal(negativeFingerprintGroup{Key: "I|redis", ZoneID: "I"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(candidateJSON), `"ZoneID":"I"`) || !strings.Contains(string(groupJSON), `"Key":"I|redis"`) || !strings.Contains(string(groupJSON), `"ZoneID":"I"`) {
		t.Fatalf("JSON fields must match Vue prop contract: candidate=%s group=%s", candidateJSON, groupJSON)
	}
}
