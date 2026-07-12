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
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/version"
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
	for _, want := range []string{
		"config: ok",
		"rustscan: ok",
		"nmap: ok",
		"ports: ok",
		"nse rules: ok",
		"tag rules: ok",
		"database: ok",
		"reports: ok",
	} {
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
		"top1000",
		"--profile",
		"--host-workers",
		"--rustscan-args",
		"--nmap-args",
		"--httpx-args",
		"--nuclei-args",
		"--json",
		"--html",
		"--artifacts",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

func TestExecuteScanReturnsPortErrorBeforeProfileError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: rustscan\n  nmap: nmap\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "192.0.2.1",
		"--ports", "invalid",
		"--profile", "unknown",
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("error = %v, want invalid port error", err)
	}
}

func TestExecuteScanDoesNotOpenStoreWhenSharedPreflightFails(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "missing-rustscan")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

	storeOpened := false
	runnerCalled := false
	var stderr bytes.Buffer
	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "192.0.2.1",
		"--db", filepath.Join(dir, "scan.db"),
		"--json", filepath.Join(dir, "report.json"),
	}, &bytes.Buffer{}, &stderr, cliDeps{
		openStore: func(string) (*store.Store, error) {
			storeOpened = true
			return nil, errors.New("store should not be opened")
		},
		newRunner: func() tools.Runner {
			runnerCalled = true
			return failRunner{}
		},
	})
	if err == nil || err.Error() != "preflight failed" {
		t.Fatalf("error = %v, want preflight failed", err)
	}
	if !strings.Contains(stderr.String(), "[scan] preflight error rustscan:") {
		t.Fatalf("stderr = %q, want preflight diagnostic", stderr.String())
	}
	if storeOpened || runnerCalled {
		t.Fatalf("storeOpened=%t runnerCalled=%t, want both false", storeOpened, runnerCalled)
	}
}

func TestExecuteScanStoresArtifactDirUnderSelectedRoot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	artifactRoot := filepath.Join(dir, "custom-artifacts")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	writeFile(t, configPath, "tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

	runner := &recordingRunner{outputs: [][]byte{
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
		[]byte("127.0.0.1 -> [80]\n"),
		[]byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`),
	}}
	now := time.Unix(10, 0)

	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "127.0.0.1",
		"--db", dbPath,
		"--json", jsonPath,
		"--artifacts", artifactRoot,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{
		newRunner: func() tools.Runner { return runner },
		now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runID := now.Format("20060102-150405")
	runRow, err := scanStore.GetScanRun(runID)
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	wantDir := filepath.Join(artifactRoot, runID)
	if runRow.ArtifactDir != wantDir {
		t.Fatalf("artifact dir mismatch: got %q want %q", runRow.ArtifactDir, wantDir)
	}
	if _, err := os.Stat(wantDir); err != nil {
		t.Fatalf("expected artifact dir at %s: %v", wantDir, err)
	}
}

func TestExecuteScanPrintsPreflightSummary(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")

	var stdout, stderr bytes.Buffer
	runner := &fakeRunner{
		outputs: [][]byte{
			[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
			[]byte("Open 80\nOpen 443\n"),
			[]byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`),
			[]byte(`{"url":"http://127.0.0.1","status-code":200,"title":"nginx","tech":["nginx"]}`),
		},
	}
	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "127.0.0.1",
		"--db", filepath.Join(dir, "scan.db"),
		"--json", filepath.Join(dir, "report.json"),
	}, &stdout, &stderr, cliDeps{newRunner: func() tools.Runner { return runner }})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stderr.String(), "[scan] preflight targets=1 ports=top1000 profile=normal workers=1") {
		t.Fatalf("expected preflight summary, got %q", stderr.String())
	}
}

func TestExecuteScanStopsOnPreflightError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "missing-rustscan")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "127.0.0.1",
		"--db", filepath.Join(dir, "scan.db"),
		"--json", filepath.Join(dir, "report.json"),
	}, &stdout, &stderr, cliDeps{newRunner: func() tools.Runner { return failRunner{} }})
	if err == nil {
		t.Fatal("expected preflight error")
	}
	if !strings.Contains(stderr.String(), "[scan] preflight error rustscan:") {
		t.Fatalf("expected preflight error output, got %q", stderr.String())
	}
}

