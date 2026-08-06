package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestCommandGateTokensAreRequestBoundAndRestartLocal(t *testing.T) {
	now := time.Unix(1, 0)
	store := newCommandGateStore()
	token, err := store.issueChallenge("request-a", now)
	if err != nil {
		t.Fatal(err)
	}
	if store.consumeChallenge(token, "request-b", now) {
		t.Fatal("challenge token authorized a different request")
	}
	restartToken, err := store.issueChallenge("request-a", now)
	if err != nil {
		t.Fatal(err)
	}
	if newCommandGateStore().consumeChallenge(restartToken, "request-a", now) {
		t.Fatal("challenge token survived an in-memory store restart")
	}
}

func TestCommandGateViewForEntryUsesFiveCatalogLevels(t *testing.T) {
	base := knowledgebase.Entry{ID: "entry", ReviewStatus: knowledgebase.ReviewStatusStable, Safety: knowledgebase.Safety{Mode: knowledgebase.SafetySafe}, Commands: knowledgebase.Commands{Nuclei: "nuclei -t test.yaml -u {{host}}:{{port}}"}}
	cases := []struct {
		name     string
		entry    knowledgebase.Entry
		level    string
		required bool
	}{
		{name: "stable safe", entry: base, level: "safe", required: false},
		{name: "needs review safe", entry: func() knowledgebase.Entry {
			value := base
			value.ReviewStatus = knowledgebase.ReviewStatusNeedsReview
			return value
		}(), level: "needs-review", required: true},
		{name: "optional", entry: func() knowledgebase.Entry {
			value := base
			value.Safety = knowledgebase.Safety{Mode: knowledgebase.SafetyOptional, Effects: []string{"authentication-attempt"}}
			return value
		}(), level: "optional", required: true},
		{name: "manual gated", entry: func() knowledgebase.Entry {
			value := base
			value.Safety = knowledgebase.Safety{Mode: knowledgebase.SafetyManualGated, Effects: []string{"file-read"}}
			return value
		}(), level: "manual-gated", required: true},
		{name: "legacy unknown", entry: func() knowledgebase.Entry {
			value := base
			value.ReviewStatus = knowledgebase.ReviewStatusLegacyUnknown
			value.Safety = knowledgebase.Safety{Mode: knowledgebase.SafetyLegacyUnknown}
			return value
		}(), level: "legacy-unknown", required: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view, err := commandGateViewForEntry(tc.entry, "nuclei", nil)
			if err != nil || view.Level != tc.level || commandGateRequired(view) != tc.required {
				t.Fatalf("view=%#v required=%t err=%v", view, commandGateRequired(view), err)
			}
		})
	}
}

func postCommandWithGate(t *testing.T, handler http.Handler, path, form string) *httptest.ResponseRecorder {
	t.Helper()
	post := func(value string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(value))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	res := post(form)
	if res.Code != http.StatusPreconditionRequired {
		return res
	}
	body := res.Body.String()
	for _, forbidden := range []string{"\"full_command\"", "\"tool_args\"", "\"tool_link\"", "\"commands\"", "raw_args"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unconfirmed command response leaked %q: %s", forbidden, body)
		}
	}
	var challenge struct {
		Gate commandGateView `json:"gate"`
	}
	if err := json.Unmarshal([]byte(body), &challenge); err != nil {
		t.Fatalf("decode command gate challenge: %v", err)
	}
	if challenge.Gate.Challenge == "" {
		t.Fatal("command gate challenge token is empty")
	}
	values, err := url.ParseQuery(form)
	if err != nil {
		t.Fatalf("parse command form: %v", err)
	}
	values.Set("gate_token", challenge.Gate.Challenge)
	values.Set("acknowledge", "1")
	return post(values.Encode())
}

