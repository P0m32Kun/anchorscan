package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

func TestExecuteToolsCheckReportsConfiguredTools(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, filepath.Join(dir, "rustscan"), "")
	writeFile(t, filepath.Join(dir, "nmap"), "")
	writeFile(t, filepath.Join(dir, "httpx"), "")
	writeFile(t, filepath.Join(dir, "nuclei"), "")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "rustscan")+"\n  nmap: "+filepath.Join(dir, "nmap")+"\n  httpx: "+filepath.Join(dir, "httpx")+"\n  nuclei: "+filepath.Join(dir, "nuclei")+"\n")

	var stdout bytes.Buffer
	err := run([]string{"tools", "check", "--config", configPath}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	for _, name := range []string{"rustscan: ok", "nmap: ok", "httpx: ok", "nuclei: ok"} {
		if !strings.Contains(output, name) {
			t.Fatalf("expected %q in output %q", name, output)
		}
	}
}

func TestExecuteRootHelpShowsCommands(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{"Usage:", "scan", "report", "tools check"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

func TestExecuteScanHelpShowsFlags(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"scan", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{"Usage: anchorscan scan", "--target", "--ports", "--json", "--html"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

func TestExecuteScanWritesJSONAndHTML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	htmlPath := filepath.Join(dir, "report.html")

	writeFile(t, configPath, "tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: /opt/httpx\nscan:\n  ports: 8080\n")

	runner := &fakeRunner{
		outputs: [][]byte{
			[]byte("Open 8080\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat" version="9.0.65"/></port></ports></host></nmaprun>`),
			[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "192.168.1.10",
		"--db", dbPath,
		"--json", jsonPath,
		"--html", htmlPath,
	}, &stdout, &stderr, cliDeps{
		newRunner: func() tools.Runner { return runner },
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	for _, path := range []string{jsonPath, htmlPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}
	if !strings.Contains(stdout.String(), "run_id=") {
		t.Fatalf("expected run id output, got %q", stdout.String())
	}
	for _, line := range []string{
		"[scan] run",
		"[scan] target 192.168.1.10",
		"[scan] rustscan",
		"[scan] nmap",
		"[scan] httpx",
		"[scan] report json",
		"[scan] report html",
		"[scan] done",
	} {
		if !strings.Contains(stderr.String(), line) {
			t.Fatalf("expected log %q in stderr %q", line, stderr.String())
		}
	}
}

func TestExecuteReportWritesHTMLFromStoredRun(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	htmlPath := filepath.Join(dir, "report.html")
	jsonPath := filepath.Join(dir, "report.json")
	writeFile(t, configPath, "tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: /opt/httpx\n  nuclei: /opt/nuclei\n")

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", sampleFingerprint()); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}

	err = run([]string{
		"report",
		"--config", configPath,
		"--db", dbPath,
		"--run-id", "run-1",
		"--json", jsonPath,
		"--html", htmlPath,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	for _, path := range []string{jsonPath, htmlPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}
}

type fakeRunner struct {
	outputs [][]byte
	index   int
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	if f.index >= len(f.outputs) {
		return nil, errors.New("unexpected command")
	}
	out := f.outputs[f.index]
	f.index++
	return out, nil
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}
