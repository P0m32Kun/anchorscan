package web

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestHomePageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", ProjectID: "p1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(2, 0), FinishedAt: time.Unix(3, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "missing.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "AnchorScan") || !strings.Contains(res.Body.String(), "Local Lab") {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "项目") || !strings.Contains(res.Body.String(), "最近扫描") {
		t.Fatalf("expected chinese ui copy: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `/projects/p1`) || !strings.Contains(res.Body.String(), `/runs/run-1`) {
		t.Fatalf("expected home links in body: %s", res.Body.String())
	}
	// footer version is rendered from the version package, not hardcoded
	if !strings.Contains(res.Body.String(), "AnchorScan Console v1.5.1") {
		t.Fatalf("expected versioned footer in body: %s", res.Body.String())
	}
}

func TestCreateProjectFromWeb(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{
		"name":            "Local Lab",
		"description":     "Test",
		"default_targets": "127.0.0.1\n127.0.0.2",
		"default_ports":   "top1000",
		"exclude_targets": "127.0.0.2",
		"exclude_ports":   "22,3306",
		"default_profile": "normal",
	} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField returned error: %v", err)
		}
	}
	fileWriter, err := writer.CreateFormFile("targets_file", "targets.txt")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte("192.168.1.0/24\n")); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d", res.Code)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Local Lab" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
	if !strings.Contains(projects[0].DefaultTargets, "192.168.1.0/24") || projects[0].ExcludeTargets != "127.0.0.2" || projects[0].ExcludePorts != "22,3306" {
		t.Fatalf("unexpected saved project fields: %#v", projects[0])
	}
}

func TestNewScanPageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultTargets: "127.0.0.1", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/scan/new", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "开始扫描") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
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
}

func TestToolNewPageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultTargets: "127.0.0.1", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/tools/new", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"单独运行工具", "rustscan", "nmap", "httpx", "nuclei", "Local Lab"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body: %s", want, body)
		}
	}
}

func TestToolDetailPageRendersNmapHelpAndPresets(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/tools/nmap", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{
		`name="tool" value="nmap"`,
		`name="raw_args"`,
		"Nmap 单工具调用",
		"可选绑定到项目",
		"中文 Help",
		"存活检测",
		`data-set-raw-args="-sn 192.168.1.10"`,
		"tool-output",
		"Local Lab",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body: %s", want, body)
		}
	}
	for _, unwanted := range []string{`name="mode"`, `name="target"`, `name="url"`, `name="ports"`, `name="tags"`, `name="template"`, `name="extra_args"`} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect split parameter field %q in body: %s", unwanted, body)
		}
	}
}

