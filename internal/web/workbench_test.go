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
// includes the negative candidate section and the incomplete check section when
// the appropriate detection check data is present.
func TestWorkbenchPageRendersNegativeAndIncomplete(t *testing.T) {
	handler, projectID, _ := setupNegativeWorkbenchProject(t)

	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	// negative candidate port 80 should appear
	if !strings.Contains(body, "10.0.0.1:80") {
		t.Fatalf("expected negative candidate 10.0.0.1:80 in body")
	}
	// incomplete check port 443 should appear
	if !strings.Contains(body, "10.0.0.1:443") {
		t.Fatalf("expected incomplete check 10.0.0.1:443 in body")
	}
	// queue tabs should be rendered
	if !strings.Contains(body, "queue-negative") {
		t.Fatalf("expected queue-negative panel in body")
	}
	if !strings.Contains(body, "queue-incomplete") {
		t.Fatalf("expected queue-incomplete panel in body")
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

	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/workbench", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()

	// Extract the JSON embedded in the page.
	negStart := strings.Index(body, `id="negative-data"`)
	incStart := strings.Index(body, `id="incomplete-data"`)
	if negStart < 0 || incStart < 0 {
		t.Fatalf("expected embedded JSON blocks, got: (truncated)")
	}

	extractJSON := func(marker string) string {
		start := strings.Index(body, marker)
		if start < 0 {
			return ""
		}
		after := body[start:]
		open := strings.Index(after, ">")
		if open < 0 {
			return ""
		}
		rest := after[open+1:]
		close := strings.Index(rest, "</script>")
		if close < 0 {
			return ""
		}
		return strings.TrimSpace(rest[:close])
	}

	negJSON := extractJSON(`id="negative-data"`)
	incJSON := extractJSON(`id="incomplete-data"`)

	var negItems []map[string]any
	var incItems []map[string]any
	if err := json.Unmarshal([]byte(negJSON), &negItems); err != nil {
		t.Fatalf("unmarshal negative-data returned error: %v — json: %s", err, negJSON)
	}
	if err := json.Unmarshal([]byte(incJSON), &incItems); err != nil {
		t.Fatalf("unmarshal incomplete-data returned error: %v — json: %s", err, incJSON)
	}

	// Negative queue: only port 80 (both completed)
	if len(negItems) != 1 {
		t.Fatalf("expected 1 negative item, got %d", len(negItems))
	}
	asset := negItems[0]["Asset"].(map[string]any)
	if int(asset["Port"].(float64)) != 80 {
		t.Fatalf("expected port 80 in negative queue, got %v", asset["Port"])
	}

	// Incomplete queue: only port 443 (nuclei failed)
	if len(incItems) != 1 {
		t.Fatalf("expected 1 incomplete item, got %d", len(incItems))
	}
	incAsset := incItems[0]["Asset"].(map[string]any)
	if int(incAsset["Port"].(float64)) != 443 {
		t.Fatalf("expected port 443 in incomplete queue, got %v", incAsset["Port"])
	}
}
