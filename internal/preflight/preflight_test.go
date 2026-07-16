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
			Rustscan: filepath.Join(dir, "missing-rustscan"),
			Nmap:     filepath.Join(dir, "missing-nmap"),
		},
	})

	if len(result.Errors) < 3 {
		t.Fatalf("expected multiple blocking errors, got %#v", result.Errors)
	}
	if !result.HasErrors() {
		t.Fatal("expected HasErrors to be true")
	}
}

func TestRunBlocksInvalidPortExpression(t *testing.T) {
	dir := t.TempDir()
	rustscan := executable(t, dir, "rustscan")
	nmap := executable(t, dir, "nmap")

	result := Run(Options{
		ConfigDir: dir,
		DBPath:    filepath.Join(dir, "data", "scan.db"),
		JSONPath:  filepath.Join(dir, "reports", "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "eighty",
		Tools:     config.ToolPaths{Rustscan: rustscan, Nmap: nmap},
	})

	if !result.HasErrors() {
		t.Fatalf("expected invalid port error, got %#v", result)
	}
}

func TestRunSummarizesScanAndWarnsForFullRange(t *testing.T) {
	dir := t.TempDir()
	rustscan := executable(t, dir, "rustscan")
	nmap := executable(t, dir, "nmap")

	result := Run(Options{
		ConfigDir: filepath.Join(dir, "config"),
		DBPath:    filepath.Join(dir, "data", "scan.db"),
		JSONPath:  filepath.Join(dir, "reports", "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "1-65535",
		Tools:     config.ToolPaths{Rustscan: rustscan, Nmap: nmap},
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
	executable(t, dir, "rustscan")
	executable(t, dir, "nmap")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result := Run(Options{
		ConfigDir: dir,
		DBPath:    filepath.Join(dir, "scan.db"),
		JSONPath:  filepath.Join(dir, "scan.json"),
		Targets:   []string{"127.0.0.1"},
		PortSpec:  "22",
		Tools:     config.ToolPaths{Rustscan: "rustscan", Nmap: "nmap"},
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
