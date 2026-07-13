package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type serverSequenceRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
	started  chan struct{}
	block    chan struct{}
}

func (r *serverSequenceRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
	if r.started != nil {
		close(r.started)
		r.started = nil
	}
	if r.block != nil {
		<-r.block
	}
	if r.index >= len(r.outputs) {
		return []byte{}, nil
	}
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

func (r *serverSequenceRunner) hasArgs(binary string, want ...string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		all := true
		for _, arg := range want {
			found := false
			for _, got := range cmd[1:] {
				if got == arg {
					found = true
					break
				}
			}
			if !found {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func (r *serverSequenceRunner) callCount(binary string) int {
	count := 0
	for _, cmd := range r.commands {
		if len(cmd) > 0 && cmd[0] == binary {
			count++
		}
	}
	return count
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

func closeServer(t *testing.T, handler http.Handler) {
	t.Helper()
	closer, ok := handler.(interface{ Close() error })
	if !ok {
		return
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
}

func TestNavIncludesImportNmapEntry(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	body := res.Body.String()
	if !strings.Contains(body, `href="/import/nmap"`) || !strings.Contains(body, "导入 Nmap XML") {
		t.Fatalf("expected import nav entry, got: %s", body)
	}
}
