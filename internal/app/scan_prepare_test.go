package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/config"
)

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
	if prepared.Options.ConfigSnapshot != "" {
		t.Fatalf("ConfigSnapshot = %q, want zero value", prepared.Options.ConfigSnapshot)
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
	if err := os.Remove(fixture.rustscan); err != nil {
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

type prepareScanFixture struct{ dir, configPath, dbPath, jsonPath, artifactPath, rustscan, httpx, configYAML string }

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
	fixture := prepareScanFixture{dir: dir, rustscan: tool("rustscan"), httpx: tool("httpx")}
	nmap, nuclei := tool("nmap"), tool("nuclei")
	fixture.configPath = filepath.Join(dir, "anchorscan.yaml")
	fixture.dbPath, fixture.jsonPath, fixture.artifactPath = filepath.Join(dir, "scan.db"), filepath.Join(dir, "out", "report.json"), filepath.Join(dir, "artifacts")
	fixture.configYAML = "tools:\n  rustscan: " + fixture.rustscan + "\n  nmap: " + nmap + "\n  httpx: " + fixture.httpx + "\n  nuclei: " + nuclei + "\nscan:\n  ports: 80,443,8080\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 3\n    nmap_args: [-T3]\n  slow:\n    host_workers: 1\n    nmap_args: [-T2]\n"
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
