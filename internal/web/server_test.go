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
}

func TestCreateProjectFromWeb(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

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
}

func TestScanCreateUsesProjectDefaultsAndExclusions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: 80,443\n  profile: normal\nprofiles:\n  slow:\n    host_workers: 1\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
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
	if !runner.hasArgs("/opt/rustscan", "-a", "127.0.0.1", "--ports", "80,8080") {
		t.Fatalf("unexpected rustscan args: %#v", runner.commands)
	}
	if runner.callCount("/opt/rustscan") != 1 {
		t.Fatalf("expected single rustscan target after exclusions, got %#v", runner.commands)
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
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "redis-default-logins") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "<details") || !strings.Contains(res.Body.String(), "matched-at") {
		t.Fatalf("expected finding details in body: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "筛选") || !strings.Contains(res.Body.String(), "证据与详情") {
		t.Fatalf("expected chinese report copy: %s", res.Body.String())
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
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1?view=hosts&q=redis", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "按主机聚合") || !strings.Contains(body, "复制 IP:PORT") || !strings.Contains(body, "/reports/run-1/assets.csv?q=redis") {
		t.Fatalf("expected asset workbench controls: %s", body)
	}
	if !strings.Contains(body, "127.0.0.1") || !strings.Contains(body, "6379,6380") {
		t.Fatalf("expected grouped host row: %s", body)
	}
	if strings.Contains(body, "127.0.0.2") {
		t.Fatalf("expected redis filter to exclude non-matching host: %s", body)
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

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1/assets.txt?q=redis&kind=ip_port", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("txt status mismatch: %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("unexpected txt content-type: %s", ct)
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
	csvBody := res.Body.String()
	if !strings.Contains(csvBody, "ip,port,service,product,version,url") || !strings.Contains(csvBody, "127.0.0.1,6379,redis,Redis,7.2.0,") {
		t.Fatalf("unexpected csv export: %s", csvBody)
	}
	if strings.Contains(csvBody, "127.0.0.2") {
		t.Fatalf("expected filtered csv export: %s", csvBody)
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