func TestExecuteScanPassesProfileAndToolArgs(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	writeFile(t, configPath, `tools:
  rustscan: `+toolPath+`
  nmap: `+toolPath+`
  httpx: `+toolPath+`
  nuclei: `+toolPath+`
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
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
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

	if !runner.hasArgs(toolPath, "--batch-size", "100") {
		t.Fatalf("expected slow profile rustscan args in %#v", runner.commands)
	}
	if !runner.hasArgs(toolPath, "-rate-limit", "20") {
		t.Fatalf("expected slow profile httpx args in %#v", runner.commands)
	}
	if !runner.hasArgs(toolPath, "-T2", "--max-retries", "5") {
		t.Fatalf("expected nmap override args in %#v", runner.commands)
	}
}

func TestExecuteScanWritesJSONAndHTML(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	htmlPath := filepath.Join(dir, "report.html")

	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\nscan:\n  ports: 8080\n")

	runner := &fakeRunner{
		outputs: [][]byte{
			[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
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

func sampleFingerprint() fingerprint.ServiceFingerprint {
	return fingerprint.ServiceFingerprint{
		IP:         "192.168.1.10",
		Port:       8080,
		Service:    "http",
		Product:    "Apache Tomcat",
		Normalized: "http",
		IsWeb:      true,
		URL:        "http://192.168.1.10:8080",
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

type failRunner struct{}

func (failRunner) Run(context.Context, string, []string) ([]byte, error) {
	return nil, errors.New("runner should not be called")
}

func TestExecuteImportNmapWritesRunAndReports(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "sample.xml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "import.json")
	htmlPath := filepath.Join(dir, "import.html")
	writeFile(t, xmlPath, `<nmaprun>
  <host>
    <address addr="10.0.0.53"/>
    <ports>
      <port protocol="tcp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
        <cpe>cpe:/a:isc:bind:9.18</cpe>
        <script id="dns-version" output="9.18.0"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
    </ports>
  </host>
</nmaprun>`)

	var stdout bytes.Buffer
	err := run([]string{
		"import-nmap",
		"--xml", xmlPath,
		"--db", dbPath,
		"--json", jsonPath,
		"--html", htmlPath,
	}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "run_id=") {
		t.Fatalf("expected run_id in stdout: %q", stdout.String())
	}
	for _, path := range []string{jsonPath, htmlPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 1 {
		t.Fatalf("expected one run, got %d err=%v", len(runs), err)
	}
	fps, err := scanStore.ListFingerprints(runs[0].RunID)
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 2 {
		t.Fatalf("expected two fingerprints (tcp+udp), got %d", len(fps))
	}
	protos := map[string]bool{}
	for _, fp := range fps {
		protos[fp.Protocol] = true
	}
	if !protos["tcp"] || !protos["udp"] {
		t.Fatalf("expected both tcp and udp, got %v", protos)
	}
}

func TestExecuteImportNmapRejectsEmptyXML(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "empty.xml")
	dbPath := filepath.Join(dir, "scan.db")
	writeFile(t, xmlPath, "")

	err := run([]string{
		"import-nmap",
		"--xml", xmlPath,
		"--db", dbPath,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "empty XML file") {
		t.Fatalf("expected empty XML error, got: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}

func TestExecuteImportNmapRejectsNonNmaprun(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "foo.xml")
	dbPath := filepath.Join(dir, "scan.db")
	writeFile(t, xmlPath, `<foo><bar/></foo>`)

	err := run([]string{
		"import-nmap",
		"--xml", xmlPath,
		"--db", dbPath,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}

func TestExecuteImportNmapHelpShowsFlags(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"import-nmap", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Usage: anchorscan import-nmap",
		"--xml",
		"--db",
		"--run-id",
		"--project",
		"--json",
		"--html",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
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

func TestVersionCommandPrintsVersion(t *testing.T) {
	cases := [][]string{{"version"}, {"--version"}, {"-v"}}
	for _, args := range cases {
		var stdout bytes.Buffer
		if err := run(args, &stdout, &bytes.Buffer{}, cliDeps{}); err != nil {
			t.Fatalf("run(%v) returned error: %v", args, err)
		}
		if !strings.Contains(stdout.String(), "anchorscan version "+version.Version) {
			t.Fatalf("run(%v) output missing version: %q", args, stdout.String())
		}
	}
}
