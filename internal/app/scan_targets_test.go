package app

import (
	"context"
	"path/filepath"
	"testing"
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