func gateCatalogEntry(id, status, mode string, effects []string, cleanup, commandTemplate string) map[string]any {
	command := "nuclei -t " + commandTemplate + " -u {{host}}:{{port}}"
	entry := map[string]any{
		"id":       id,
		"title":    id + " title",
		"severity": "高危",
		"status":   status,
		"safety":   map[string]any{"mode": mode, "effects": effects},
		"match": map[string]any{
			"nuclei": []string{id}, "nse": []string{}, "manual-review": []string{}, "cve": []string{},
		},
		"sections":  map[string]string{"漏洞描述": id + " description", "修复建议": id + " remediation"},
		"verify":    map[string]any{"tool": "nuclei", "template": commandTemplate, "target": "host:port"},
		"command":   command,
		"sources":   []string{"https://example.test/" + id},
		"generated": map[string]string{"by": "test", "at": "2026-01-01T00:00:00Z"},
	}
	if cleanup != "" {
		entry["safety"].(map[string]any)["cleanup"] = cleanup
	}
	return entry
}

func setupCommandGateCatalog(t *testing.T) (http.Handler, map[string]report.Finding) {
	t.Helper()
	dir := t.TempDir()
	entries := []map[string]any{
		gateCatalogEntry("safe-entry", "stable", "safe", []string{}, "", "network/safe.yaml"),
		gateCatalogEntry("review-entry", "needs-review", "safe", []string{}, "", "network/review.yaml"),
		gateCatalogEntry("optional-entry", "stable", "optional", []string{"authentication-attempt"}, "停止认证尝试", "network/optional.yaml"),
		gateCatalogEntry("manual-entry", "stable", "manual-gated", []string{"file-read", "test-file-create"}, "删除测试文件", "network/manual.yaml"),
		gateCatalogEntry("invalid-command", "stable", "safe", []string{}, "", "network/expected.yaml"),
		gateCatalogEntry("invalid-safety", "stable", "safe", []string{"authentication-attempt"}, "", "network/invalid-safety.yaml"),
		gateCatalogEntry("no-command-entry", "stable", "safe", []string{}, "", "network/no-command.yaml"),
	}
	entries[4]["command"] = "nuclei -t network/other.yaml -u {{host}}:{{port}}"
	entries[6]["command"] = ""
	catalog, err := json.Marshal(map[string]any{"version": 2, "source": "handbook-v3", "entry_count": len(entries), "entries": entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), catalog, 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: catalog.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "scan.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveScanRun(store.ScanRun{RunID: "gate-run", Status: "completed", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	findings := make(map[string]report.Finding, len(entries))
	for i, entry := range entries {
		id := entry["id"].(string)
		finding := report.Finding{IP: "192.0.2." + string(rune('1'+i)), Port: 443, Protocol: "tcp", Source: "nuclei", ID: id, Severity: "high", Summary: id + " title"}
		if err := st.SaveFinding("gate-run", finding); err != nil {
			t.Fatal(err)
		}
		findings[id] = finding
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	return handler, findings
}

func postGateRequest(handler http.Handler, path string, values url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decodeGateChallenge(t *testing.T, res *httptest.ResponseRecorder) commandGateView {
	t.Helper()
	if res.Code != http.StatusPreconditionRequired {
		t.Fatalf("gate status=%d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, forbidden := range []string{"full_command", "tool_args", "tool_link", "raw_args", "nuclei -t"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unconfirmed response leaked %q: %s", forbidden, body)
		}
	}
	var payload struct {
		Gate commandGateView `json:"gate"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Gate
}

func TestCommandGateEnforcesCatalogSafetyAndOneTimeAcknowledgement(t *testing.T) {
	handler, findings := setupCommandGateCatalog(t)
	path := "/reports/gate-run/commands"

	safe := postGateRequest(handler, path, url.Values{"finding_key": {report.FindingKey(findings["safe-entry"])}, "tool": {"nuclei"}})
	if safe.Code != http.StatusOK || !strings.Contains(safe.Body.String(), "full_command") {
		t.Fatalf("stable safe command=%d %s", safe.Code, safe.Body.String())
	}

	cases := []struct {
		id      string
		level   string
		review  string
		effects []string
		cleanup string
		message string
	}{
		{id: "review-entry", level: "needs-review", review: "needs-review", message: "待复核"},
		{id: "optional-entry", level: "optional", review: "stable", effects: []string{"authentication-attempt"}, cleanup: "停止认证尝试"},
		{id: "manual-entry", level: "manual-gated", review: "stable", effects: []string{"file-read", "test-file-create"}, cleanup: "删除测试文件"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			values := url.Values{
				"finding_key": {report.FindingKey(findings[tc.id])},
				"tool":        {"nuclei"},
				"mode":        {"safe"},
				"effects":     {""},
				"cleanup":     {"attacker-controlled"},
				"confirmed":   {"1"},
			}
			gate := decodeGateChallenge(t, postGateRequest(handler, path+"?safety_mode=safe&review_status=stable", values))
			if gate.Level != tc.level || gate.ReviewStatus != tc.review || strings.Join(gate.Effects, ",") != strings.Join(tc.effects, ",") || gate.Cleanup != tc.cleanup || !strings.Contains(gate.Message, tc.message) {
				t.Fatalf("gate=%#v", gate)
			}
			values.Set("gate_token", gate.Challenge)
			values.Set("acknowledge", "1")
			confirmed := postGateRequest(handler, path+"?safety_mode=safe&review_status=stable", values)
			if confirmed.Code != http.StatusOK || !strings.Contains(confirmed.Body.String(), "full_command") || !strings.Contains(confirmed.Body.String(), "tool_link") {
				t.Fatalf("confirmed command=%d %s", confirmed.Code, confirmed.Body.String())
			}
			replayed := postGateRequest(handler, path+"?safety_mode=safe&review_status=stable", values)
			if replayed.Code != http.StatusForbidden || strings.Contains(replayed.Body.String(), "full_command") {
				t.Fatalf("replayed challenge=%d %s", replayed.Code, replayed.Body.String())
			}
		})
	}
}

func TestCommandGateRejectsMissingOrInvalidCatalogDataWithoutLeakingCommand(t *testing.T) {
	handler, findings := setupCommandGateCatalog(t)
	path := "/reports/gate-run/commands"
	for _, id := range []string{"invalid-command", "invalid-safety"} {
		res := postGateRequest(handler, path, url.Values{"finding_key": {report.FindingKey(findings[id])}, "tool": {"nuclei"}, "mode": {"safe"}, "confirmed": {"1"}})
		if res.Code != http.StatusUnprocessableEntity && res.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", id, res.Code, res.Body.String())
		}
		for _, forbidden := range []string{"full_command", "tool_args", "tool_link", "raw_args", "nuclei -t"} {
			if strings.Contains(res.Body.String(), forbidden) {
				t.Fatalf("%s leaked %q: %s", id, forbidden, res.Body.String())
			}
		}
	}
}

func TestToolPrefillGrantIsOneTimeAndIgnoresForgedToken(t *testing.T) {
	handler, findings := setupCommandGateCatalog(t)
	res := postGateRequest(handler, "/reports/gate-run/commands", url.Values{"finding_key": {report.FindingKey(findings["safe-entry"])}, "tool": {"nuclei"}})
	if res.Code != http.StatusOK {
		t.Fatalf("safe command=%d %s", res.Code, res.Body.String())
	}
	var command struct {
		ToolLink string `json:"tool_link"`
		ToolArgs string `json:"tool_args"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &command); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command.ToolLink, "gate_token=") || strings.Contains(command.ToolLink, "raw_args=") {
		t.Fatalf("tool link=%q", command.ToolLink)
	}
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, command.ToolLink, nil))
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), command.ToolArgs) {
		t.Fatalf("first prefill=%d %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, command.ToolLink, nil))
	if strings.Contains(second.Body.String(), command.ToolArgs) {
		t.Fatalf("replayed prefill leaked args: %s", second.Body.String())
	}
	forged := httptest.NewRecorder()
	forgedPath := "/tools/nuclei?gate_token=forged&raw_args=" + url.QueryEscape(command.ToolArgs)
	handler.ServeHTTP(forged, httptest.NewRequest(http.MethodGet, forgedPath, nil))
	if strings.Contains(forged.Body.String(), command.ToolArgs) {
		t.Fatalf("forged prefill leaked args: %s", forged.Body.String())
	}
	handcrafted := httptest.NewRecorder()
	handler.ServeHTTP(handcrafted, httptest.NewRequest(http.MethodGet, "/tools/nuclei?raw_args="+url.QueryEscape(command.ToolArgs), nil))
	if strings.Contains(handcrafted.Body.String(), command.ToolArgs) {
		t.Fatalf("hand-crafted raw_args query must not prefill catalog commands: %s", handcrafted.Body.String())
	}
}

func TestLegacyCommandAndKnowledgeBaseDetailFailClosed(t *testing.T) {
	handler, projectID, runID, dbPath := setupWorkbenchProject(t)
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := st.ListFindings(runID)
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings=%#v err=%v", findings, err)
	}
	_ = st.Close()
	values := url.Values{"finding_key": {report.FindingKey(findings[0])}, "tool": {"nuclei"}, "mode": {"safe"}, "confirmed": {"1"}}
	gate := decodeGateChallenge(t, postGateRequest(handler, "/reports/"+runID+"/commands", values))
	if gate.Level != "legacy-unknown" || gate.ReviewStatus != "legacy-unknown" || gate.SafetyMode != "legacy-unknown" || !strings.Contains(gate.Message, "旧 Markdown 未声明 safety") {
		t.Fatalf("legacy gate=%#v", gate)
	}
	values.Set("gate_token", gate.Challenge)
	values.Set("acknowledge", "1")
	confirmed := postGateRequest(handler, "/reports/"+runID+"/commands", values)
	if confirmed.Code != http.StatusOK || !strings.Contains(confirmed.Body.String(), "full_command") {
		t.Fatalf("legacy confirmed=%d %s", confirmed.Code, confirmed.Body.String())
	}
	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/kb/redis-default-login", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "旧 Markdown 未声明 safety") || !strings.Contains(detail.Body.String(), "legacy-unknown") || !strings.Contains(detail.Body.String(), "nuclei -t redis-default-logins") {
		t.Fatalf("legacy KB detail status=%d legacy-message=%t legacy-status=%t command-shown=%t", detail.Code, strings.Contains(detail.Body.String(), "旧 Markdown 未声明 safety"), strings.Contains(detail.Body.String(), "legacy-unknown"), strings.Contains(detail.Body.String(), "nuclei -t redis-default-logins"))
	}
	_ = projectID
}

func TestKnowledgeBaseDetailShowsCommandsForAllEntries(t *testing.T) {
	handler, _ := setupCommandGateCatalog(t)
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{id: "safe-entry", want: []string{"stable", "safe", "nuclei -t network/safe.yaml"}},
		{id: "review-entry", want: []string{"needs-review", "待复核", "acknowledgement", "nuclei -t network/review.yaml"}},
		{id: "optional-entry", want: []string{"optional", "authentication-attempt", "停止认证尝试", "nuclei -t network/optional.yaml"}},
		{id: "manual-entry", want: []string{"manual-gated", "file-read", "test-file-create", "删除测试文件", "nuclei -t network/manual.yaml"}},
		{id: "no-command-entry", want: []string{"stable", "知识库未提供可用命令"}},
	} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/kb/"+tc.id, nil))
		if res.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", tc.id, res.Code, res.Body.String())
		}
		for _, want := range tc.want {
			if !strings.Contains(res.Body.String(), want) {
				t.Fatalf("%s missing %q: %s", tc.id, want, res.Body.String())
			}
		}
	}
}
