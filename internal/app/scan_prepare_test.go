package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/config"
)

func testPort(t *testing.T, value string) int {
	t.Helper()
	port, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid test port %q: %v", value, err)
	}
	return port
}

func writePackagedConfig(t *testing.T, fixture prepareScanFixture) string {
	t.Helper()
	configDir := filepath.Join(fixture.dir, "package", "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "default.yaml")
	if err := os.WriteFile(configPath, []byte(fixture.configYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nse.yaml", "service-tags.yaml"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(configDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return configPath
}

func TestPrepareScanBuildsOptionsFromDefaultsAndOverrides(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	prepared, err := PrepareScan(fixture.request())
	if err != nil {
		t.Fatalf("PrepareScan returned error: %v", err)
	}
	if prepared.Preflight.HasErrors() {
		t.Fatalf("PrepareScan returned preflight errors: %#v", prepared.Preflight.Errors)
	}
	if got, want := prepared.Options.Ports, "80,443,8080"; got != want {
		t.Fatalf("Ports = %q, want %q", got, want)
	}
	if got, want := prepared.Options.ProfileName, "normal"; got != want {
		t.Fatalf("ProfileName = %q, want %q", got, want)
	}
	if got, want := prepared.Options.HostWorkers, 3; got != want {
		t.Fatalf("HostWorkers = %d, want %d", got, want)
	}
	if got, want := prepared.Options.ExtraArgs.Nmap, []string{"-T3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Nmap args = %#v, want %#v", got, want)
	}
	if got, want := prepared.Preflight.Summary.NSERuleCount, len(prepared.Options.NSERules); got != want {
		t.Fatalf("NSE rule count = %d, want %d", got, want)
	}
	if got, want := prepared.Preflight.Summary.TagRuleCount, len(prepared.Options.TagRules); got != want {
		t.Fatalf("tag rule count = %d, want %d", got, want)
	}
	if !strings.Contains(prepared.Options.ConfigSnapshot, `"discovery_mode":"auto"`) || !strings.Contains(prepared.Options.ConfigSnapshot, `"includes":["192.0.2.1/32"]`) {
		t.Fatalf("ConfigSnapshot = %q, want scope and discovery mode", prepared.Options.ConfigSnapshot)
	}

	prepared, err = PrepareScan(PrepareScanRequest{
		ConfigPath: fixture.configPath, TargetSpec: "192.0.2.1", PortSpec: "22",
		DBPath: fixture.dbPath, JSONReportPath: fixture.jsonPath, ArtifactRoot: fixture.artifactPath,
		Overrides: config.Overrides{ProfileName: "slow", HostWorkers: 5, NmapArgs: "-sV --version-light"},
	})
	if err != nil {
		t.Fatalf("PrepareScan with overrides returned error: %v", err)
	}
	if got, want := prepared.Options.ProfileName, "slow"; got != want {
		t.Fatalf("ProfileName = %q, want %q", got, want)
	}
	if got, want := prepared.Options.HostWorkers, 5; got != want {
		t.Fatalf("HostWorkers = %d, want %d", got, want)
	}
	if got, want := prepared.Options.ExtraArgs.Nmap, []string{"-sV", "--version-light"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Nmap args = %#v, want %#v", got, want)
	}
}

func TestPrepareScanLoadsPackagedRulesForSSHExecution(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	req := fixture.request()
	req.ConfigPath = writePackagedConfig(t, fixture)
	req.PortSpec = "22"
	prepared, err := PrepareScan(req)
	if err != nil || prepared.Preflight.HasErrors() {
		t.Fatalf("PrepareScan = %#v, %v", prepared, err)
	}
	runner := &recordingSequenceRunner{outputs: [][]byte{
		fathomJSONL("192.0.2.1", 22, "ssh", "OpenSSH", ""),
		[]byte(`<nmaprun/>`),
		[]byte{},
	}}
	if err := RunScan(context.Background(), runner, newScanStore(t), prepared.Options); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if !runner.hasArgs(prepared.Options.Tools.Nmap, "--script", "ssh2-enum-algos,ssh-hostkey") {
		t.Fatalf("expected configured SSH NSE invocation, commands=%#v", runner.commands)
	}
	if !runner.hasArgs(prepared.Options.Tools.Nuclei, "-tags", "ssh", "-target", "192.0.2.1:22") {
		t.Fatalf("expected SSH nuclei -tags invocation, commands=%#v", runner.commands)
	}
	// SSH must run via -tags (never an in-repo -t template) and exclude default-login.
	var nucleiCmd []string
	for _, cmd := range runner.commands {
		if len(cmd) > 0 && cmd[0] == prepared.Options.Tools.Nuclei {
			nucleiCmd = cmd
			break
		}
	}
	if nucleiCmd != nil && !strings.Contains(strings.Join(nucleiCmd, " "), "default-login") {
		t.Fatalf("expected nuclei -etags to exclude default-login, command=%#v", nucleiCmd)
	}
}

func TestPrepareScanLoadsPackagedRulesForTomcatAndX11Execution(t *testing.T) {
	for _, test := range []struct {
		name    string
		port    string
		service string
		product string
		outputs [][]byte
		tags    string
		target  string
	}{
		{
			name: "tomcat nuclei URL", port: "8080", service: "http", product: "Apache Tomcat",
			outputs: [][]byte{[]byte(`{"url":"http://192.0.2.1:8080","tech":["tomcat"]}`), []byte{}},
			tags:    "tomcat,apache-tomcat", target: "http://192.0.2.1:8080",
		},
		{
			name: "x11 nuclei hostport", port: "6000", service: "x11",
			outputs: [][]byte{[]byte{}},
			tags:    "x11", target: "192.0.2.1:6000",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPrepareScanFixture(t)
			req := fixture.request()
			req.ConfigPath = writePackagedConfig(t, fixture)
			req.PortSpec = test.port
			prepared, err := PrepareScan(req)
			if err != nil || prepared.Preflight.HasErrors() {
				t.Fatalf("PrepareScan = %#v, %v", prepared, err)
			}
			outputs := [][]byte{fathomJSONL("192.0.2.1", testPort(t, test.port), test.service, test.product, "")}
			runner := &recordingSequenceRunner{outputs: append(outputs, test.outputs...)}
			scanStore := newScanStore(t)
			if err := RunScan(context.Background(), runner, scanStore, prepared.Options); err != nil {
				t.Fatalf("RunScan returned error: %v", err)
			}
			if !runner.hasArgs(prepared.Options.Tools.Nuclei, "-tags", test.tags, "-target", test.target) {
				t.Fatalf("expected configured nuclei invocation, commands=%#v", runner.commands)
			}
			if runner.hasArgs(prepared.Options.Tools.Nmap, "--script") {
				t.Fatalf("expected no NSE invocation, commands=%#v", runner.commands)
			}
			checks, err := scanStore.ListDetectionChecks(prepared.Options.RunID)
			if err != nil {
				t.Fatal(err)
			}
			foundNuclei := false
			for _, check := range checks {
				if check.Engine == "nuclei" {
					foundNuclei = true
					if check.Status != "completed" {
						t.Fatalf("nuclei check = %#v, want completed", check)
					}
				}
			}
			if !foundNuclei {
				t.Fatalf("expected nuclei detection check, got %#v", checks)
			}
		})
	}
}

func TestPrepareScanValidatesDiscoveryMode(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	for _, test := range []struct {
		name string
		mode string
		want string
		err  string
	}{
		{name: "default", want: DiscoveryAuto},
		{name: "assume up", mode: DiscoveryAssumeUp, want: DiscoveryAssumeUp},
		{name: "invalid", mode: "skip", err: "invalid discovery mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := fixture.request()
			req.DiscoveryMode = test.mode
			prepared, err := PrepareScan(req)
			if test.err != "" {
				if err == nil || !strings.Contains(err.Error(), test.err) {
					t.Fatalf("error = %v, want %q", err, test.err)
				}
				return
			}
			if err != nil || prepared.Options.DiscoveryMode != test.want {
				t.Fatalf("PrepareScan = %#v, %v; want discovery %q", prepared, err, test.want)
			}
		})
	}
}

