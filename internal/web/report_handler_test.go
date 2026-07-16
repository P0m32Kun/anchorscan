package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

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

func TestReportPageRendersMatchedVulnerabilityAggregate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(filepath.Join(dir, "handbook.md"), []byte(knowledgeBaseFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: handbook.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-aggregate", Target: "192.0.2.0/24", Ports: "445", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 51; i++ {
		if err := scanStore.SaveFinding("run-aggregate", report.Finding{IP: "192.0.2." + strconv.Itoa(i), Port: 445, Protocol: "tcp", Source: "nuclei", ID: "smb-signing", Severity: "high", Summary: "SMB signing"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanStore.SaveFinding("run-aggregate", report.Finding{IP: "198.51.100.10", Port: 80, Source: "nuclei", ID: "smb-signing", Severity: "info", Summary: "info-only"}); err != nil {
		t.Fatal(err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-aggregate?view=vulnerabilities&findings_page=2", nil))
	body := res.Body.String()
	for _, want := range []string{"SMB 签名未启用（中危）", "描述。", "启用签名。", "192.0.2.1:445/tcp", "192.0.2.51:445/tcp"} {
		if !strings.Contains(body, want) {
			t.Fatalf("aggregate response missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "info-only") {
		t.Fatalf("aggregate response included info finding: %s", body)
	}
	if copies := strings.Count(body, "data-copy-text="); copies != 5 {
		t.Fatalf("aggregate response has %d copy controls, want 5: %s", copies, body)
	}
	if !strings.Contains(body, `value="vulnerabilities" selected`) {
		t.Fatalf("aggregate response did not keep the selected view: %s", body)
	}

	results := httptest.NewRecorder()
	handler.ServeHTTP(results, httptest.NewRequest(http.MethodGet, "/reports/run-aggregate?severity=info", nil))
	if results.Code != http.StatusOK || !strings.Contains(results.Body.String(), "info-only") {
		t.Fatalf("vulnerability view unexpectedly changed the existing results: %d %s", results.Code, results.Body.String())
	}
}

func TestReportPageVulnerabilityAggregateFormatsAssetsAndEscapesHTML(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	configPath := filepath.Join(dir, "config.yaml")
	handbook := strings.Replace(knowledgeBaseFixture, "描述。", "<b>描述</b>", 1)
	if err := os.WriteFile(filepath.Join(dir, "handbook.md"), []byte(handbook), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("knowledge_base:\n  path: handbook.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-assets", Target: "192.0.2.0/24", Ports: "445", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatal(err)
	}
	for _, finding := range []report.Finding{
		{IP: "192.0.2.1", Port: 445, Protocol: "TCP", Source: "nuclei", ID: "smb-signing", Severity: "high"},
		{IP: "192.0.2.1", Port: 445, Protocol: "tcp", Source: "nuclei", ID: "smb-signing", Severity: "high"},
		{IP: "2001:0db8:0:0:0:0:0:1", Port: 445, Protocol: "tcp", Source: "nuclei", ID: "smb-signing", Severity: "high"},
		{IP: "192.0.2.2", Protocol: "UDP", Source: "nuclei", ID: "smb-signing", Severity: "high"},
		{Port: 8443, Protocol: "tcp", Target: "target.example", Source: "nuclei", ID: "smb-signing", Severity: "high"},
	} {
		if err := scanStore.SaveFinding("run-assets", finding); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-assets?view=vulnerabilities", nil))
	body := res.Body.String()
	for _, want := range []string{"192.0.2.1:445/tcp", "192.0.2.2/udp", "[2001:db8::1]:445/tcp", "target.example:8443/tcp", "&lt;b&gt;描述&lt;/b&gt;"} {
		if !strings.Contains(body, want) {
			t.Fatalf("aggregate response missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, `<pre><b>描述</b></pre>`) {
		t.Fatalf("aggregate response rendered unescaped description: %s", body)
	}
	if strings.Contains(body, "<pre>192.0.2.1:445/tcp\n192.0.2.1:445/tcp") {
		t.Fatalf("aggregate response did not deduplicate assets: %s", body)
	}
	if first, second := strings.Index(body, "192.0.2.1:445/tcp"), strings.Index(body, "192.0.2.2/udp"); first < 0 || second < first {
		t.Fatalf("aggregate assets are not stably sorted: %s", body)
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
	appScript := strings.Index(body, `<script src="/static/app.js"></script>`)
	reportUIScript := strings.Index(body, `<script src="/static/report-ui.js"></script>`)
	if appScript == -1 || reportUIScript == -1 || appScript > reportUIScript {
		t.Fatalf("expected app.js before report-ui.js: %s", body)
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
