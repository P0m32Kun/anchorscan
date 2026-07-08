package tools

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	args   []string
	output []byte
	err    error
}

func (f *fakeRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	f.args = append([]string{binary}, args...)
	if f.err != nil {
		return f.output, f.err
	}
	return f.output, nil
}

func TestDiscoverPortsBuildsRustscanCommand(t *testing.T) {
	runner := &fakeRunner{output: []byte("192.168.1.10 -> [80,443]\n")}
	got, err := DiscoverPorts(context.Background(), runner, "/opt/rustscan", "192.168.1.10", "80,443", []string{"--batch-size", "500"})
	if err != nil {
		t.Fatalf("DiscoverPorts returned error: %v", err)
	}

	wantPorts := []int{80, 443}
	if !reflect.DeepEqual(got, wantPorts) {
		t.Fatalf("ports mismatch: got %#v want %#v", got, wantPorts)
	}

	wantArgs := []string{"/opt/rustscan", "-a", "192.168.1.10", "--ports", "80,443", "-g", "--no-banner", "--batch-size", "500"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args mismatch: got %#v want %#v", runner.args, wantArgs)
	}
}

func TestDiscoverPortsUsesRangeForPortRanges(t *testing.T) {
	runner := &fakeRunner{output: []byte("127.0.0.1 -> [6379,8080]\n")}
	got, err := DiscoverPorts(context.Background(), runner, "/opt/rustscan", "127.0.0.1", "1-65535", nil)
	if err != nil {
		t.Fatalf("DiscoverPorts returned error: %v", err)
	}

	wantPorts := []int{6379, 8080}
	if !reflect.DeepEqual(got, wantPorts) {
		t.Fatalf("ports mismatch: got %#v want %#v", got, wantPorts)
	}

	wantArgs := []string{"/opt/rustscan", "-a", "127.0.0.1", "--range", "1-65535", "-g", "--no-banner"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args mismatch: got %#v want %#v", runner.args, wantArgs)
	}
}

func TestDiscoverPortsReturnsRunnerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	_, err := DiscoverPorts(context.Background(), runner, "/opt/rustscan", "192.168.1.10", "80", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscoverPortsIncludesToolOutputOnFailure(t *testing.T) {
	runner := &fakeRunner{output: []byte("permission denied\n"), err: errors.New("exit status 1")}
	_, err := DiscoverPorts(context.Background(), runner, "/opt/rustscan", "192.168.1.10", "80", nil)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected tool output in error, got %v", err)
	}
}
