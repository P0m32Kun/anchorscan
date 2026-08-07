package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFailsWhenEnabledRuleFilesAreMissingOrEmpty(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	t.Chdir(dir)

	writeFile(t, filepath.Join(dir, "nse.yaml"), "")
	writeFile(t, filepath.Join(dir, "service-tags.yaml"), "")
	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if !containsCheck(checks, "nse rules", false) || !containsCheck(checks, "tag rules", false) {
		t.Fatalf("empty enabled rules must fail: %#v", checks)
	}
}

func TestConfiguredInvalidRdpscanFails(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\n  rdpscan: /missing/rdpscan\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "nse.yaml"), "ssh: [ssh-hostkey]\n")
	writeFile(t, filepath.Join(dir, "service-tags.yaml"), "- name: ssh\n  service: [ssh]\n  nuclei_tags: [ssh]\n")

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if !containsCheck(checks, "rdpscan", false) || !HasFailures(checks) {
		t.Fatalf("configured invalid rdpscan must fail: %#v", checks)
	}
}

func TestRunReportsMissingTool(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`tools:
  nmap: /missing/nmap
scan:
  ports: top1000
  profile: normal
profiles:
  normal:
    host_workers: 1
`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if !HasFailures(checks) {
		t.Fatalf("expected failures: %#v", checks)
	}
	if !containsCheck(checks, "nmap", false) {
		t.Fatalf("expected nmap failure: %#v", checks)
	}
}

func TestRunReportsDatabaseMigrationFailure(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")
	badDB := filepath.Join(dir, "scan.db")
	writeFile(t, badDB, "not sqlite")

	checks := Run(Options{ConfigPath: configPath, DBPath: badDB, ReportDir: filepath.Join(dir, "reports")})
	if !containsCheck(checks, "database", false) {
		t.Fatalf("expected database failure: %#v", checks)
	}
}

func TestRunChecksDatabaseCanOpen(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: filepath.Join(dir, "reports")})
	if !containsCheck(checks, "database", true) {
		t.Fatalf("expected database ok: %#v", checks)
	}
}

func TestFathomMissingFails(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	// fathom is the sole alive/port/fingerprint engine (spec v2.0); doctor
	// must surface a missing path as a fail even though the blocking logic
	// lives in preflight.
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if !containsCheck(checks, "fathom", false) {
		t.Fatalf("missing fathom must fail doctor: %#v", checks)
	}
}

func containsCheck(checks []Check, name string, ok bool) bool {
	status := StatusFail
	if ok {
		status = StatusOK
	}
	return containsStatus(checks, name, status)
}

func containsStatus(checks []Check, name string, status Status) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
	if filepath.Ext(path) == ".yaml" && strings.Contains(content, "scan:") {
		dir := filepath.Dir(path)
		if err := os.WriteFile(filepath.Join(dir, "nse.yaml"), []byte("http:\n  - http-title\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "service-tags.yaml"), []byte("- name: http\n  service: [http]\n  nuclei_tags: [http]\n  target: url\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeExecutable(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
	return path
}

func TestRunFailsWhenRuleFilesAreMissing(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	for _, fileName := range []string{"nse.yaml", "service-tags.yaml"} {
		if err := os.Remove(filepath.Join(dir, fileName)); err != nil {
			t.Fatal(err)
		}
	}

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if !containsCheck(checks, "nse rules", false) || !containsCheck(checks, "tag rules", false) {
		t.Fatalf("expected missing rule failures: %#v", checks)
	}
}

func TestRdpscanMissingReportsOptionalHint(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\n  fathom: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "nse.yaml"), "ssh: [ssh-hostkey]\n")
	writeFile(t, filepath.Join(dir, "service-tags.yaml"), "- name: ssh\n  service: [ssh]\n  nuclei_tags: [ssh]\n")

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if HasFailures(checks) {
		t.Fatalf("rdpscan missing should not make doctor fail: %#v", checks)
	}
	if !containsStatus(checks, "rdpscan", StatusWarning) {
		t.Fatalf("expected rdpscan warning: %#v", checks)
	}
	var found bool
	for _, c := range checks {
		if c.Name == "rdpscan" {
			found = true
			if !strings.Contains(c.Message, "not installed") {
				t.Fatalf("expected rdpscan hint, got %q", c.Message)
			}
		}
	}
	if !found {
		t.Fatalf("rdpscan check not found: %#v", checks)
	}
}
