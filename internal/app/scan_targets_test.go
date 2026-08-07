package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestRunScanAssumeUpSkipsAliveSweep(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		fathomJSONL("192.0.2.10", 22, "ssh", "OpenSSH", ""),
	}}
	if err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
		RunID: "run-assume-up", Targets: []string{"192.0.2.10"}, Ports: "22", DiscoveryMode: DiscoveryAssumeUp,
		Tools: ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	for _, command := range runner.commands {
		if command[0] == "/opt/nmap" && strings.Contains(strings.Join(command, " "), "-sn") {
			t.Fatalf("assume-up must skip alive sweep: %#v", runner.commands)
		}
	}
	if len(runner.commands) == 0 || runner.commands[0][0] != "/opt/fathom" {
		t.Fatalf("first command = %#v, want fathom", runner.commands)
	}
}

func TestRunScanClampsHostWorkers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		hostWorkers int
		wantActive  int
	}{
		{name: "defaults to one", hostWorkers: 0, wantActive: 1},
		{name: "caps at target count", hostWorkers: 99, wantActive: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := newPostAliveConcurrencyRunner(tc.wantActive)
			err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
				RunID:          "run-worker-boundary",
				HostWorkers:    tc.hostWorkers,
				Targets:        []string{"10.0.0.0/30"},
				Ports:          "22",
				Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
				JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
			})
			if err != nil {
				t.Fatalf("RunScan returned error: %v", err)
			}
			if runner.maxActive != tc.wantActive {
				t.Fatalf("max active = %d, want %d", runner.maxActive, tc.wantActive)
			}
		})
	}
}

func TestRunScanSkipsPortScanWhenHostHasNoFathomOutput(t *testing.T) {
	runner := &downHostRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-down", Targets: []string{"172.22.0.7"}, Ports: "1-65535", Tools: ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	// M4.4: there is no outer nmap -sn anymore — fathom probes the host and
	// emits nothing for a down host (or a host with all probed ports closed).
	if runner.fathomCalls != 1 {
		t.Fatalf("expected fathom to probe the host, got %d calls", runner.fathomCalls)
	}
}

// TestRunScanAutoModeScansAllScopeAddressesWithFathom pins the M4.4 auto-mode
// flow for IPv4: no nmap -sn sweep; every scope address goes straight to
// `fathom scan` (alive probing is internal to fathom).
func TestRunScanAutoModeScansAllScopeAddressesWithFathom(t *testing.T) {
	runner := &aliveSweepRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-cidr", Targets: []string{"172.22.0.0/30"}, Ports: "1-1000", Tools: ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	want := [][]string{
		{"/opt/fathom", "scan", "--json", "172.22.0.0", "-p", "1-1000"},
		{"/opt/fathom", "scan", "--json", "172.22.0.1", "-p", "1-1000"},
		{"/opt/fathom", "scan", "--json", "172.22.0.2", "-p", "1-1000"},
		{"/opt/fathom", "scan", "--json", "172.22.0.3", "-p", "1-1000"},
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("commands = %#v want %#v", runner.commands, want)
	}
}

func TestRunScanFastProfileDoesNotReduceFathomTargets(t *testing.T) {
	runner := &profileSensitiveAliveRunner{}
	err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
		RunID:          "run-fast-alive",
		ProfileName:    "fast",
		HostWorkers:    2,
		Targets:        []string{"172.22.0.0/30"},
		Ports:          "22",
		Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
		ExtraArgs:      ToolExtraArgs{Nmap: []string{"-T4", "--max-retries", "1"}},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	// M4.4: every IPv4 scope address enters scanTarget (fathom probes alive
	// internally), so the fast profile cannot shrink the target set either.
	if runner.fathomCalls != 4 {
		t.Fatalf("fast profile scanned %d fathom targets, want 4", runner.fathomCalls)
	}
}

type profileSensitiveAliveRunner struct {
	fathomCalls int
}

func (r *profileSensitiveAliveRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if binary == "/opt/fathom" {
		r.fathomCalls++
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected command: %s %s (no nmap alive sweep in IPv4 auto mode)", binary, joined)
}

func TestRunScanMarksCanceledWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &cancelAfterFirstTargetRunner{cancel: cancel}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		HostWorkers:    1,
		Targets:        []string{"192.168.1.10", "192.168.1.11"},
		Ports:          "22",
		Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}
	err = RunScan(ctx, runner, scanStore, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	run, getErr := scanStore.GetScanRun("run-1")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "canceled" {
		t.Fatalf("status mismatch: %#v", run)
	}
	if runner.calls != 1 {
		t.Fatalf("expected only one target start before cancellation, got %d calls", runner.calls)
	}
}

func TestRunScanMarksCanceledWhenToolIsKilledAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &killedAfterCancelRunner{cancel: cancel}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "slow",
		HostWorkers:    1,
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}
	err = RunScan(ctx, runner, scanStore, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	run, getErr := scanStore.GetScanRun("run-1")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "canceled" {
		t.Fatalf("status mismatch: %#v", run)
	}
}

