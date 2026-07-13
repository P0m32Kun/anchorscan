package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/version"
)

func TestExecuteRootHelpShowsCommands(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{"Usage:", "scan", "tool", "report", "tools check", "doctor", "web", "cancel"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}

func TestRunUnknownCommandPreservesStderrAndError(t *testing.T) {
	var stderr bytes.Buffer
	err := run([]string{"missing"}, &bytes.Buffer{}, &stderr, cliDeps{})
	if err == nil || err.Error() != "unknown command" {
		t.Fatalf("error = %v", err)
	}
	if stderr.String() != "unknown command: missing\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func sampleFingerprint() fingerprint.ServiceFingerprint {
	return fingerprint.ServiceFingerprint{
		IP:         "192.168.1.10",
		Port:       8080,
		Service:    "http",
		Product:    "Apache Tomcat",
		Normalized: "http",
		IsWeb:      true,
		URL:        "http://192.168.1.10:8080",
	}
}

type fakeRunner struct {
	outputs [][]byte
	index   int
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	if f.index >= len(f.outputs) {
		return nil, errors.New("unexpected command")
	}
	out := f.outputs[f.index]
	f.index++
	return out, nil
}

type recordingRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
}

func (r *recordingRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	cmd := append([]string{binary}, args...)
	r.commands = append(r.commands, cmd)
	if r.index >= len(r.outputs) {
		return nil, errors.New("unexpected command")
	}
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

func (r *recordingRunner) hasArg(binary string, arg string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, item := range cmd[1:] {
			if item == arg {
				return true
			}
		}
	}
	return false
}

func (r *recordingRunner) hasArgs(binary string, args ...string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for i := 1; i+len(args) <= len(cmd); i++ {
			match := true
			for j := range args {
				if cmd[i+j] != args[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

type failRunner struct{}

func (failRunner) Run(context.Context, string, []string) ([]byte, error) {
	return nil, errors.New("runner should not be called")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}

func writeExecutable(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
	return path
}

func TestVersionCommandPrintsVersion(t *testing.T) {
	cases := [][]string{{"version"}, {"--version"}, {"-v"}}
	for _, args := range cases {
		var stdout bytes.Buffer
		if err := run(args, &stdout, &bytes.Buffer{}, cliDeps{}); err != nil {
			t.Fatalf("run(%v) returned error: %v", args, err)
		}
		if !strings.Contains(stdout.String(), "anchorscan version "+version.Version) {
			t.Fatalf("run(%v) output missing version: %q", args, stdout.String())
		}
	}
}
