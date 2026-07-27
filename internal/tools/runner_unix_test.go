//go:build !windows

package tools

import (
	"context"
	"testing"
	"time"
)

func TestExecRunnerKillsProcessTreeOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	runner := NewExecRunner()
	_, err := runner.Run(ctx, "/bin/sh", []string{"-c", "sleep 30 & wait"})
	if err == nil {
		t.Fatal("expected error after cancellation")
	}
}
