package app

import (
	"context"
	"path/filepath"
	"testing"
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
