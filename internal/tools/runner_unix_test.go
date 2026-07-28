//go:build !windows

package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecRunnerKillsProcessTreeOnCancel(t *testing.T) {
	pidFile := t.TempDir() + "/child.pid"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, err := NewExecRunner().Run(ctx, "/bin/sh", []string{"-c", fmt.Sprintf("sleep 30 & echo $! > %q; wait", pidFile)})
		result <- err
	}()

	var childPID int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("child pid was not recorded")
	}

	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected error after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not return after cancellation")
	}

	if err := syscall.Kill(childPID, 0); err == nil {
		t.Fatalf("child process %d is still running", childPID)
	} else if err != syscall.ESRCH {
		t.Fatalf("check child process %d: %v", childPID, err)
	}
}

func TestExecRunnerKillsProcessTreeOnTimeout(t *testing.T) {
	pidFile := t.TempDir() + "/child.pid"
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, err := NewExecRunner().Run(ctx, "/bin/sh", []string{"-c", fmt.Sprintf("sleep 30 & echo $! > %q; wait", pidFile)})
		result <- err
	}()

	var childPID int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("child pid was not recorded")
	}

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected error after timeout")
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not return after timeout")
	}
	if err := syscall.Kill(childPID, 0); err == nil {
		t.Fatalf("child process %d is still running", childPID)
	} else if err != syscall.ESRCH {
		t.Fatalf("check child process %d: %v", childPID, err)
	}
}
