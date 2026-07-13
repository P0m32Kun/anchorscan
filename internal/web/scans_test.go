package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestNewScanPageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, filepath.Join(dir, "ports-highrisk.txt"), "21,22,3306\n")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultTargets: "127.0.0.1", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/scans/new", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "发起扫描") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `<input type="hidden" name="project_id" value="p1">`) {
		t.Fatalf("expected bound project_id hidden input, got body=%s", body)
	}
	if !strings.Contains(body, "Local Lab") {
		t.Fatalf("expected project name in body, got body=%s", body)
	}
	if !strings.Contains(body, "<textarea name=\"target\"") {
		t.Fatalf("expected target textarea, got body=%s", body)
	}
	if !strings.Contains(body, "支持IP、CIDR(网段)或自定义范围，多目标用英文逗号或换行分隔") {
		t.Fatalf("expected updated target help text, got body=%s", body)
	}
	for _, want := range []string{"本次扫描覆盖设置", "临时覆盖档位", "临时覆盖目标", "临时覆盖端口"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected scan override copy %q, got body=%s", want, body)
		}
	}
	if !strings.Contains(body, `name="artifact_root"`) {
		t.Fatalf("expected artifact root input, got body=%s", body)
	}
	if !strings.Contains(body, `data-insert-ports="21,22,3306"`) {
		t.Fatalf("expected highrisk button to insert CSV, got body=%s", body)
	}
}

func TestScanCreateRendersPreflightErrors(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "missing-rustscan")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,443")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{
		ConfigPath: configPath,
		DBPath:     dbPath,
		Runner:     &serverSequenceRunner{},
		Now:        func() time.Time { return time.Unix(10, 0) },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := strings.NewReader("project_id=p1&target=127.0.0.1&ports=top1000&profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/scan", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "预检失败") || !strings.Contains(rec.Body.String(), "rustscan") {
		t.Fatalf("expected preflight errors in body, got %q", rec.Body.String())
	}
}

func TestScanCreatePassesTop1000ToRustscanTop(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	writeFile(t, configPath, "tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	runner := &serverSequenceRunner{outputs: [][]byte{
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
		[]byte("127.0.0.1 -> [80]\n"),
		[]byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`),
	}}
	handler, err := NewServer(ServerOptions{
		ConfigPath: configPath,
		DBPath:     dbPath,
		Runner:     runner,
		Now:        func() time.Time { return time.Unix(10, 0) },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := strings.NewReader("project_id=p1&target=127.0.0.1&ports=top1000&profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/scan", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d body=%s", rec.Code, rec.Body.String())
	}
	for range 50 {
		if runner.hasArgs(rustscanPath, "--top") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !runner.hasArgs(rustscanPath, "--top") {
		t.Fatalf("expected top1000 passed to rustscan --top, got %#v", runner.commands)
	}
}

func TestScanCreatePassesPortRangeToRustscanRange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	writeFile(t, configPath, "tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	runner := &serverSequenceRunner{outputs: [][]byte{
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
		[]byte("127.0.0.1 -> [80]\n"),
		[]byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`),
	}}
	handler, err := NewServer(ServerOptions{
		ConfigPath: configPath,
		DBPath:     dbPath,
		Runner:     runner,
		Now:        func() time.Time { return time.Unix(10, 0) },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := strings.NewReader("project_id=p1&target=127.0.0.1&ports=100-1000&profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/scan", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d body=%s", rec.Code, rec.Body.String())
	}
	for range 50 {
		if runner.hasArgs(rustscanPath, "--range", "100-1000") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !runner.hasArgs(rustscanPath, "--range", "100-1000") {
		t.Fatalf("expected port range passed to rustscan, got %#v", runner.commands)
	}
}

func TestScanCreateRejectsUnsupportedPortFormats(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "missing-rustscan")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{
		ConfigPath: configPath,
		DBPath:     dbPath,
		Runner:     &serverSequenceRunner{},
		Now:        func() time.Time { return time.Unix(10, 0) },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	for _, ports := range []string{"full", "highrisk", "80,100-200"} {
		body := strings.NewReader("project_id=p1&target=127.0.0.1&ports=" + ports + "&profile=normal")
		req := httptest.NewRequest(http.MethodPost, "/scan", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for ports=%q, got %d", ports, rec.Code)
		}
	}
}

func TestScanCreateDoesNotStartManagerWhenPreparationReturnsOrdinaryError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "scan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Runner: &serverSequenceRunner{}})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader("project_id=p1&target=127.0.0.1&ports=full"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no run after preparation error, got %#v", runs)
	}
}

