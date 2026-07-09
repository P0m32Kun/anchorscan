package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

func TestManagerAllowsOnlyOneActiveScan(t *testing.T) {
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer scanStore.Close()
	manager := NewManager(sleepRunner{}, scanStore)
	opts := ScanOptions{RunID: "run-1", ProfileName: "normal", Targets: []string{"127.0.0.1"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := manager.Start(ctx, opts); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if _, err := manager.Start(context.Background(), opts); err == nil {
		t.Fatal("expected active scan error")
	}
	cancel()
	waitForInactive(t, manager)
}

func TestManagerAllowsOnlyOneActiveToolRun(t *testing.T) {
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	manager := NewManager(sleepRunner{}, scanStore)
	opts := ToolRunOptions{
		RunID: "tool-1", Tool: "rustscan", Target: "127.0.0.1", Ports: "22",
		Tools: ToolPaths{Rustscan: "/opt/rustscan"}, JSONReportPath: filepath.Join(t.TempDir(), "tool.json"),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := manager.StartTool(ctx, opts); err != nil {
		t.Fatalf("StartTool returned error: %v", err)
	}
	if _, err := manager.StartTool(context.Background(), opts); err == nil {
		t.Fatal("expected active tool run error")
	}
	cancel()
	waitForInactive(t, manager)
}

type sleepRunner struct{}

func (sleepRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		return []byte("127.0.0.1 -> []\n"), nil
	}
}

var _ tools.Runner = sleepRunner{}

func waitForInactive(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.After(500 * time.Millisecond)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if manager.ActiveRunID() == "" {
			return
		}
		select {
		case <-deadline:
			t.Fatal("manager stayed active after cancellation")
		case <-ticker.C:
		}
	}
}
