package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesToolPathsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
tools:
  rustscan: /usr/local/bin/rustscan
  nmap: /usr/local/bin/nmap
  httpx: /usr/local/bin/httpx
  nuclei: /usr/local/bin/nuclei
scan:
  ports: top100
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Tools.Rustscan != "/usr/local/bin/rustscan" {
		t.Fatalf("unexpected rustscan path: %q", cfg.Tools.Rustscan)
	}
	if cfg.Scan.Ports != "top100" {
		t.Fatalf("unexpected default ports: %q", cfg.Scan.Ports)
	}
}

func TestLoadTagRulesParsesSnakeCaseFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service-tags.yaml")
	content := []byte(`
- name: tomcat
  service: ["http"]
  product: ["apache tomcat"]
  tech: ["tomcat"]
  nuclei_tags: ["tomcat", "apache-tomcat"]
  target: "url"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadTagRules(path)
	if err != nil {
		t.Fatalf("LoadTagRules returned error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("unexpected rules: %#v", rules)
	}
	if got := rules[0].NucleiTags; len(got) != 2 || got[0] != "tomcat" || rules[0].Target != "url" {
		t.Fatalf("unexpected parsed rule: %#v", rules[0])
	}
}
