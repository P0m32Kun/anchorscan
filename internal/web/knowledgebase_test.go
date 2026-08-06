package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const knowledgeBaseFixture = "<!-- anchorscan-catalog\nversion: 1\n-->\n\n### SMB 签名未启用（严重）\n\n<!-- anchorscan-entry\nid: smb-signing\naliases: []\nmatch:\n  nuclei: [smb-signing]\n  nse: []\n  manual-review: []\n  cve: []\n-->\n\n#### 漏洞描述\n\n描述。\n\n#### 验证命令\n\n#### 修复建议\n\n启用签名。\n"

func TestKnowledgeBaseListAndDetail(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "handbook.md"), []byte(knowledgeBaseFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: handbook.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	for _, path := range []string{"/kb?q=SMB", "/kb/smb-signing"} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "SMB 签名未启用") {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), ">严重<") || strings.Contains(res.Body.String(), ">critical<") {
			t.Fatalf("%s: critical severity is not localized: %s", path, res.Body.String())
		}
	}
}

func TestKnowledgeBaseLoadsExternalJSONCatalog(t *testing.T) {
	dir := t.TempDir()
	catalogSource, err := os.ReadFile(filepath.Clean("../knowledgebase/testdata/catalog-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(dir, "external-catalog.json")
	if err := os.WriteFile(catalogPath, catalogSource, 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: "+catalogPath+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/kb", nil))
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, "knowledgebase-status\">ready") || !strings.Contains(body, "SMB 签名未启用") || !strings.Contains(body, "/kb/nuclei-code") {
		t.Fatalf("external JSON catalog not loaded: %d %s", res.Code, body)
	}
	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/kb/smb-signing", nil))
	detailBody := detail.Body.String()
	for _, want := range []string{"agent:test", "来源与生成信息"} {
		if !strings.Contains(detailBody, want) {
			t.Fatalf("knowledge base detail does not surface audit sources: missing %q in %s", want, detailBody)
		}
	}
}

func TestKnowledgeBaseLegacyMarkdownLoadsAsLegacyUnknown(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	handbookPath, err := filepath.Abs(filepath.Clean("../knowledgebase/testdata/handbook-v2.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: "+handbookPath+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/kb/smb-signing", nil))
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, "legacy-unknown") || !strings.Contains(body, "旧 Markdown 未声明 safety") {
		t.Fatalf("legacy Markdown did not fail closed as legacy-unknown: %d %s", res.Code, body)
	}
}

func TestKnowledgeBaseMissingPathShowsUnavailableDiagnostic(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	missing := filepath.Join(dir, "does-not-exist.md")
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: "+missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/kb", nil))
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, "knowledgebase-status\">unavailable") || !strings.Contains(body, "no such file") || strings.Contains(body, "knowledgebase-entry") {
		t.Fatalf("missing knowledge base must show unavailable diagnostic without entries: %d %s", res.Code, body)
	}
}

func TestKnowledgeBaseIncompatibleFileShowsUnavailableDiagnostic(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("not a catalog"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: "+bad+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/kb", nil))
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, "knowledgebase-status\">unavailable") || !strings.Contains(body, "catalog JSON 无效") || strings.Contains(body, "knowledgebase-entry") {
		t.Fatalf("incompatible knowledge base must show unavailable diagnostic without entries: %d %s", res.Code, body)
	}
}

func TestKnowledgeBaseDetailWrapsLongTextAndNavHasIcon(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "handbook.md"), []byte(knowledgeBaseFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: handbook.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)

	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/kb/smb-signing", nil))
	body := detail.Body.String()
	if !strings.Contains(body, `class="panel knowledgebase-content"`) {
		t.Fatalf("knowledgebase detail missing wrapping container: %s", body)
	}
	if !strings.Contains(body, `id="nav-kb"><svg`) {
		t.Fatalf("knowledgebase nav missing icon: %s", body)
	}

	css := httptest.NewRecorder()
	handler.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/static/style.css", nil))
	if !strings.Contains(css.Body.String(), ".knowledgebase-content pre") || !strings.Contains(css.Body.String(), "overflow-wrap: anywhere") {
		t.Fatalf("knowledgebase wrapping styles missing: %s", css.Body.String())
	}
}
