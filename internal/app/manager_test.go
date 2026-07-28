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

func TestManagerRejectsRunHeldByAnotherManager(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	firstStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer firstStore.Close()
	secondStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer secondStore.Close()
	first := NewManager(sleepRunner{}, firstStore)
	second := NewManager(sleepRunner{}, secondStore)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstOpts := ScanOptions{RunID: "run-1", ProfileName: "normal", Targets: []string{"127.0.0.1"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "first.json")}
	if _, err := first.Start(ctx, firstOpts); err != nil {
		t.Fatal(err)
	}
	secondOpts := firstOpts
	secondOpts.RunID = "run-2"
	if _, err := second.Start(context.Background(), secondOpts); err == nil || err.Error() != "scan already running: run-1" {
		t.Fatalf("second manager error = %v", err)
	}
	cancel()
	waitForInactive(t, first)
}

func TestManagerStartRecordsScanRunBeforeReturning(t *testing.T) {
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer scanStore.Close()
	manager := NewManager(sleepRunner{}, scanStore)
	opts := ScanOptions{
		RunID:          "run-early",
		ProfileName:    "normal",
		Targets:        []string{"127.0.0.1"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		ArtifactRoot:   t.TempDir(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := manager.Start(ctx, opts); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	// The web UI redirects to the run detail page as soon as Start returns, so
	// the row must exist synchronously — otherwise the page races RunScan's
	// asynchronous save and 404s.
	run, err := scanStore.GetScanRun("run-early")
	if err != nil {
		t.Fatalf("run must be visible immediately after Start: %v", err)
	}
	if run.Status != "running" {
		t.Fatalf("status = %q, want running", run.Status)
	}
	if run.Target != "127.0.0.1" || run.Ports != "22" {
		t.Fatalf("unexpected run target/ports: %q %q", run.Target, run.Ports)
	}
	if run.ArtifactDir == "" {
		t.Fatal("artifact dir should be recorded")
	}
	cancel()
	waitForInactive(t, manager)
}

func TestManagerStartToolRecordsRunBeforeReturning(t *testing.T) {
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer scanStore.Close()
	manager := NewManager(sleepRunner{}, scanStore)
	opts := ToolRunOptions{
		RunID: "tool-early", Tool: "rustscan", Target: "127.0.0.1", Ports: "22",
		Tools: ToolPaths{Rustscan: "/opt/rustscan"}, JSONReportPath: filepath.Join(t.TempDir(), "tool.json"),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := manager.StartTool(ctx, opts); err != nil {
		t.Fatalf("StartTool returned error: %v", err)
	}
	run, err := scanStore.GetScanRun("tool-early")
	if err != nil {
		t.Fatalf("tool run must be visible immediately after StartTool: %v", err)
	}
	if run.Kind != "tool" || run.Status != "running" {
		t.Fatalf("unexpected tool run row: kind=%q status=%q", run.Kind, run.Status)
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