func TestScanCreateKeepsConflictAndRedirectResponses(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	writeFile(t, configPath, "tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\nscan:\n  ports: 80\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	runner := &serverSequenceRunner{started: make(chan struct{}), block: make(chan struct{})}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Runner: runner, Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	form := "project_id=p1&target=127.0.0.1&ports=80"
	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form))
	firstReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(first, firstReq)
	if first.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", first.Code)
	}
	<-runner.started

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form))
	secondReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(second, secondReq)
	if second.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", second.Code)
	}
	close(runner.block)
}

func TestScanCreateUsesProjectDefaultsAndExclusions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "22,80,8080\n")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  slow:\n    host_workers: 1\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{
		ID:             "p1",
		Name:           "Local Lab",
		DefaultTargets: "127.0.0.1\n127.0.0.2",
		DefaultPorts:   "top1000",
		ExcludeTargets: "127.0.0.2",
		ExcludePorts:   "22",
		DefaultProfile: "slow",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	runner := &serverSequenceRunner{outputs: [][]byte{
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
		[]byte("127.0.0.1 -> [80,8080]\n"),
		[]byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
	}}
	handler, err := NewServer(ServerOptions{
		ConfigPath: configPath,
		DBPath:     dbPath,
		Listen:     "127.0.0.1:8088",
		Runner:     runner,
		Now:        func() time.Time { return time.Unix(10, 0) },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	form := strings.NewReader("project_id=p1")
	req := httptest.NewRequest(http.MethodPost, "/scan", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}

	var run store.ScanRun
	for range 50 {
		runs, err := scanStore.ListScanRuns(10)
		if err != nil {
			t.Fatalf("ListScanRuns returned error: %v", err)
		}
		if len(runs) == 1 && runs[0].Status == "completed" {
			run = runs[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run.RunID == "" {
		t.Fatalf("expected completed run, got none")
	}
	if run.Target != "127.0.0.1" || run.Ports != "80,8080" || run.Profile != "slow" {
		t.Fatalf("unexpected run: %#v", run)
	}
	reportPath := filepath.Join(dir, "projects", "p1", "runs", run.RunID, "report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected managed report at %s: %v", reportPath, err)
	}
	wantArtifactDir := filepath.Join(dir, "projects", "p1", "runs", run.RunID)
	if run.ArtifactDir != wantArtifactDir {
		t.Fatalf("unexpected artifact dir: got %q want %q", run.ArtifactDir, wantArtifactDir)
	}
	if _, err := os.Stat(wantArtifactDir); err != nil {
		t.Fatalf("expected managed artifact dir at %s: %v", wantArtifactDir, err)
	}
	if !runner.hasArgs(rustscanPath, "-a", "127.0.0.1", "--ports", "80,8080") {
		t.Fatalf("unexpected rustscan args: %#v", runner.commands)
	}
	if !runner.hasArgs(nmapPath, "-sn", "127.0.0.1") {
		t.Fatalf("unexpected alive check args: %#v", runner.commands)
	}
	if runner.callCount(rustscanPath) != 1 {
		t.Fatalf("expected single rustscan target after exclusions, got %#v", runner.commands)
	}
}

func TestScanCreateLoadsConfigBeforeValidatingProject(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "scan: [\n")

	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	_, wantErr := config.Load(configPath)
	if wantErr == nil {
		t.Fatal("config.Load returned nil error")
	}

	for _, tc := range []struct {
		name       string
		form       string
		unexpected string
	}{
		{name: "missing project ID", unexpected: "project_id is required"},
		{name: "unknown project ID", form: "project_id=does-not-exist", unexpected: sql.ErrNoRows.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(tc.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status mismatch: got %d body=%s", res.Code, res.Body.String())
			}
			if !strings.Contains(res.Body.String(), wantErr.Error()) {
				t.Fatalf("expected config error %q, got body=%s", wantErr, res.Body.String())
			}
			if strings.Contains(res.Body.String(), tc.unexpected) {
				t.Fatalf("expected config error before project validation, got body=%s", res.Body.String())
			}
		})
	}
}
