package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

// cmdFathomJSONL renders one `fathom scan --json` line for CLI-layer tests
// (mirror of the app-layer fathomJSONL helper).
func cmdFathomJSONL(ip string, port int, service, product string) []byte {
	line := fmt.Sprintf(`{"host":"%s","port":%d,"service":"%s","product":"%s","version":""}`, ip, port, service, product)
	return []byte(line + "\n")
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
		"IPv6",
		"CIDR",
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

func TestLogPreflightReportsEffectiveToolTimeouts(t *testing.T) {
	var stderr bytes.Buffer
	logPreflight(&stderr, preflight.Result{Summary: preflight.Summary{Timeouts: config.ToolTimeouts{
		Rustscan: "0", Nmap: "30s", Httpx: "150ms", NSE: "5m", Nuclei: "1m", Rdpscan: "0",
	}}})
	if got := stderr.String(); !strings.Contains(got, "rustscan=0 nmap=30s httpx=150ms nse=5m nuclei=1m rdpscan=0 (0=unlimited)") {
		t.Fatalf("preflight log missing timeouts: %q", got)
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

func TestExecuteScanRejectsInvalidDiscoveryMode(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: rustscan\n  nmap: nmap\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	err := run([]string{"scan", "--config", configPath, "--target", "192.0.2.1", "--discovery", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "invalid discovery mode") {
		t.Fatalf("error = %v, want discovery mode error", err)
	}
}

func TestExecuteScanDoesNotOpenStoreWhenSharedPreflightFails(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  fathom: "+filepath.Join(dir, "missing-fathom")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

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
	if !strings.Contains(stderr.String(), "[scan] preflight error fathom:") {
		t.Fatalf("stderr = %q, want preflight diagnostic", stderr.String())
	}
	if storeOpened || runnerCalled {
		t.Fatalf("storeOpened=%t runnerCalled=%t, want both false", storeOpened, runnerCalled)
	}
}

func TestExecuteScanRejectsInvalidScopeBeforeOpeningStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "scan:\n  ports: 80\n  profile: normal\n")
	storeOpened := false

	err := run([]string{"scan", "--config", configPath, "--target=-Pn", "--db", filepath.Join(dir, "scan.db"), "--json", filepath.Join(dir, "report.json")}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{
		newRunner: func() tools.Runner { return failRunner{} },
		openStore: func(string) (*store.Store, error) {
			storeOpened = true
			return nil, errors.New("store must not open")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid target") {
		t.Fatalf("run error = %v, want invalid target", err)
	}
	if storeOpened {
		t.Fatal("scope rejection opened the store")
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
	fathomPath := writeExecutable(t, dir, "fathom")
	writeFile(t, configPath, "tools:\n  fathom: "+fathomPath+"\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeScanRuleFiles(t, dir)

	runner := &recordingRunner{outputs: [][]byte{
		cmdFathomJSONL("127.0.0.1", 80, "http", "nginx"),
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
	writeFile(t, configPath, "tools:\n  fathom: "+toolPath+"\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeScanRuleFiles(t, dir)
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")

	var stdout, stderr bytes.Buffer
	runner := &fakeRunner{
		outputs: [][]byte{
			cmdFathomJSONL("127.0.0.1", 80, "http", "nginx"),
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
	writeFile(t, configPath, "tools:\n  fathom: "+filepath.Join(dir, "missing-fathom")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
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
	if !strings.Contains(stderr.String(), "[scan] preflight error fathom:") {
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
  fathom: `+toolPath+`
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
	writeScanRuleFiles(t, dir)

	runner := &recordingRunner{outputs: [][]byte{
		cmdFathomJSONL("192.168.1.10", 8080, "http", "Apache Tomcat"),
		[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
		[]byte{},
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

	if !runner.hasArgs(toolPath, "scan", "--json", "192.168.1.10", "-p", "8080") {
		t.Fatalf("expected fathom fixed argv, commands=%#v", runner.commands)
	}
	if !runner.hasArgs(toolPath, "-rate-limit", "20") {
		t.Fatalf("expected slow profile httpx args in %#v", runner.commands)
	}
	if !runner.hasArgs(toolPath, "-rate-limit", "10") {
		t.Fatalf("expected slow profile nuclei args in %#v", runner.commands)
	}
}

func TestExecuteScanWritesJSONAndHTML(t *testing.T) {
	dir := t.TempDir()
	toolPath := writeExecutable(t, dir, "tool")
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	htmlPath := filepath.Join(dir, "report.html")

	writeFile(t, configPath, "tools:\n  fathom: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\nscan:\n  ports: 8080\n")
	writeScanRuleFiles(t, dir)

	runner := &fakeRunner{
		outputs: [][]byte{
			cmdFathomJSONL("192.168.1.10", 8080, "http", "Apache Tomcat"),
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
		"[scan] fathom",
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

func writeScanRuleFiles(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "nse.yaml"), "http: [http-title]\n")
	writeFile(t, filepath.Join(dir, "service-tags.yaml"), "- name: http\n  service: [http]\n  nuclei_tags: [http]\n  target: url\n")
}
