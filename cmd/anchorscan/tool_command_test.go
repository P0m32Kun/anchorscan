package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/tools"
)

func TestExecuteToolHelpShowsTools(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"tool", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	for _, want := range []string{"anchorscan tool", "rustscan", "nmap", "httpx", "nuclei"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in %q", want, stdout.String())
		}
	}
}

func TestExecuteToolNucleiRejectsMissingTagsAndTemplate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: rustscan\n  nmap: nmap\n  httpx: httpx\n  nuclei: nuclei\n")

	err := run([]string{
		"tool", "nuclei",
		"--config", configPath,
		"--db", filepath.Join(dir, "scan.db"),
		"--json", filepath.Join(dir, "report.json"),
		"--url", "http://example.test",
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "nuclei requires tags or template") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecuteToolNmapAliveWritesRunOutput(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	writeFile(t, configPath, "tools:\n  rustscan: rustscan\n  nmap: nmap\n  httpx: httpx\n  nuclei: nuclei\n")
	runner := &recordingRunner{outputs: [][]byte{[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`)}}

	var stdout bytes.Buffer
	err := run([]string{
		"tool", "nmap",
		"--config", configPath,
		"--db", dbPath,
		"--json", jsonPath,
		"--target", "192.0.2.10",
		"--mode", "alive",
		"--args", "--min-rate 50",
	}, &stdout, &bytes.Buffer{}, cliDeps{newRunner: func() tools.Runner { return runner }})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "run_id=tool-nmap-") || !strings.Contains(stdout.String(), "json="+jsonPath) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !runner.hasArgs("nmap", "-sn", "192.0.2.10", "-oX", "-", "--min-rate", "50") {
		t.Fatalf("commands = %#v", runner.commands)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
}
