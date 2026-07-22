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
