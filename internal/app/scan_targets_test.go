package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestRunScanClampsHostWorkers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		hostWorkers int
		wantActive  int
	}{
		{name: "defaults to one", hostWorkers: 0, wantActive: 1},
		{name: "caps at live targets", hostWorkers: 99, wantActive: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := []string{"10.0.0.1", "10.0.0.2"}
			runner := newPostAliveConcurrencyRunner(targets, tc.wantActive)
			err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
				RunID:          "run-worker-boundary",
				HostWorkers:    tc.hostWorkers,
				Targets:        []string{"10.0.0.0/30"},
				Ports:          "22",
				Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
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

func TestRunScanSkipsPortScanWhenHostIsDown(t *testing.T) {
	runner := &downHostRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-down", Targets: []string{"172.22.0.7"}, Ports: "1-65535", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if runner.rustscanCalls != 0 {
		t.Fatalf("expected rustscan to be skipped for down host, got %d calls", runner.rustscanCalls)
	}
}

func TestRunScanUsesAliveSweepResultsAsTargets(t *testing.T) {
	runner := &aliveSweepRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-cidr", Targets: []string{"172.22.0.0/30"}, Ports: "1-1000", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	want := [][]string{
		{"/opt/nmap", "-sn", "172.22.0.0/30", "-oX", "-"},
		{"/opt/rustscan", "-a", "172.22.0.1", "--range", "1-1000", "-g", "--no-banner"},
		{"/opt/rustscan", "-a", "172.22.0.2", "--range", "1-1000", "-g", "--no-banner"},
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("commands = %#v want %#v", runner.commands, want)
	}
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
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
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
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
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

func TestRunScanRespectsProfileHostWorkersAfterAliveSweep(t *testing.T) {
	for _, tc := range []struct {
		name    string
		workers int
	}{
		{name: "slow", workers: 1},
		{name: "normal", workers: 3},
		{name: "fast", workers: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := []string{
				"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4",
				"10.0.0.5", "10.0.0.6", "10.0.0.7", "10.0.0.8",
				"10.0.0.9", "10.0.0.10", "10.0.0.11", "10.0.0.12",
			}
			runner := newPostAliveConcurrencyRunner(targets, tc.workers)
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
				Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
				JSONReportPath: reportPath,
			}

			if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
				t.Fatalf("RunScan returned error: %v", err)
			}
			if runner.aliveCalls != 1 {
				t.Fatalf("expected one alive sweep, got %d", runner.aliveCalls)
			}
			if runner.maxActive != tc.workers {
				t.Fatalf("expected max active %d, got %d", tc.workers, runner.maxActive)
			}
			if runner.rustscanCalls != len(targets) {
				t.Fatalf("expected %d rustscan calls, got %d", len(targets), runner.rustscanCalls)
			}
		})
	}
}

func TestRunScanContinuesAfterTargetFailure(t *testing.T) {
	runner := &failFirstRunner{outputs: [][]byte{
		[]byte("192.168.1.11 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.11" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
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
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
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
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
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
