package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestExecuteDoctorPrintsChecks(t *testing.T) {
	dir := t.TempDir()
	toolPath := filepath.Join(dir, "tool")
	writeFile(t, toolPath, "")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

	var stdout bytes.Buffer
	err := run([]string{"doctor", "--config", configPath, "--db", filepath.Join(dir, "scan.db"), "--reports", dir}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	for _, want := range []string{"config: ok", "rustscan: ok", "nmap: ok", "database: ok", "reports: ok"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in %q", want, stdout.String())
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
	for _, want := range []string{"Usage:", "scan", "tool", "report", "tools check", "doctor", "web", "cancel"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

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

func TestExecuteWebHelpShowsListen(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"web", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "--listen") {
		t.Fatalf("expected --listen in %q", stdout.String())
	}
}

func TestExecuteCancelPostsToServer(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/runs/run-1/cancel" {
			called = true
			w.WriteHeader(http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := run([]string{"cancel", "--run-id", "run-1", "--server", server.URL}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !called {
		t.Fatal("expected cancel request")
	}
}

func TestExecuteScanHelpShowsFlags(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"scan", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Usage: anchorscan scan",
		"--target",
		"IP range",
		"--ports",
		"100-1000",
		"--profile",
		"--host-workers",
		"--rustscan-args",
		"--nmap-args",
		"--httpx-args",
		"--nuclei-args",
		"--json",
		"--html",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

func TestExecuteScanPassesProfileAndToolArgs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	writeFile(t, configPath, `tools:
  rustscan: /opt/rustscan
  nmap: /opt/nmap
  httpx: /opt/httpx
  nuclei: /opt/nuclei
scan:
  ports: 8080
  profile: normal
profiles:
  normal:
    host_workers: 3
    rustscan_args: ["--batch-size", "500"]
    nmap_args: ["-T3"]
    httpx_args: ["-rate-limit", "100"]
    nuclei_args: ["-rate-limit", "50"]
  slow:
    host_workers: 1
    rustscan_args: ["--batch-size", "100"]
    nmap_args: ["-T2"]
    httpx_args: ["-rate-limit", "20"]
    nuclei_args: ["-rate-limit", "10"]
`)

	runner := &recordingRunner{outputs: [][]byte{
		[]byte("Open 8080\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
	}}

	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "192.168.1.10",
		"--db", dbPath,
		"--json", jsonPath,
		"--profile", "slow",
		"--host-workers", "2",
		"--nmap-args", "-T2 --max-retries 5",
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{newRunner: func() tools.Runner { return runner }})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !runner.hasArgs("/opt/rustscan", "--batch-size", "100") {
		t.Fatalf("expected slow profile rustscan args in %#v", runner.commands)
	}
	if !runner.hasArgs("/opt/httpx", "-rate-limit", "20") {
		t.Fatalf("expected slow profile httpx args in %#v", runner.commands)
	}
	if !runner.hasArgs("/opt/nmap", "-T2", "--max-retries", "5") {
		t.Fatalf("expected nmap override args in %#v", runner.commands)
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

type recordingRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
}

func (r *recordingRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	cmd := append([]string{binary}, args...)
	r.commands = append(r.commands, cmd)
	if r.index >= len(r.outputs) {
		return nil, errors.New("unexpected command")
	}
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

func (r *recordingRunner) hasArg(binary string, arg string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, item := range cmd[1:] {
			if item == arg {
				return true
			}
		}
	}
	return false
}

func (r *recordingRunner) hasArgs(binary string, args ...string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for i := 1; i+len(args) <= len(cmd); i++ {
			match := true
			for j := range args {
				if cmd[i+j] != args[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}