func TestPrepareScanAppliesExclusionsButKeepsPreflightPortSpec(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	prepared, err := PrepareScan(PrepareScanRequest{
		ConfigPath: fixture.configPath, TargetSpec: "192.0.2.1,192.0.2.2,192.0.2.3", ExcludeTargets: "192.0.2.2",
		PortSpec: "80,443,8080", ExcludePorts: "443", DBPath: fixture.dbPath, JSONReportPath: fixture.jsonPath, ArtifactRoot: fixture.artifactPath,
	})
	if err != nil {
		t.Fatalf("PrepareScan returned error: %v", err)
	}
	if got, want := prepared.Options.Targets, []string{"192.0.2.1", "192.0.2.3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Targets = %#v, want %#v", got, want)
	}
	if got, want := prepared.Options.Ports, "80,8080"; got != want {
		t.Fatalf("Ports = %q, want %q", got, want)
	}
	if got, want := prepared.Preflight.Summary.PortSpec, "80,443,8080"; got != want {
		t.Fatalf("Preflight PortSpec = %q, want %q", got, want)
	}
}

func TestPrepareScanReturnsPreflightErrorsWithoutOptions(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	if err := os.Remove(fixture.configPath); err != nil {
		t.Fatal(err)
	}
	// fathom is the sole scan engine (spec v2.0): without it in config the
	// preflight must error and PrepareScan must not produce any options.
	if err := os.WriteFile(fixture.configPath, []byte(strings.Replace(fixture.configYAML, "  fathom: ", "  fathom: \"\"\n  # ", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareScan(fixture.request())
	if err != nil {
		t.Fatalf("PrepareScan returned ordinary error: %v", err)
	}
	if !prepared.Preflight.HasErrors() {
		t.Fatal("Preflight.HasErrors() = false, want true")
	}
	if !reflect.DeepEqual(prepared.Options, ScanOptions{}) {
		t.Fatalf("Options = %#v, want zero value", prepared.Options)
	}
}

func TestPrepareScanReturnsWarningsWithOptions(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	if err := os.WriteFile(fixture.configPath, []byte(strings.Replace(fixture.configYAML, "  httpx: "+fixture.httpx+"\n", "  httpx: \"\"\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareScan(fixture.request())
	if err != nil {
		t.Fatalf("PrepareScan returned error: %v", err)
	}
	if prepared.Preflight.HasErrors() || len(prepared.Preflight.Warnings) == 0 {
		t.Fatalf("Preflight = %#v, want warnings without errors", prepared.Preflight)
	}
	if prepared.Options.Tools.Httpx != "" || prepared.Options.Ports == "" {
		t.Fatalf("Options = %#v, want complete options with empty optional tool", prepared.Options)
	}
}

func TestPrepareScanUsesFixedOrdinaryErrorOrder(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	prepared, err := PrepareScan(PrepareScanRequest{
		ConfigPath: fixture.configPath, TargetSpec: "192.0.2.1", PortSpec: "invalid", DBPath: fixture.dbPath, JSONReportPath: fixture.jsonPath,
		Overrides: config.Overrides{ProfileName: "unknown"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("error = %v, want invalid port error", err)
	}
	if !reflect.DeepEqual(prepared, PreparedScan{}) {
		t.Fatalf("PreparedScan = %#v, want zero value", prepared)
	}
}

func TestPrepareScanKeepsRuleFileFallback(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	if err := os.Remove(filepath.Join(fixture.dir, "nse.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.dir, "service-tags.yaml")); err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "config", "nse.yaml"), []byte("fallback:\n  - fallback-script\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "config", "service-tags.yaml"), []byte("- name: fallback\n  match: fallback\n  tags: [fallback]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	prepared, err := PrepareScan(fixture.request())
	if err != nil {
		t.Fatalf("PrepareScan returned error: %v", err)
	}
	if got, want := prepared.Options.NSERules, map[string][]string{"fallback": {"fallback-script"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NSERules = %#v, want %#v", got, want)
	}
	if got, want := len(prepared.Options.TagRules), 1; got != want {
		t.Fatalf("TagRules = %#v, want one fallback rule", got)
	}
}

func TestPrepareScanEquivalentRequestsShareExecutionFields(t *testing.T) {
	fixture := newPrepareScanFixture(t)
	first, err := PrepareScan(fixture.request())
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareScan(PrepareScanRequest{
		ConfigPath: fixture.configPath, TargetSpec: "192.0.2.1", DBPath: fixture.dbPath,
		RunID: "another-run", ProjectID: "another-project", JSONReportPath: filepath.Join(t.TempDir(), "other.json"), ArtifactRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := executionFields(second.Options), executionFields(first.Options); !reflect.DeepEqual(got, want) {
		t.Fatalf("execution fields = %#v, want %#v", got, want)
	}
}

func executionFields(opts ScanOptions) any {
	return struct {
		Targets []string
		Ports   string
		Tools   ToolPaths
		Profile string
		Workers int
		Args    ToolExtraArgs
		NSE     map[string][]string
		Tags    []TagRule
	}{opts.Targets, opts.Ports, opts.Tools, opts.ProfileName, opts.HostWorkers, opts.ExtraArgs, opts.NSERules, opts.TagRules}
}

type prepareScanFixture struct{ dir, configPath, dbPath, jsonPath, artifactPath, httpx, configYAML string }

func newPrepareScanFixture(t *testing.T) prepareScanFixture {
	t.Helper()
	dir := t.TempDir()
	tool := func(name string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	fixture := prepareScanFixture{dir: dir, httpx: tool("httpx")}
	nmap, nuclei := tool("nmap"), tool("nuclei")
	fathom := tool("fathom")
	fixture.configPath = filepath.Join(dir, "anchorscan.yaml")
	fixture.dbPath, fixture.jsonPath, fixture.artifactPath = filepath.Join(dir, "scan.db"), filepath.Join(dir, "out", "report.json"), filepath.Join(dir, "artifacts")
	fixture.configYAML = "tools:\n  fathom: " + fathom + "\n  nmap: " + nmap + "\n  httpx: " + fixture.httpx + "\n  nuclei: " + nuclei + "\nscan:\n  ports: 80,443,8080\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 3\n    nmap_args: [-T3]\n  slow:\n    host_workers: 1\n    nmap_args: [-T2]\n"
	if err := os.WriteFile(fixture.configPath, []byte(fixture.configYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nse.yaml"), []byte("http:\n  - http-title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service-tags.yaml"), []byte("- name: web\n  match: http\n  tags: [web]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ports-top1000.txt"), []byte("80\n443\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f prepareScanFixture) request() PrepareScanRequest {
	return PrepareScanRequest{ConfigPath: f.configPath, TargetSpec: "192.0.2.1", RunID: "run-1", ProjectID: "project-1", DBPath: f.dbPath, JSONReportPath: f.jsonPath, ArtifactRoot: f.artifactPath}
}
