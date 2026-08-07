package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
)

func TestConfigPageUpdatesToolPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  nmap: /old/nmap\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 7, 21, 30, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("nmap=/new/nmap&httpx=&nuclei=&nuclei_templates=~/nuclei-templates&ports=8080&profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Tools.Nmap != "/new/nmap" || cfg.Tools.NucleiTemplates != "~/nuclei-templates" || cfg.Scan.Ports != "8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestConfigPageUpdatesKnowledgeBasePathAndShowsRestartNotice(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools: {}\nscan:\n  ports: top1000\n  profile: normal\nprofiles: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	form := strings.NewReader("nmap=&httpx=&nuclei=&ports=top1000&profile=normal&knowledge_base_path=../playbook/handbook.md")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != "/config?saved=1" {
		t.Fatalf("redirect = %d %q", res.Code, res.Header().Get("Location"))
	}
	cfg, err := config.Load(configPath)
	if err != nil || cfg.KnowledgeBase.Path != "../playbook/handbook.md" {
		t.Fatalf("config = %#v, err = %v", cfg, err)
	}
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/config?saved=1", nil))
	if !strings.Contains(res.Body.String(), "重启 AnchorScan 后生效") {
		t.Fatalf("missing restart notice: %s", res.Body.String())
	}
}

func TestConfigPageRendersAdvancedEditor(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  nuclei_templates: ~/nuclei-templates\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/config", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "name=\"raw_config\"") || !strings.Contains(body, "高级 YAML") || !strings.Contains(body, "name=\"timeout_fathom\"") || !strings.Contains(body, "value=\"0\"") || !strings.Contains(body, "name=\"nuclei_templates\"") || !strings.Contains(body, "value=\"~/nuclei-templates\"") {
		t.Fatalf("expected raw editor in body: %s", body)
	}
}

func TestConfigPageUpdatesToolTimeouts(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools: {}\nscan:\n  ports: top1000\n  profile: normal\nprofiles: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	form := strings.NewReader("nmap=&httpx=&nuclei=&ports=top1000&profile=normal&timeout_fathom=30s&timeout_nmap=0&timeout_httpx=150ms&timeout_nse=5m&timeout_nuclei=1m")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Timeouts.Fathom, "30s"; got != want {
		t.Fatalf("fathom timeout = %q, want %q", got, want)
	}
	if got, want := cfg.Timeouts.NSE, "5m"; got != want {
		t.Fatalf("NSE timeout = %q, want %q", got, want)
	}
}

func TestConfigPageRejectsInvalidToolTimeout(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools: {}\nscan:\n  ports: top1000\n  profile: normal\nprofiles: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	form := strings.NewReader("nmap=&httpx=&nuclei=&ports=top1000&profile=normal&timeout_fathom=-1s")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "invalid fathom timeout") {
		t.Fatalf("expected timeout validation error, got %d: %s", res.Code, res.Body.String())
	}
}

func TestConfigPageRawEditorUpdatesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  nmap: /old/nmap\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("mode=raw&raw_config=tools%3A%0A++nmap%3A+%2Fcustom%2Fnmap%0Ascan%3A%0A++ports%3A+8080%2C6379%0A++profile%3A+slow%0Aprofiles%3A%0A++slow%3A%0A++++host_workers%3A+1%0A")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Tools.Nmap != "/custom/nmap" || cfg.Scan.Profile != "slow" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestConfigPageRawEditorRejectsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	original := "tools:\n  nmap: /old/nmap\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("mode=raw&raw_config=tools%3A+%5Bbroken")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "invalid") || !strings.Contains(res.Body.String(), "raw_config") {
		t.Fatalf("expected validation message and raw editor: %s", res.Body.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("config should remain unchanged: %s", data)
	}
}

func TestConfigPageShowsToolDiagnosticsForMissingTools(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	// All tool paths empty: the diagnostics must surface clear missing/not-configured hints.
	if err := os.WriteFile(configPath, []byte("tools:\n  nmap: ''\n  httpx: ''\n  nuclei: ''\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/config", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "config-tool-diagnostics") {
		t.Fatalf("config page must render the tool diagnostics section")
	}
	if !strings.Contains(body, "config-tool-check status-fail") {
		t.Fatalf("missing required tools must produce a fail-status diagnostic: %s", body)
	}
	// Each external tool must appear by name so the hint is actionable.
	for _, tool := range []string{"fathom", "nmap"} {
		if !strings.Contains(body, tool) {
			t.Fatalf("config page diagnostics must mention required tool %q", tool)
		}
	}
}
