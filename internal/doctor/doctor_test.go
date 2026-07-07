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
  ports: top100
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

func containsCheck(checks []Check, name string, ok bool) bool {
	for _, check := range checks {
		if check.Name == name && check.OK == ok {
			return true
		}
	}
	return false
}
