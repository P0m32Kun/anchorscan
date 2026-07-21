package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

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
	appScript := strings.Index(body, `<script src="/static/app.js" defer></script>`)
	toolFormScript := strings.Index(body, `<script src="/static/tool-form.js" defer></script>`)
	if appScript == -1 || toolFormScript == -1 || appScript > toolFormScript {
		t.Fatalf("expected app.js before tool-form.js: %s", body)
	}
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

func TestToolPageBindsProjectZoneAndVerification(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	zones, _ := scanStore.ListProjectZones("p1")
	var zoneID string
	for _, z := range zones {
		zoneID = z.ZoneID
		break
	}
	v := store.Verification{
		ID:               "v1",
		ProjectID:        "p1",
		ZoneID:           zoneID,
		VulnerabilityKey: "key-1",
		Outcome:          "confirmed",
		Title:            "弱口令",
		Severity:         "high",
		Description:      "发现弱口令",
		Remediation:      "修改密码",
		CreatedAt:        time.Unix(1, 0),
		UpdatedAt:        time.Unix(1, 0),
	}
	if err := scanStore.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}
	scanStore.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	q := "project_id=p1&zone_id=" + zoneID + "&verification_id=v1&return=/projects/p1/workbench"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/tools/nuclei?"+q, nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{
		`value="v1"`,
		`value="` + zoneID + `"`,
		"弱口令",
		"返回工作台",
		"运行完成后可返回工作台上传证据",
		`href="/projects/p1/workbench"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in body: %s", want, body)
		}
	}
}

func TestToolPageRejectsExternalReturnURL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	scanStore.Close()

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/tools/nuclei?project_id=p1&return=https://evil.com/callback", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if strings.Contains(body, "evil.com") || strings.Contains(body, `name="return"`) {
		t.Fatalf("external return URL should not be rendered: %s", body)
	}
}

func TestToolCreateSavesZoneAndVerification(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: /opt/httpx\n  nuclei: /opt/nuclei\nscan:\n  ports: 80,443\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := scanStore.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	zones, _ := scanStore.ListProjectZones("p1")
	var zoneID string
	for _, z := range zones {
		zoneID = z.ZoneID
		break
	}
	scanStore.Close()

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

	form := "tool=nmap&project_id=p1&zone_id=" + zoneID + "&verification_id=v1&return=/projects/p1/workbench&mode=alive&target=192.0.2.10&extra_args=--min-rate+50"
	req := httptest.NewRequest(http.MethodPost, "/tools", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d body=%s", res.Code, res.Body.String())
	}
	location := res.Header().Get("Location")
	if !strings.Contains(location, "verification_id=v1") || !strings.Contains(location, "return=%2Fprojects%2Fp1%2Fworkbench") {
		t.Fatalf("unexpected redirect: %s", location)
	}

	scanStore, _ = store.Open(dbPath)
	defer scanStore.Close()
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
	if run.ZoneID != zoneID {
		t.Fatalf("expected zone id %q, got %q", zoneID, run.ZoneID)
	}
	if !strings.Contains(run.ConfigSnapshot, `"verification_id":"v1"`) {
		t.Fatalf("expected verification_id in snapshot: %s", run.ConfigSnapshot)
	}
	if run.Kind != "tool" || run.IncludeInReport {
		t.Fatalf("tool run should be kind=tool and not included in report: %#v", run)
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