func TestRunScanRespectsProfileHostWorkersAfterTargetExpansion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		workers int
	}{
		{name: "slow", workers: 1},
		{name: "normal", workers: 3},
		{name: "fast", workers: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The 10.0.0.0/28 scope expands to 16 addresses; every one enters
			// scanTarget in auto mode (M4.4: fathom owns IPv4 alive probing).
			const expandedTargets = 16
			runner := newPostAliveConcurrencyRunner(tc.workers)
			dbPath := filepath.Join(t.TempDir(), "scan.db")
			reportPath := filepath.Join(t.TempDir(), "report.json")
			scanStore, err := store.Open(dbPath)
			if err != nil {
				t.Fatalf("Open returned error: %v", err)
			}

			opts := ScanOptions{
				RunID:          "run-" + tc.name,
				ProfileName:    tc.name,
				HostWorkers:    tc.workers,
				Targets:        []string{"10.0.0.0/28"},
				Ports:          "22",
				Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
				JSONReportPath: reportPath,
			}

			if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
				t.Fatalf("RunScan returned error: %v", err)
			}
			if runner.maxActive != tc.workers {
				t.Fatalf("expected max active %d, got %d", tc.workers, runner.maxActive)
			}
			if runner.fathomCalls != expandedTargets {
				t.Fatalf("expected %d fathom calls, got %d", expandedTargets, runner.fathomCalls)
			}
		})
	}
}

func TestRunScanContinuesAfterTargetFailure(t *testing.T) {
	runner := &failFirstRunner{outputs: [][]byte{
		fathomJSONL("192.168.1.11", 22, "ssh", "OpenSSH", ""),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		HostWorkers:    1,
		Targets:        []string{"192.168.1.10", "192.168.1.11"},
		Ports:          "22",
		Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	fps, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 1 || fps[0].IP != "192.168.1.11" {
		t.Fatalf("unexpected fingerprints: %#v", fps)
	}

	events, err := scanStore.ListScanEvents("run-1", 20)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if !containsEvent(events, "error", "target", "192.168.1.10") {
		t.Fatalf("expected target error event, got %#v", events)
	}
}

func TestRunScanReturnsErrorWhenAllTargetsFail(t *testing.T) {
	runner := failRunner{err: fmt.Errorf("boom")}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		HostWorkers:    2,
		Targets:        []string{"192.168.1.10", "192.168.1.11"},
		Ports:          "22",
		Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}

	err = RunScan(context.Background(), runner, scanStore, opts)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "all targets failed") {
		t.Fatalf("expected all-targets-failed error, got %v", err)
	}
}

// ---- M4.5: fathom is IPv4-only; IPv6 targets are rejected outright ----

// TestRunScanRejectsIPv6Targets pins the M4.5 acceptance: the pipeline no
// longer falls back to an nmap -sn sweep for IPv6 (fathom is IPv4-only), so an
// IPv6 target fails with an explicit error.
func TestRunScanRejectsIPv6Targets(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{name: "single IPv6 address", targets: []string{"2001:db8::1"}},
		{name: "IPv6 CIDR", targets: []string{"2001:db8::/126"}},
		{name: "mixed IPv4 and IPv6 scope", targets: []string{"192.0.2.0/30", "2001:db8::/126"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunScan(context.Background(), &recordingSequenceRunner{}, newScanStore(t), ScanOptions{
				RunID: "run-ipv6", Targets: tt.targets, Ports: "22",
				Tools: ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
			})
			if err == nil || !strings.Contains(err.Error(), "fathom does not support IPv6 targets") {
				t.Fatalf("expected fathom IPv6 rejection, got %v", err)
			}
		})
	}
}

// TestRunScanAllowsIPv4CIDRWithoutNmap pins the reverse: an IPv4 CIDR scope no
// longer needs nmap for discovery (fathom expands and probes it internally),
// so a missing nmap binary must not block the scan.
func TestRunScanAllowsIPv4CIDRWithoutNmap(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		fathomJSONL("192.0.2.1", 22, "ssh", "OpenSSH", ""),
	}}
	err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
		RunID: "run-cidr-no-nmap", Targets: []string{"192.0.2.0/30"}, Ports: "22",
		Tools: ToolPaths{Fathom: "/opt/fathom"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if len(runner.commands) == 0 || runner.commands[0][0] != "/opt/fathom" {
		t.Fatalf("expected fathom-only run, got %#v", runner.commands)
	}
}

// TestRunScanAutoModeDerivesAliveIPsFromFathomOutput pins the M4.4 aliveIPs
// semantics: with fathom doing the IPv4 alive probing, the alive set is
// derived from scan results — hosts fathom emitted fingerprints for. A scope
// address that fathom does not output (down, or alive with all probed ports
// closed) is not alive.
func TestRunScanAutoModeDerivesAliveIPsFromFathomOutput(t *testing.T) {
	dir := t.TempDir()
	var commands [][]string
	runner := runnerFunc(func(_ context.Context, binary string, args []string) ([]byte, error) {
		if binary != "/opt/fathom" {
			return nil, fmt.Errorf("unexpected command %s %v", binary, args)
		}
		commands = append(commands, append([]string{binary}, args...))
		// Only 192.0.2.1 answers: the other three scope addresses produce no
		// fathom output (down, or alive with all probed ports closed).
		if slices.Contains(args, "192.0.2.1") {
			return fathomJSONL("192.0.2.1", 22, "ssh", "OpenSSH", ""), nil
		}
		return nil, nil
	})
	err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
		RunID:          "run-alive-derive",
		Targets:        []string{"192.0.2.0/30"},
		Ports:          "22",
		Tools:          ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
		JSONReportPath: filepath.Join(dir, "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if !slices.Contains(commands[0], "192.0.2.0") {
		t.Fatalf("expected all scope addresses to reach fathom, got %#v", commands)
	}
	if len(commands) != 4 {
		t.Fatalf("expected 4 fathom calls (one per scope address), got %#v", commands)
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var r report.ScanReport
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if !reflect.DeepEqual(r.AliveIPs, []string{"192.0.2.1"}) {
		t.Fatalf("alive_ips = %#v, want [192.0.2.1] (only the host fathom emitted)", r.AliveIPs)
	}
}
