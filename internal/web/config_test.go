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
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /old/rustscan\n  nmap: /old/nmap\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 7, 21, 30, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("rustscan=/new/rustscan&nmap=/new/nmap&httpx=&nuclei=&ports=8080&profile=normal")
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
	if cfg.Tools.Rustscan != "/new/rustscan" || cfg.Scan.Ports != "8080" {
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
	form := strings.NewReader("rustscan=&nmap=&httpx=&nuclei=&ports=top1000&profile=normal&knowledge_base_path=../playbook/handbook.md")
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
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /opt/rustscan\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
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
	if !strings.Contains(body, "name=\"raw_config\"") || !strings.Contains(body, "高级 YAML") {
		t.Fatalf("expected raw editor in body: %s", body)
	}
}

func TestConfigPageRawEditorUpdatesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /old/rustscan\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("mode=raw&raw_config=tools%3A%0A++rustscan%3A+%2Fcustom%2Frustscan%0Ascan%3A%0A++ports%3A+8080%2C6379%0A++profile%3A+slow%0Aprofiles%3A%0A++slow%3A%0A++++host_workers%3A+1%0A")
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
	if cfg.Tools.Rustscan != "/custom/rustscan" || cfg.Scan.Profile != "slow" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestConfigPageRawEditorRejectsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	original := "tools:\n  rustscan: /old/rustscan\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"
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
