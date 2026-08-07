package preflight

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/config"
)

func TestRunBlocksMissingTargetsInvalidPortsAndRequiredTools(t *testing.T) {
	dir := t.TempDir()
	result := Run(Options{
		ConfigDir: filepath.Join(dir, "config"),
		DBPath:    filepath.Join(dir, "data", "scan.db"),
		JSONPath:  filepath.Join(dir, "reports", "scan.json"),
		Targets:   nil,
		PortSpec:  "top1000",
		Tools: config.ToolPaths{
			Fathom: filepath.Join(dir, "missing-fathom"),
			Nmap:   filepath.Join(dir, "missing-nmap"),
		},
	})

	if len(result.Errors) < 3 {
		t.Fatalf("expected multiple blocking errors, got %#v", result.Errors)
	}
	if !result.HasErrors() {
		t.Fatal("expected HasErrors to be true")
	}
	for _, message := range result.Errors {
		if message.Field == "fathom" {
			return // fathom is reported as a blocking error, not a warning
		}
	}
	t.Fatalf("missing fathom blocking error in %#v", result.Errors)
}

// TestRunBlocksMissingFathom pins the M4.2 acceptance: an unconfigured fathom
// is a blocking preflight error (spec v2.0 — no legacy fallback), with the
// exact message from the brief.
func TestRunBlocksMissingFathom(t *testing.T) {
	result := Run(Options{
		ConfigDir: t.TempDir(),
		DBPath:    filepath.Join(t.TempDir(), "scan.db"),
		JSONPath:  filepath.Join(t.TempDir(), "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "22",
		Tools:     config.ToolPaths{Nmap: executable(t, t.TempDir(), "nmap")},
	})
	if !result.HasErrors() {
		t.Fatalf("expected fathom error, got %#v", result)
	}
	var found bool
	for _, message := range result.Errors {
		if message.Field == "fathom" && message.Message == "fathom is required but not configured. Set tools.fathom in config." {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected exact fathom error message in %#v", result.Errors)
	}
}

func TestRunBlocksInvalidPortExpression(t *testing.T) {
	dir := t.TempDir()
	fathom := executable(t, dir, "fathom")
	nmap := executable(t, dir, "nmap")

	result := Run(Options{
		ConfigDir: dir,
		DBPath:    filepath.Join(dir, "data", "scan.db"),
		JSONPath:  filepath.Join(dir, "reports", "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "eighty",
		Tools:     config.ToolPaths{Fathom: fathom, Nmap: nmap},
	})

	if !result.HasErrors() {
		t.Fatalf("expected invalid port error, got %#v", result)
	}
}

func TestRunSummarizesScanAndWarnsForFullRange(t *testing.T) {
	dir := t.TempDir()
	fathom := executable(t, dir, "fathom")
	nmap := executable(t, dir, "nmap")

	result := Run(Options{
		ConfigDir: filepath.Join(dir, "config"),
		DBPath:    filepath.Join(dir, "data", "scan.db"),
		JSONPath:  filepath.Join(dir, "reports", "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "1-65535",
		Tools:     config.ToolPaths{Fathom: fathom, Nmap: nmap},
		Profile:   "fast",
		Workers:   4,
		ExtraArgs: config.ToolArgs{
			Rustscan: []string{"--batch-size", "1000"},
			Nmap:     []string{"-T4"},
		},
		NSERuleCount: 2,
		TagRuleCount: 3,
	})

	if result.HasErrors() {
		t.Fatalf("expected no errors, got %#v", result.Errors)
	}
	if result.Summary.TargetCount != 1 || result.Summary.PortSpec != "1-65535" || result.Summary.Profile != "fast" {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected full range warning")
	}
}

func TestRunAcceptsToolNamesFromPATH(t *testing.T) {
	dir := t.TempDir()
	executable(t, dir, "fathom")
	executable(t, dir, "nmap")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result := Run(Options{
		ConfigDir: dir,
		DBPath:    filepath.Join(dir, "scan.db"),
		JSONPath:  filepath.Join(dir, "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "22",
		Tools:     config.ToolPaths{Fathom: "fathom", Nmap: "nmap"},
	})
	if result.HasErrors() {
		t.Fatalf("PATH tool names rejected: %#v", result.Errors)
	}
}

func executable(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}
