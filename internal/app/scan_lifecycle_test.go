package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestRunScanRecordsReportWriteFailure(t *testing.T) {
	scanStore := newScanStore(t)
	reportPath := filepath.Join(t.TempDir(), "missing", "report.json")
	err := RunScan(context.Background(), &downHostRunner{}, scanStore, ScanOptions{
		RunID:          "run-report-failure",
		Targets:        []string{"172.22.0.7"},
		Ports:          "1-65535",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	})
	if err == nil {
		t.Fatal("expected report write error")
	}
	run, getErr := scanStore.GetScanRun("run-report-failure")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "failed" || run.Error != err.Error() {
		t.Fatalf("unexpected failed run: %#v", run)
	}
}

func TestRunScanPersistsRunLifecycleAndEvents(t *testing.T) {
	runner := &sequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProjectID:      "p1",
		ProfileName:    "normal",
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
		ConfigSnapshot: "profile: normal",
	}
	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	run, err := scanStore.GetScanRun("run-1")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.Status != "completed" || run.Profile != "normal" {
		t.Fatalf("unexpected run: %#v", run)
	}
	events, err := scanStore.ListScanEvents("run-1", 20)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) == 0 || events[0].Message == "" {
		t.Fatalf("expected scan events, got %#v", events)
	}
}
