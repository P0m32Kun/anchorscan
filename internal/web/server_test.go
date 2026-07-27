package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type serverSequenceRunner struct {
	mu       sync.Mutex
	outputs  [][]byte
	commands [][]string
	index    int
	started  chan struct{}
	block    chan struct{}
}

func (r *serverSequenceRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.mu.Lock()
	r.commands = append(r.commands, append([]string{binary}, args...))
	r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	commands := make([][]string, len(r.commands))
	copy(commands, r.commands)
	for _, cmd := range commands {
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
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, cmd := range r.commands {
		if len(cmd) > 0 && cmd[0] == binary {
			count++
		}
	}
	return count
}

func (r *serverSequenceRunner) Commands() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, len(r.commands))
	for i, cmd := range r.commands {
		out[i] = append([]string(nil), cmd...)
	}
	return out
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

func TestServerRejectsCrossOriginStateChangeAndNonLoopbackListen(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"), Listen: "0.0.0.0:8088"}); err == nil {
		t.Fatal("non-loopback listen address was accepted")
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db"), Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)
	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.Header.Set("Origin", "https://attacker.example")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}

	crossSite := httptest.NewRequest(http.MethodPost, "/projects", nil)
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, crossSite)
	if res.Code != http.StatusForbidden {
		t.Fatalf("cross-site status = %d, want %d", res.Code, http.StatusForbidden)
	}
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

func TestStaticAssetsServeLeafScripts(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	for _, asset := range []struct {
		path string
		want string
	}{
		{path: "/static/app.js", want: "copyReportData"},
		{path: "/static/report-ui.js", want: "renderVulnDistribution"},
	} {
		t.Run(asset.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, asset.path, nil))
			if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), asset.want) {
				t.Fatalf("unexpected response for %s: %d %s", asset.path, res.Code, res.Body.String())
			}
		})
	}
}