func TestToolCreateRunsNativeRawArgs(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: /opt/httpx\n  nuclei: /opt/nuclei\nscan:\n  ports: 80,443\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	runner := &serverSequenceRunner{outputs: [][]byte{
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
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

	form := strings.NewReader("tool=nmap&raw_args=-sn+192.0.2.10+--min-rate+50")
	req := httptest.NewRequest(http.MethodPost, "/tools", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "fetch")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if !strings.HasPrefix(body["run_id"], "tool-nmap-") {
		t.Fatalf("unexpected ajax body: %#v", body)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	for range 200 {
		runs, err := scanStore.ListScanRuns(10)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if runner.hasArgs("/opt/nmap", "-sn", "192.0.2.10", "--min-rate", "50") && len(runs) == 1 && runs[0].Status == "completed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("unexpected nmap args: %#v", runner.commands)
}

func TestToolCreateStartsRun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: /opt/httpx\n  nuclei: /opt/nuclei\nscan:\n  ports: 80,443\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	runner := &serverSequenceRunner{outputs: [][]byte{
		[]byte(`<nmaprun><host><status state="up"/></host></nmaprun>`),
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

	form := strings.NewReader("tool=nmap&mode=alive&target=192.0.2.10&extra_args=--min-rate+50")
	req := httptest.NewRequest(http.MethodPost, "/tools", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/runs/tool-nmap-") {
		t.Fatalf("unexpected redirect: %s", location)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	var run store.ScanRun
	for range 200 {
		runs, err := scanStore.ListScanRuns(10)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if len(runs) == 1 && runs[0].Status == "completed" {
			run = runs[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run.RunID == "" || run.Profile != "tool:nmap" || run.Target != "192.0.2.10" {
		t.Fatalf("unexpected run: %#v", run)
	}
	if !runner.hasArgs("/opt/nmap", "-sn", "192.0.2.10", "--min-rate", "50") {
		t.Fatalf("unexpected nmap args: %#v", runner.commands)
	}
	reportPath := filepath.Join(dir, "runs", run.RunID, "report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected managed report at %s: %v", reportPath, err)
	}
}

func TestScanCreateRendersPreflightErrors(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "missing-rustscan")+"\n  nmap: "+filepath.Join(dir, "missing-nmap")+"\nscan:\n  ports: top100\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "ports-top100.txt"), "80,443")

	handler, err := NewServer(ServerOptions{
		ConfigPath: configPath,
		DBPath:     filepath.Join(dir, "scan.db"),
		Runner:     &serverSequenceRunner{},
		Now:        func() time.Time { return time.Unix(10, 0) },
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := strings.NewReader("target=127.0.0.1&ports=top100&profile=normal")
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

func TestScanCreateUsesProjectDefaultsAndExclusions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	rustscanPath := writeExecutable(t, dir, "rustscan")
	nmapPath := writeExecutable(t, dir, "nmap")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: "+rustscanPath+"\n  nmap: "+nmapPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: 80,443\n  profile: normal\nprofiles:\n  slow:\n    host_workers: 1\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
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
		DefaultPorts:   "22,80,8080",
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
	wantArtifactDir := filepath.Join(dir, "artifacts", run.RunID)
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

func TestDeleteProjectRemovesManagedFilesAndDatabaseRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{
		ID:             "p1",
		Name:           "Local Lab",
		DefaultTargets: "127.0.0.1",
		DefaultPorts:   "6379",
		DefaultProfile: "normal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "6379",
		Profile:        "normal",
		Status:         "completed",
		ArtifactDir:    filepath.Join(dir, "artifacts", "run-1"),
		ConfigSnapshot: "profile: normal",
		StartedAt:      now,
		FinishedAt:     now.Add(time.Second),
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := scanStore.SaveFinding("run-1", report.Finding{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	if err := scanStore.AppendScanEvent(store.ScanEvent{RunID: "run-1", Time: now.Add(2 * time.Second), Level: "info", Stage: "nmap", Message: "done"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}
	reportPath := filepath.Join(dir, "projects", "p1", "runs", "run-1", "report.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	artifactPath := filepath.Join(dir, "artifacts", "run-1", "rustscan-127.0.0.1.txt")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("127.0.0.1 -> [6379]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("_method=delete")
	req := httptest.NewRequest(http.MethodPost, "/projects/p1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}

	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected no projects, got %#v", projects)
	}
	runs, err := scanStore.ListProjectScanRuns("p1", 10)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no runs, got %#v", runs)
	}
	fps, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 0 {
		t.Fatalf("expected no fingerprints, got %#v", fps)
	}
	findings, err := scanStore.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
	events, err := scanStore.ListScanEvents("run-1", 10)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %#v", events)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "p1")); !os.IsNotExist(err) {
		t.Fatalf("expected project dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "artifacts", "run-1")); !os.IsNotExist(err) {
		t.Fatalf("expected artifact dir to be removed, got err=%v", err)
	}
}

func TestDeleteProjectRejectsRunningRuns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveProject(store.Project{
		ID:             "p1",
		Name:           "Local Lab",
		DefaultTargets: "127.0.0.1",
		DefaultPorts:   "6379",
		DefaultProfile: "normal",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "6379",
		Profile:        "normal",
		Status:         "running",
		ConfigSnapshot: "profile: normal",
		StartedAt:      now,
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("_method=delete")
	req := httptest.NewRequest(http.MethodPost, "/projects/p1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected project to remain, got %#v", projects)
	}
}

func TestDeleteScanRunRemovesManagedFilesAndDatabaseRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	now := time.Unix(1, 0)
	if err := scanStore.SaveScanRun(store.ScanRun{
		RunID:          "run-1",
		ProjectID:      "p1",
		Target:         "127.0.0.1",
		Ports:          "6379",
		Profile:        "normal",
		Status:         "completed",
		ArtifactDir:    filepath.Join(dir, "artifacts", "run-1"),
		ConfigSnapshot: "profile: normal",
		StartedAt:      now,
		FinishedAt:     now.Add(time.Second),
	}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := scanStore.SaveFinding("run-1", report.Finding{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	if err := scanStore.AppendScanEvent(store.ScanEvent{RunID: "run-1", Time: now.Add(2 * time.Second), Level: "info", Stage: "nmap", Message: "done"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}
	reportPath := filepath.Join(dir, "projects", "p1", "runs", "run-1", "report.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	artifactPath := filepath.Join(dir, "artifacts", "run-1", "nmap-127.0.0.1.xml")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("<nmaprun/>"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("_method=delete")
	req := httptest.NewRequest(http.MethodPost, "/runs/run-1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}

	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no runs, got %#v", runs)
	}
	fps, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 0 {
		t.Fatalf("expected no fingerprints, got %#v", fps)
	}
	findings, err := scanStore.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
	events, err := scanStore.ListScanEvents("run-1", 10)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %#v", events)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "p1", "runs", "run-1")); !os.IsNotExist(err) {
		t.Fatalf("expected run dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "artifacts", "run-1")); !os.IsNotExist(err) {
		t.Fatalf("expected artifact dir to be removed, got err=%v", err)
	}
}

func TestRunEventsAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "running", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.AppendScanEvent(store.ScanEvent{RunID: "run-1", Time: time.Unix(2, 0), Level: "info", Stage: "nmap", Message: "still running"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/runs/run-1/events", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "still running") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
	var events []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &events); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(events) != 1 || events[0]["time"] == nil || events[0]["stage"] == nil || events[0]["message"] == nil {
		t.Fatalf("unexpected json fields: %#v", events)
	}
}

func TestRunStatusAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/runs/run-1/status", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if body["status"] != "completed" {
		t.Fatalf("unexpected status response: %#v", body)
	}
}

func TestRunPageLoadsStatusPolling(t *testing.T) {
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), DBPath: filepath.Join(t.TempDir(), "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/static/app.js", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "/status") || !strings.Contains(res.Body.String(), "refreshRunStatus") {
		t.Fatalf("expected run status polling script: %s", res.Body.String())
	}
}

func TestReportPageRendersFindings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "6379", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := scanStore.SaveFinding("run-1", report.Finding{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379", Output: "{\n  \"matched-at\": \"127.0.0.1:6379\"\n}"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "redis-default-logins") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "<details") || !strings.Contains(res.Body.String(), "matched-at") {
		t.Fatalf("expected finding details in body: %s", res.Body.String())
	}
	if strings.Contains(res.Body.String(), "探测规则:") || strings.Contains(res.Body.String(), "危险指数:") {
		t.Fatalf("expected details panel to avoid duplicated finding metadata: %s", res.Body.String())
	}
	if strings.Contains(res.Body.String(), "展开原始输出") || strings.Contains(res.Body.String(), `class="evidence-details"`) {
		t.Fatalf("expected finding evidence to render directly after opening details: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "筛选") || !strings.Contains(res.Body.String(), "证据与详情") {
		t.Fatalf("expected chinese report copy: %s", res.Body.String())
	}
	for _, want := range []string{
		`type="checkbox" name="severity" value="critical"`,
		`type="checkbox" name="severity" value="high"`,
		`href="/reports/run-1/export?format=json"`,
		`href="/reports/run-1/export?format=html"`,
		`href="/reports/run-1/export?format=csv"`,
	} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected %q in report page: %s", want, res.Body.String())
		}
	}
}

func TestReportPagePaginatesAssetsAndFindings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "1-100", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	for i := 1; i <= 55; i++ {
		fp := fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 8000 + i, Service: "http", Product: "svc", Normalized: "http"}
		if err := scanStore.SaveFingerprint("run-1", fp); err != nil {
			t.Fatalf("SaveFingerprint returned error: %v", err)
		}
		finding := report.Finding{IP: "127.0.0.1", Port: 8000 + i, Source: "nuclei", ID: "finding-" + strconv.Itoa(i), Severity: "info", Summary: "summary", Target: "http://127.0.0.1"}
		if err := scanStore.SaveFinding("run-1", finding); err != nil {
			t.Fatalf("SaveFinding returned error: %v", err)
		}
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "finding-1") || strings.Contains(body, "finding-55") {
		t.Fatalf("expected first page findings only: %s", body)
	}
	if !strings.Contains(body, "资产第 1 / 2 页") || !strings.Contains(body, "漏洞第 1 / 2 页") {
		t.Fatalf("expected pagination label: %s", body)
	}
	if !strings.Contains(body, "findings_page=2") || !strings.Contains(body, "assets_page=2") {
		t.Fatalf("expected next page links: %s", body)
	}

	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1?assets_page=2&findings_page=2", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body = res.Body.String()
	if !strings.Contains(body, "finding-55") || strings.Contains(body, "finding-1") {
		t.Fatalf("expected second page findings only: %s", body)
	}

	// Per-page size selector: switching to 10/rows re-paginates and keeps filters.
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1?findings_size=10&q=svc", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body = res.Body.String()
	if !strings.Contains(body, "漏洞第 1 / 6 页") {
		t.Fatalf("expected 6 pages at size 10: %s", body)
	}
	// size links must preserve the keyword filter and drop the page param so
	// switching size resets to the first page. Since url.Values.Encode sorts
	// keys, a page param would sort before size and break this exact prefix.
	// html/template escapes "&" in the URL attribute.
	if !strings.Contains(body, `value="?findings_size=10&amp;q=svc"`) {
		t.Fatalf("expected size 10 link to carry filter and drop page: %s", body)
	}
	// the active size option should be marked selected
	if !strings.Contains(body, `value="?findings_size=10&amp;q=svc" selected`) {
		t.Fatalf("expected size 10 selected: %s", body)
	}
}

func TestReportPageFiltersFindingsByService(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "6379,8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	for _, fp := range []fingerprint.ServiceFingerprint{
		{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"},
		{IP: "127.0.0.1", Port: 8080, Service: "http", Product: "Apache Tomcat", Normalized: "http"},
	} {
		if err := scanStore.SaveFingerprint("run-1", fp); err != nil {
			t.Fatalf("SaveFingerprint returned error: %v", err)
		}
	}
	for _, finding := range []report.Finding{
		{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379"},
		{IP: "127.0.0.1", Port: 8080, Source: "nuclei", ID: "tomcat-detect", Severity: "info", Summary: "Tomcat Detect", Target: "http://127.0.0.1:8080"},
	} {
		if err := scanStore.SaveFinding("run-1", finding); err != nil {
			t.Fatalf("SaveFinding returned error: %v", err)
		}
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1?service=redis", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "redis-default-logins") || strings.Contains(body, "tomcat-detect") {
		t.Fatalf("unexpected filtered report: %s", body)
	}
}

func TestReportPageFiltersFindingsByMultipleSeveritySelections(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "443,6379,8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	for _, finding := range []report.Finding{
		{IP: "127.0.0.1", Port: 443, Source: "nuclei", ID: "critical-one", Severity: "critical", Summary: "Critical One", Target: "https://127.0.0.1"},
		{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "high-one", Severity: "high", Summary: "High One", Target: "127.0.0.1:6379"},
		{IP: "127.0.0.1", Port: 8080, Source: "nuclei", ID: "info-one", Severity: "info", Summary: "Info One", Target: "http://127.0.0.1:8080"},
	} {
		if err := scanStore.SaveFinding("run-1", finding); err != nil {
			t.Fatalf("SaveFinding returned error: %v", err)
		}
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1?severity=critical&severity=high", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "critical-one") || !strings.Contains(body, "high-one") || strings.Contains(body, "info-one") {
		t.Fatalf("unexpected severity-filtered report: %s", body)
	}
}

func TestReportPageRendersHostViewAndAssetWorkbench(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "6379,6380,8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	for _, fp := range []fingerprint.ServiceFingerprint{
		{IP: "127.0.0.1", Port: 6379, Service: "unknown", Product: "Redis", Version: "7.2.0", Normalized: "redis"},
		{IP: "127.0.0.1", Port: 6380, Service: "redis", Product: "Redis", Version: "6.2.0", Normalized: "redis"},
		{IP: "127.0.0.2", Port: 8080, Service: "http", Product: "Apache Tomcat", Version: "10.1.0", URL: "http://127.0.0.2:8080", Normalized: "http"},
	} {
		if err := scanStore.SaveFingerprint("run-1", fp); err != nil {
			t.Fatalf("SaveFingerprint returned error: %v", err)
		}
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1?view=hosts&q=redis", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "按主机聚合") || !strings.Contains(body, "复制 IP:PORT") || !strings.Contains(body, "/reports/run-1/assets.csv?q=redis") {
		t.Fatalf("expected asset workbench controls: %s", body)
	}
	if !strings.Contains(body, `<script src="/static/app.js"></script>`) {
		t.Fatalf("expected report page copy script: %s", body)
	}
	if !strings.Contains(body, "127.0.0.1") || !strings.Contains(body, "6379,6380") {
		t.Fatalf("expected grouped host row: %s", body)
	}
	if strings.Contains(body, "127.0.0.2") {
		t.Fatalf("expected redis filter to exclude non-matching host: %s", body)
	}
}

func TestReportPageCollapsesLongRunMetadata(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	longPorts := strings.Join([]string{
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
		"11", "12", "13", "14", "15", "16", "17", "18", "19", "20",
		"21", "22", "23", "24", "25", "26", "27", "28", "29", "30",
	}, ",")
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1,127.0.0.2", Ports: longPorts, Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "展开全部扫描参数") || !strings.Contains(body, "run-meta-details") {
		t.Fatalf("expected collapsed run metadata: %s", body)
	}
	if strings.Contains(body, `端口: <span class="mono-value">`+longPorts+`</span>`) {
		t.Fatalf("expected long ports outside the report header: %s", body)
	}
}

func TestReportAssetExportSupportsFilteredTXTAndCSV(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "6379,8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	for _, fp := range []fingerprint.ServiceFingerprint{
		{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Version: "7.2.0", Normalized: "redis"},
		{IP: "127.0.0.2", Port: 8080, Service: "http", Product: "Apache Tomcat", Version: "10.1.0", URL: "http://127.0.0.2:8080", Normalized: "http"},
	} {
		if err := scanStore.SaveFingerprint("run-1", fp); err != nil {
			t.Fatalf("SaveFingerprint returned error: %v", err)
		}
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/assets.txt?q=redis&kind=ip_port", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("txt status mismatch: %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("unexpected txt content-type: %s", ct)
	}
	if cd := res.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="run-1-assets.txt"`) {
		t.Fatalf("unexpected txt content-disposition: %s", cd)
	}
	txtBody := strings.TrimSpace(res.Body.String())
	if txtBody != "127.0.0.1:6379" {
		t.Fatalf("unexpected txt export: %q", txtBody)
	}

	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/assets.csv?q=redis", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("csv status mismatch: %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("unexpected csv content-type: %s", ct)
	}
	if cd := res.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="run-1-assets.csv"`) {
		t.Fatalf("unexpected csv content-disposition: %s", cd)
	}
	csvBody := res.Body.String()
	if !strings.Contains(csvBody, "ip,port,protocol,service,product,version,cpe,url") || !strings.Contains(csvBody, "127.0.0.1,6379,,redis,Redis,7.2.0,,") {
		t.Fatalf("unexpected csv export: %s", csvBody)
	}
	if strings.Contains(csvBody, "127.0.0.2") {
		t.Fatalf("expected filtered csv export: %s", csvBody)
	}
}

func TestReportExportDownloadsRicherFormats(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "6379,8080", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Version: "7.2.0", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	for _, finding := range []report.Finding{
		{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379", Output: "{\"matched-at\":\"127.0.0.1:6379\"}"},
		{IP: "127.0.0.1", Port: 8080, Source: "nuclei", ID: "tomcat-detect", Severity: "info", Summary: "Tomcat Detect", Target: "http://127.0.0.1:8080", Output: "{\"matched-at\":\"http://127.0.0.1:8080\"}"},
	} {
		if err := scanStore.SaveFinding("run-1", finding); err != nil {
			t.Fatalf("SaveFinding returned error: %v", err)
		}
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/export?format=html&severity=high", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("html status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	if cd := res.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="anchorscan-run-1.html"`) {
		t.Fatalf("unexpected html content-disposition: %s", cd)
	}
	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("unexpected html content-type: %s", ct)
	}
	if !strings.Contains(res.Body.String(), "matched-at") || strings.Contains(res.Body.String(), "tomcat-detect") {
		t.Fatalf("unexpected html export: %s", res.Body.String())
	}

	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/export?format=json&severity=high", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("json status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	if cd := res.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="anchorscan-run-1.json"`) {
		t.Fatalf("unexpected json content-disposition: %s", cd)
	}
	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected json content-type: %s", ct)
	}
	if !strings.Contains(res.Body.String(), "redis-default-logins") || strings.Contains(res.Body.String(), "tomcat-detect") {
		t.Fatalf("unexpected json export: %s", res.Body.String())
	}

	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/export?format=csv&severity=high", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("csv status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	if cd := res.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="anchorscan-run-1.csv"`) {
		t.Fatalf("unexpected csv content-disposition: %s", cd)
	}
	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("unexpected csv content-type: %s", ct)
	}
	if !strings.Contains(res.Body.String(), "severity,source,id,ip,port,protocol,service,product,target,summary,evidence") || !strings.Contains(res.Body.String(), "redis-default-logins") || strings.Contains(res.Body.String(), "tomcat-detect") {
		t.Fatalf("unexpected csv export: %s", res.Body.String())
	}

	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/export?format=pdf", nil))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown export format, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestConfigPageUpdatesToolPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /old/rustscan\n  nmap: /old/nmap\nscan:\n  ports: top100\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 7, 21, 30, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("rustscan=/new/rustscan&nmap=/new/nmap&httpx=&nuclei=&ports=8080&profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Tools.Rustscan != "/new/rustscan" || cfg.Scan.Ports != "8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestConfigPageRendersAdvancedEditor(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /opt/rustscan\nscan:\n  ports: top100\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/config", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "name=\"raw_config\"") || !strings.Contains(body, "高级 YAML") {
		t.Fatalf("expected raw editor in body: %s", body)
	}
}

func TestConfigPageRawEditorUpdatesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /old/rustscan\nscan:\n  ports: top100\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("mode=raw&raw_config=tools%3A%0A++rustscan%3A+%2Fcustom%2Frustscan%0Ascan%3A%0A++ports%3A+8080%2C6379%0A++profile%3A+slow%0Aprofiles%3A%0A++slow%3A%0A++++host_workers%3A+1%0A")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Tools.Rustscan != "/custom/rustscan" || cfg.Scan.Profile != "slow" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

type serverSequenceRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
}

func (r *serverSequenceRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
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

func TestConfigPageRawEditorRejectsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	original := "tools:\n  rustscan: /old/rustscan\nscan:\n  ports: top100\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db")})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)
	form := strings.NewReader("mode=raw&raw_config=tools%3A+%5Bbroken")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "invalid") || !strings.Contains(res.Body.String(), "raw_config") {
		t.Fatalf("expected validation message and raw editor: %s", res.Body.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("config should remain unchanged: %s", data)
	}
}

func TestImportNmapFormRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/import/nmap", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "导入 Nmap XML") || !strings.Contains(body, `name="xml_file"`) {
		t.Fatalf("expected import form, got: %s", body)
	}
	if !strings.Contains(body, `name="project_id"`) || !strings.Contains(body, "Local Lab") {
		t.Fatalf("expected project selector with project name, got: %s", body)
	}
}

func TestImportNmapRunRedirectsToRun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "scan.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte(`<nmaprun>
  <host>
    <address addr="10.0.0.53"/>
    <ports>
      <port protocol="tcp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
    </ports>
  </host>
</nmaprun>`)); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", res.Code)
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/runs/") {
		t.Fatalf("expected redirect to /runs/, got %q", location)
	}

	// 验证 DB 有完成态 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("expected one completed run, got %d err=%v", len(runs), err)
	}
}

func TestImportNmapRunEmptyFileRendersFormError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "empty.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte("")); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 (form re-render), got %d", res.Code)
	}
	pageBody := res.Body.String()
	if !strings.Contains(pageBody, "empty XML file") {
		t.Fatalf("expected error banner, got: %s", pageBody)
	}

	// 验证 DB 无新增 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}

func TestImportNmapRunNonNmaprunRendersFormError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "foo.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte(`<foo><bar/></foo>`)); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %s", res.Body.String())
	}

	// 验证 DB 无新增 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
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
