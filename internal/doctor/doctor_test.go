package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunReportsMissingTool(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`tools:
  rustscan: /missing/rustscan
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
	if !containsCheck(checks, "rustscan", false) {
		t.Fatalf("expected rustscan failure: %#v", checks)
	}
}

func TestRunReportsDatabaseMigrationFailure(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
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
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: filepath.Join(dir, "reports")})
	if !containsCheck(checks, "database", true) {
		t.Fatalf("expected database ok: %#v", checks)
	}
}

func containsCheck(checks []Check, name string, ok bool) bool {
	for _, check := range checks {
		if check.Name == name && check.OK == ok {
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
}

func writeExecutable(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
	return path
}
