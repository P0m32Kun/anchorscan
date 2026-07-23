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

func saveProjectWithZones(t *testing.T, s *store.Store, project store.Project) {
	t.Helper()
	if err := s.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones(project.ID); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
}

func TestNewScanPageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, filepath.Join(dir, "ports-highrisk.txt"), "21,22,3306\n")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})
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
	if !strings.Contains(body, `data-scan-create-props=`) {
		t.Fatalf("expected Vue scan-create props, got body=%s", body)
	}
	if !strings.Contains(body, "<noscript>") || !strings.Contains(body, `name="project_id" value="p1"`) {
		t.Fatalf("expected no-script scan fallback, got body=%s", body)
	}
	if !strings.Contains(body, "Local Lab") {
		t.Fatalf("expected project name in body, got body=%s", body)
	}
	for _, want := range []string{"I区", "II区", "III区", "projectId", "highriskPorts"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected scan-create prop %q, got body=%s", want, body)
		}
	}
	if !strings.Contains(body, "21,22,3306") {
		t.Fatalf("expected highrisk ports in props, got body=%s", body)
	}
}

func TestNewScanPageProvidesSingleZoneDefaultToVue(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	project := store.Project{ID: "p1", Name: "Single Zone", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := scanStore.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateProjectZone(store.ProjectZone{ProjectID: project.ID, ZoneID: "dmz", Name: "DMZ", SortOrder: 1}); err != nil {
		t.Fatalf("CreateProjectZone returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/projects/p1/scans/new", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	if !strings.Contains(body, `data-scan-create`) {
		t.Fatalf("expected Vue scan-create mount point, got body=%s", body)
	}
	if !strings.Contains(body, `data-default-zone-id="dmz"`) {
		t.Fatalf("expected single zone default in page props, got body=%s", body)
	}
}

func TestScanCreateErrorsReturnsEmptyArray(t *testing.T) {
	if got := scanCreateErrors(nil); got == nil || len(got) != 0 {
		t.Fatalf("scanCreateErrors(nil) = %#v, want empty array", got)
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
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})

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

	body := strings.NewReader("project_id=p1&zone_id=I&target=127.0.0.1&ports=top1000&profile=normal&access_point=lab&tester_ip=127.0.0.2")
	req := httptest.NewRequest(http.MethodPost, "/scan", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `data-scan-create-props=`) || !strings.Contains(rec.Body.String(), "rustscan") {
		t.Fatalf("expected preflight errors in scan-create props, got %q", rec.Body.String())
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
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})

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

	body := strings.NewReader("project_id=p1&zone_id=I&target=127.0.0.1&ports=top1000&profile=normal&access_point=lab&tester_ip=127.0.0.2")
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
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})

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

	body := strings.NewReader("project_id=p1&zone_id=I&target=127.0.0.1&ports=100-1000&profile=normal&access_point=lab&tester_ip=127.0.0.2")
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
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})

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
		body := strings.NewReader("project_id=p1&zone_id=I&target=127.0.0.1&ports=" + ports + "&profile=normal&access_point=lab&tester_ip=127.0.0.2")
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
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Runner: &serverSequenceRunner{}})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader("project_id=p1&zone_id=I&target=127.0.0.1&ports=full&access_point=lab&tester_ip=127.0.0.2"))
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
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})
	runner := &serverSequenceRunner{started: make(chan struct{}), block: make(chan struct{})}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Runner: runner, Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	form := "project_id=p1&zone_id=I&target=127.0.0.1&ports=80&access_point=lab&tester_ip=127.0.0.2"
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

func TestScanCreateUsesExplicitParametersAndSavesRunFields(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	writeFile(t, filepath.Join(dir, "ports-top1000.txt"), "80,8080\n")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: top1000\n  profile: normal\nprofiles:\n  slow:\n    host_workers: 1\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})

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

	form := strings.NewReader("project_id=p1&zone_id=I&label=segment-a&target=127.0.0.1&exclude_targets=192.0.2.1&ports=80,8080&exclude_ports=22&profile=normal&access_point=core-sw-a&tester_ip=10.0.0.5&notes=lab")
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
	if run.ZoneID != "I" || run.Label != "segment-a" || run.Target != "127.0.0.1" || run.Ports != "80,8080" || run.Profile != "normal" {
		t.Fatalf("unexpected run parameters: %#v", run)
	}
	if run.AccessPoint != "core-sw-a" || run.TesterIP != "10.0.0.5" || run.Notes != "lab" {
		t.Fatalf("unexpected run context fields: %#v", run)
	}
	if !strings.Contains(run.ConfigSnapshot, `"exclude_targets":"192.0.2.1"`) || !strings.Contains(run.ConfigSnapshot, `"exclude_ports":"22"`) {
		t.Fatalf("expected exclusions in snapshot: %s", run.ConfigSnapshot)
	}
	if run.Kind != "scan" {
		t.Fatalf("expected kind scan, got %q", run.Kind)
	}
	if !run.IncludeInReport {
		t.Fatalf("expected completed scan to be included by default")
	}
	reportPath := filepath.Join(dir, "projects", "p1", "runs", run.RunID, "report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected managed report at %s: %v", reportPath, err)
	}
	wantArtifactDir := filepath.Join(dir, "projects", "p1", "runs", run.RunID)
	if run.ArtifactDir != wantArtifactDir {
		t.Fatalf("unexpected artifact dir: got %q want %q", run.ArtifactDir, wantArtifactDir)
	}
	if !runner.hasArgs(rustscanPath, "-a", "127.0.0.1", "--ports", "80,8080") {
		t.Fatalf("unexpected rustscan args: %#v", runner.commands)
	}
}

func TestScanCreateRejectsMissingZoneTargetOrPorts(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "scan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	saveProjectWithZones(t, scanStore, store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)})
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath, Runner: &serverSequenceRunner{}})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	cases := []string{
		"project_id=p1&target=127.0.0.1&ports=80&profile=normal",
		"project_id=p1&zone_id=I&ports=80&profile=normal",
		"project_id=p1&zone_id=I&target=127.0.0.1&profile=normal",
	}
	for _, form := range cases {
		req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for form %q, got %d", form, res.Code)
		}
	}
	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no runs, got %#v", runs)
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
