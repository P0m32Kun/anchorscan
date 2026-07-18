package app

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestRunLeaseRejectsSecondEntryWithoutCreatingRun(t *testing.T) {
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

	started := make(chan struct{})
	var once sync.Once
	runner := runnerFunc(func(ctx context.Context, _ string, _ []string) ([]byte, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunTool(ctx, runner, firstStore, ToolRunOptions{RunID: "tool-1", Tool: "rustscan", Target: "192.0.2.1", Ports: "80", Tools: ToolPaths{Rustscan: "fixture"}, JSONReportPath: filepath.Join(t.TempDir(), "first.json")})
	}()
	<-started
	err = RunTool(context.Background(), runner, secondStore, ToolRunOptions{RunID: "tool-2", Tool: "rustscan", Target: "192.0.2.2", Ports: "80", Tools: ToolPaths{Rustscan: "fixture"}, JSONReportPath: filepath.Join(t.TempDir(), "second.json")})
	if err == nil || err.Error() != "scan already running: tool-1" {
		t.Fatalf("second entry error = %v", err)
	}
	if _, err := secondStore.GetScanRun("tool-2"); err == nil {
		t.Fatal("rejected entry created a run")
	}
	cancel()
	<-done
}
