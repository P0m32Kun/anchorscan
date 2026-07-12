package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

var aliveNmapXML = []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`)

func TestFindingFromNucleiUsesResultEndpoint(t *testing.T) {
	fallback := fingerprint.ServiceFingerprint{IP: "172.22.0.1", Port: 8080, Protocol: "tcp"}
	result := tools.NucleiFinding{
		TemplateID: "redis-default-logins",
		Name:       "Redis Default Login",
		Severity:   "high",
		IP:         "172.22.0.1",
		Port:       "6379",
		MatchedAt:  "172.22.0.1:6379",
	}

	finding := findingFromNuclei(result, fallback, nil)
	if finding.IP != "172.22.0.1" || finding.Port != 6379 || finding.Protocol != "tcp" {
		t.Fatalf("finding endpoint = %s:%d/%s, want 172.22.0.1:6379/tcp", finding.IP, finding.Port, finding.Protocol)
	}
	if finding.Target != "172.22.0.1:6379" {
		t.Fatalf("finding target = %q", finding.Target)
	}
}

func TestScanOptionsIncludesTask2MetadataFields(t *testing.T) {
	type fieldCheck struct {
		name string
		typ  reflect.Type
	}

	for _, check := range []fieldCheck{
		{name: "ProfileName", typ: reflect.TypeOf("")},
		{name: "HostWorkers", typ: reflect.TypeOf(0)},
		{name: "ExtraArgs", typ: reflect.TypeOf(ToolExtraArgs{})},
		{name: "ProjectID", typ: reflect.TypeOf("")},
		{name: "ConfigSnapshot", typ: reflect.TypeOf("")},
	} {
		field, ok := reflect.TypeOf(ScanOptions{}).FieldByName(check.name)
		if !ok {
			t.Fatalf("expected ScanOptions.%s", check.name)
		}
		if field.Type != check.typ {
			t.Fatalf("expected ScanOptions.%s type %v, got %v", check.name, check.typ, field.Type)
		}
	}
}

func TestRunScanStoresFingerprintAndWritesJSONReport(t *testing.T) {
	runner := &sequenceRunner{
		outputs: [][]byte{
			aliveNmapXML,
			[]byte("192.168.1.10 -> [8080]\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat" version="9.0.65"/></port></ports></host></nmaprun>`),
			[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
			[]byte("{\"template-id\":\"tomcat-default-login\",\"matcher-name\":\"basic-auth\",\"extractor-results\":[\"admin:admin\"],\"curl-command\":\"curl -u admin:admin http://192.168.1.10:8080/manager/html\",\"info\":{\"name\":\"Tomcat Default Login\",\"severity\":\"high\"},\"matched-at\":\"http://192.168.1.10:8080\"}\n"),
		},
	}

	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:   "run-1",
		Targets: []string{"192.168.1.10"},
		Ports:   "8080",
		Tools: ToolPaths{
			Rustscan: "/opt/rustscan",
			Nmap:     "/opt/nmap",
			Httpx:    "/opt/httpx",
			Nuclei:   "/opt/nuclei",
		},
		JSONReportPath: reportPath,
		NSERules: map[string][]string{
			"http": {"http-tomcat-manager"},
		},
		TagRules: []TagRule{
			{
				Name:       "tomcat",
				Service:    []string{"http"},
				Product:    []string{"apache tomcat"},
				Tech:       []string{"tomcat"},
				NucleiTags: []string{"tomcat"},
				Target:     "url",
			},
		},
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	rows, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].URL != "http://192.168.1.10:8080" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	findings, err := scanStore.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("unexpected findings: %#v", findings)
	}
	for _, finding := range findings {
		if finding.Source != "nuclei" {
			continue
		}
		if !strings.Contains(finding.Output, "curl-command") || !strings.Contains(finding.Output, "admin:admin") {
			t.Fatalf("expected rich nuclei evidence output, got %#v", finding)
		}
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report not written: %v", err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var decoded report.ScanReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(decoded.Hosts) != 1 || len(decoded.Hosts[0].Ports[0].Findings) != 1 {
		t.Fatalf("unexpected report: %#v", decoded)
	}
}

func TestRunScanPersistsRunLifecycleAndEvents(t *testing.T) {
	runner := &sequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProjectID:      "p1",
		ProfileName:    "normal",
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
		ConfigSnapshot: "profile: normal",
	}
	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	run, err := scanStore.GetScanRun("run-1")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.Status != "completed" || run.Profile != "normal" {
		t.Fatalf("unexpected run: %#v", run)
	}
	events, err := scanStore.ListScanEvents("run-1", 20)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) == 0 || events[0].Message == "" {
		t.Fatalf("expected scan events, got %#v", events)
	}
}

// TestRunScanRunsNSEAndNucleiForSSH locks the dual-engine contract: once a
// service is fingerprinted as SSH and rules are configured, both the nmap NSE
// engine AND nuclei (with -tags ssh) must be invoked. SSH is non-web, so httpx
// is never called — the runner output sequence reflects that ordering.
func TestRunScanRunsNSEAndNucleiForSSH(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
		[]byte(`<nmaprun><host><ports><port><script id="ssh2-enum-algos" output="kex_algorithms:..."/></port></ports></host></nmaprun>`),
		[]byte("{\"template-id\":\"ssh-server-info\",\"info\":{\"name\":\"SSH Server Info\",\"severity\":\"info\"},\"matched-at\":\"192.168.1.10:22\"}\n"),
	}}

	opts := ScanOptions{
		RunID:          "run-ssh-dual",
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Nuclei: "/opt/nuclei"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		NSERules: map[string][]string{
			"ssh": {"ssh2-enum-algos", "ssh-hostkey"},
		},
		TagRules: []TagRule{
			{Name: "ssh", Service: []string{"ssh"}, NucleiTags: []string{"ssh", "default-login"}, Target: "hostport"},
		},
	}
	if err := RunScan(context.Background(), runner, newScanStore(t), opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	// NSE: nmap must be invoked with --script for both ssh scripts on port 22.
	if !runner.hasArgs("/opt/nmap", "--script", "ssh2-enum-algos,ssh-hostkey", "-p", "22") {
		t.Fatalf("expected nmap NSE invocation with ssh scripts, commands=%#v", runner.commands)
	}
	// Nuclei: must be invoked with -tags containing ssh, targeting IP:22, jsonl output.
	if !runner.hasArgs("/opt/nuclei", "-tags", "ssh,default-login", "-target", "192.168.1.10:22", "-jsonl") {
		t.Fatalf("expected nuclei invocation with ssh tags, commands=%#v", runner.commands)
	}
}

// TestRunScanRunsNSEAndNucleiForRedis guards against the SSH case being a
// special-cased accident: a second non-web service (redis) must also trigger
// both engines when configured.
func TestRunScanRunsNSEAndNucleiForRedis(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [6379]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="6379"><state state="open"/><service name="redis" product="Redis"/></port></ports></host></nmaprun>`),
		[]byte(`<nmaprun><host><ports><port><script id="redis-info" output="redis_version:7.0.0"/></port></ports></host></nmaprun>`),
		[]byte("{\"template-id\":\"redis-info\",\"info\":{\"name\":\"Redis Info\",\"severity\":\"info\"},\"matched-at\":\"192.168.1.10:6379\"}\n"),
	}}

	opts := ScanOptions{
		RunID:          "run-redis-dual",
		Targets:        []string{"192.168.1.10"},
		Ports:          "6379",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Nuclei: "/opt/nuclei"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		NSERules: map[string][]string{
			"redis": {"redis-info"},
		},
		TagRules: []TagRule{
			{Name: "redis", Service: []string{"redis"}, NucleiTags: []string{"redis", "default-login"}, Target: "hostport"},
		},
	}
	if err := RunScan(context.Background(), runner, newScanStore(t), opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if !runner.hasArgs("/opt/nmap", "--script", "redis-info", "-p", "6379") {
		t.Fatalf("expected nmap NSE invocation with redis-info, commands=%#v", runner.commands)
	}
	if !runner.hasArgs("/opt/nuclei", "-tags", "redis,default-login", "-target", "192.168.1.10:6379", "-jsonl") {
		t.Fatalf("expected nuclei invocation with redis tags, commands=%#v", runner.commands)
	}
}

func TestRunScanSkipsNmapWhenRustscanFindsNoOpenPorts(t *testing.T) {
	runner := &emptyPortRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-empty", Targets: []string{"172.22.0.7"}, Ports: "1-1000", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if runner.nmapCalls != 0 {
		t.Fatalf("expected nmap to be skipped, got %d calls", runner.nmapCalls)
	}
}

func TestRunScanSkipsPortScanWhenHostIsDown(t *testing.T) {
	runner := &downHostRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-down", Targets: []string{"172.22.0.7"}, Ports: "1-65535", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if runner.rustscanCalls != 0 {
		t.Fatalf("expected rustscan to be skipped for down host, got %d calls", runner.rustscanCalls)
	}
}

func TestRunScanUsesAliveSweepResultsAsTargets(t *testing.T) {
	runner := &aliveSweepRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-cidr", Targets: []string{"172.22.0.0/30"}, Ports: "1-1000", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	want := [][]string{
		{"/opt/nmap", "-sn", "172.22.0.0/30", "-oX", "-"},
		{"/opt/rustscan", "-a", "172.22.0.1", "--range", "1-1000", "-g", "--no-banner"},
		{"/opt/rustscan", "-a", "172.22.0.2", "--range", "1-1000", "-g", "--no-banner"},
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("commands = %#v want %#v", runner.commands, want)
	}
}

func TestRunScanAddsManualReviewForRDP(t *testing.T) {
	runner := &sequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.0.2.10 -> [3389]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="3389"><state state="open"/><service name="ms-wbt-server" product="Microsoft Terminal Services"/></port></ports></host></nmaprun>`),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-bluekeep", Targets: []string{"192.0.2.10"}, Ports: "3389", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := scanStore.ListFindings("run-bluekeep")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "manual-review:CVE-2019-0708" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestRunScanMarksCanceledWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &cancelAfterFirstTargetRunner{cancel: cancel}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		HostWorkers:    1,
		Targets:        []string{"192.168.1.10", "192.168.1.11"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}
	err = RunScan(ctx, runner, scanStore, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	run, getErr := scanStore.GetScanRun("run-1")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "canceled" {
		t.Fatalf("status mismatch: %#v", run)
	}
	if runner.calls != 1 {
		t.Fatalf("expected only one target start before cancellation, got %d calls", runner.calls)
	}
}

func TestRunScanMarksCanceledWhenToolIsKilledAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &killedAfterCancelRunner{cancel: cancel}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "slow",
		HostWorkers:    1,
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}
	err = RunScan(ctx, runner, scanStore, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	run, getErr := scanStore.GetScanRun("run-1")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "canceled" {
		t.Fatalf("status mismatch: %#v", run)
	}
}

func TestRunScanLogsNmapHeartbeat(t *testing.T) {
	oldHeartbeat := nmapHeartbeatEvery
	nmapHeartbeatEvery = time.Millisecond
	defer func() { nmapHeartbeatEvery = oldHeartbeat }()

	runner := &sequenceRunner{
		outputs: [][]byte{
			aliveNmapXML,
			[]byte("192.168.1.10 -> [6379,8080]\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="6379"><state state="open"/><service name="redis" product="Redis"/></port><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		},
		sleeps: []time.Duration{
			0,
			0,
			10 * time.Millisecond,
		},
	}

	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	var mu sync.Mutex
	var logs []string
	opts := ScanOptions{
		RunID:          "run-1",
		Targets:        []string{"192.168.1.10"},
		Ports:          "6379,8080",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
		Logf: func(format string, args ...any) {
			mu.Lock()
			defer mu.Unlock()
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	wantSubstrings := []string{
		"nmap 192.168.1.10 ports=[6379 8080] (service detection may be slow)",
		"nmap 192.168.1.10 still running elapsed=",
		"nmap 192.168.1.10 services=2 elapsed=",
	}
	for _, want := range wantSubstrings {
		if !containsLogSubstring(logs, want) {
			t.Fatalf("expected log containing %q in %#v", want, logs)
		}
	}
}

func TestRunScanPassesExtraArgsToTools(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [8080]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
		[]byte("{" + `"template-id":"tomcat-default-login","info":{"name":"Tomcat Default Login","severity":"high"},"matched-at":"http://192.168.1.10:8080"` + "}\n"),
	}}

	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		Targets:        []string{"192.168.1.10"},
		Ports:          "8080",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Httpx: "/opt/httpx", Nuclei: "/opt/nuclei"},
		JSONReportPath: reportPath,
		ExtraArgs: ToolExtraArgs{
			Rustscan: []string{"--batch-size", "500"},
			Nmap:     []string{"-T3"},
			Httpx:    []string{"-rate-limit", "100"},
			Nuclei:   []string{"-rate-limit", "50"},
		},
		NSERules: map[string][]string{
			"http": {"http-tomcat-manager"},
		},
		TagRules: []TagRule{{Name: "tomcat", Service: []string{"http"}, Product: []string{"tomcat"}, NucleiTags: []string{"tomcat"}, Target: "url"}},
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	for _, check := range []struct{ binary, arg string }{
		{"/opt/rustscan", "--batch-size"},
		{"/opt/httpx", "-rate-limit"},
		{"/opt/nuclei", "-rate-limit"},
	} {
		if !runner.hasArg(check.binary, check.arg) {
			t.Fatalf("expected %s arg %s in %#v", check.binary, check.arg, runner.commands)
		}
	}
	if !runner.hasArgs("/opt/nmap", "-sV", "--version-intensity", "7", "-T3") {
		t.Fatalf("expected nmap fingerprint args in %#v", runner.commands)
	}
}

func TestRunScanRespectsProfileHostWorkersAfterAliveSweep(t *testing.T) {
	for _, tc := range []struct {
		name    string
		workers int
	}{
		{name: "slow", workers: 1},
		{name: "normal", workers: 3},
		{name: "fast", workers: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := []string{
				"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4",
				"10.0.0.5", "10.0.0.6", "10.0.0.7", "10.0.0.8",
				"10.0.0.9", "10.0.0.10", "10.0.0.11", "10.0.0.12",
			}
			runner := newPostAliveConcurrencyRunner(targets, tc.workers)
			dbPath := filepath.Join(t.TempDir(), "scan.db")
			reportPath := filepath.Join(t.TempDir(), "report.json")
			scanStore, err := store.Open(dbPath)
			if err != nil {
				t.Fatalf("Open returned error: %v", err)
			}

			opts := ScanOptions{
				RunID:          "run-" + tc.name,
				ProfileName:    tc.name,
				HostWorkers:    tc.workers,
				Targets:        []string{"10.0.0.0/28"},
				Ports:          "22",
				Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
				JSONReportPath: reportPath,
			}

			if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
				t.Fatalf("RunScan returned error: %v", err)
			}
			if runner.aliveCalls != 1 {
				t.Fatalf("expected one alive sweep, got %d", runner.aliveCalls)
			}
			if runner.maxActive != tc.workers {
				t.Fatalf("expected max active %d, got %d", tc.workers, runner.maxActive)
			}
			if runner.rustscanCalls != len(targets) {
				t.Fatalf("expected %d rustscan calls, got %d", len(targets), runner.rustscanCalls)
			}
		})
	}
}

func TestRunScanContinuesAfterTargetFailure(t *testing.T) {
	runner := &failFirstRunner{outputs: [][]byte{
		[]byte("192.168.1.11 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.11" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		HostWorkers:    1,
		Targets:        []string{"192.168.1.10", "192.168.1.11"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	fps, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 1 || fps[0].IP != "192.168.1.11" {
		t.Fatalf("unexpected fingerprints: %#v", fps)
	}

	events, err := scanStore.ListScanEvents("run-1", 20)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if !containsEvent(events, "error", "target", "192.168.1.10") {
		t.Fatalf("expected target error event, got %#v", events)
	}
}

func TestRunScanReturnsErrorWhenAllTargetsFail(t *testing.T) {
	runner := failRunner{err: fmt.Errorf("boom")}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		HostWorkers:    2,
		Targets:        []string{"192.168.1.10", "192.168.1.11"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}

	err = RunScan(context.Background(), runner, scanStore, opts)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "all targets failed") {
		t.Fatalf("expected all-targets-failed error, got %v", err)
	}
}

func TestRunScanWritesAuditArtifacts(t *testing.T) {
	dir := t.TempDir()
	scanStore := newScanStore(t)
	runner := runnerFunc(func(_ context.Context, binary string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case binary == "nmap" && strings.Contains(joined, "-sn"):
			return []byte(`<nmaprun><host><status state="up"/><address addr="127.0.0.1"/></host></nmaprun>`), nil
		case binary == "rustscan":
			return []byte("127.0.0.1 -> [80]\n"), nil
		case binary == "nmap" && strings.Contains(joined, "-sV"):
			return []byte(`<nmaprun><host><address addr="127.0.0.1"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`), nil
		case binary == "httpx":
			return []byte("{\"url\":\"http://127.0.0.1:80\",\"status-code\":200,\"title\":\"ok\",\"tech\":[\"nginx\"]}\n"), nil
		default:
			return []byte(""), nil
		}
	})

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-artifacts",
		Targets:        []string{"127.0.0.1"},
		Ports:          "80",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Httpx: "httpx"},
		ProfileName:    "normal",
		HostWorkers:    1,
		ArtifactRoot:   dir,
		JSONReportPath: filepath.Join(dir, "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	artifactDir := filepath.Join(dir, "run-artifacts")
	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	joinedNames := strings.Join(names, "\n")
	for _, want := range []string{"nmap-alive", "rustscan-127.0.0.1-ports", "nmap-service-127.0.0.1", "httpx-127.0.0.1-80"} {
		if !strings.Contains(joinedNames, want) {
			t.Fatalf("missing artifact %q in files:\n%s", want, joinedNames)
		}
	}

	run, err := scanStore.GetScanRun("run-artifacts")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.ArtifactDir != artifactDir {
		t.Fatalf("artifact dir mismatch: got %q want %q", run.ArtifactDir, artifactDir)
	}
}

func TestRunScanWritesFailedNucleiOutputArtifact(t *testing.T) {
	dir := t.TempDir()
	scanStore := newScanStore(t)
	runner := runnerFunc(func(_ context.Context, binary string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case binary == "nmap" && strings.Contains(joined, "-sn"):
			return []byte(`<nmaprun><host><status state="up"/><address addr="127.0.0.1"/></host></nmaprun>`), nil
		case binary == "rustscan":
			return []byte("127.0.0.1 -> [80]\n"), nil
		case binary == "nmap" && strings.Contains(joined, "-sV"):
			return []byte(`<nmaprun><host><address addr="127.0.0.1"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`), nil
		case binary == "httpx":
			return []byte("{\"url\":\"http://127.0.0.1:80\",\"status-code\":200,\"title\":\"ok\",\"tech\":[\"nginx\"]}\n"), nil
		case binary == "nuclei":
			return []byte("{\"template-id\":\"demo\",\"info\":{\"name\":\"demo\",\"severity\":\"medium\"},\"matched-at\":\"http://127.0.0.1:80\"}\n"), errors.New("exit status 1")
		default:
			return []byte(""), nil
		}
	})

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-failed-nuclei",
		Targets:        []string{"127.0.0.1"},
		Ports:          "80",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Httpx: "httpx", Nuclei: "nuclei"},
		ProfileName:    "normal",
		HostWorkers:    1,
		ArtifactRoot:   dir,
		JSONReportPath: filepath.Join(dir, "report.json"),
		TagRules: []TagRule{
			{
				Name:       "nginx",
				Service:    []string{"http"},
				Product:    []string{"nginx"},
				Tech:       []string{"nginx"},
				NucleiTags: []string{"demo"},
				Target:     "url",
			},
		},
	})
	if err == nil {
		t.Fatal("expected scan error")
	}

	entries, readErr := os.ReadDir(filepath.Join(dir, "run-failed-nuclei"))
	if readErr != nil {
		t.Fatalf("ReadDir returned error: %v", readErr)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "nuclei-127.0.0.1-80-demo") {
			return
		}
	}
	t.Fatalf("expected failed nuclei artifact, got %#v", entries)
}

type sequenceRunner struct {
	outputs [][]byte
	sleeps  []time.Duration
	index   int
}

func (s *sequenceRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	if s.index < len(s.sleeps) {
		time.Sleep(s.sleeps[s.index])
	}
	out := s.outputs[s.index]
	s.index++
	return out, nil
}

type runnerFunc func(ctx context.Context, binary string, args []string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, binary string, args []string) ([]byte, error) {
	return f(ctx, binary, args)
}

func newScanStore(t *testing.T) *store.Store {
	t.Helper()
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return scanStore
}

type cancelRunner struct{}

func (cancelRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	return nil, ctx.Err()
}

type cancelAfterFirstTargetRunner struct {
	cancel func()
	calls  int
}

func (r *cancelAfterFirstTargetRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	r.calls++
	if r.calls == 1 {
		r.cancel()
		<-ctx.Done()
	}
	return nil, ctx.Err()
}

type killedAfterCancelRunner struct {
	cancel func()
	calls  int
}

func (r *killedAfterCancelRunner) Run(ctx context.Context, binary string, _ []string) ([]byte, error) {
	r.calls++
	switch {
	case binary == "/opt/rustscan":
		return []byte("192.168.1.10 -> [22]\n"), nil
	case binary == "/opt/nmap":
		r.cancel()
		<-ctx.Done()
		return nil, fmt.Errorf("signal: killed")
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type recordingSequenceRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
}

func (r *recordingSequenceRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

func (r *recordingSequenceRunner) hasArg(binary string, arg string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, got := range cmd[1:] {
			if got == arg {
				return true
			}
		}
	}
	return false
}

func (r *recordingSequenceRunner) hasArgs(binary string, args ...string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, arg := range args {
			found := false
			for _, got := range cmd[1:] {
				if got == arg {
					found = true
					break
				}
			}
			if !found {
				goto next
			}
		}
		return true
	next:
	}
	return false
}

func containsLogSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

type blockingRunner struct {
	mu        sync.Mutex
	active    int
	maxActive int
}

func (r *blockingRunner) Run(_ context.Context, binary string, _ []string) ([]byte, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	switch binary {
	case "/opt/rustscan":
		return []byte("127.0.0.1 -> [22]\n"), nil
	case "/opt/nmap":
		return []byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`), nil
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type postAliveConcurrencyRunner struct {
	mu            sync.Mutex
	targets       []string
	wantActive    int
	release       chan struct{}
	released      bool
	aliveCalls    int
	rustscanCalls int
	active        int
	maxActive     int
}

func newPostAliveConcurrencyRunner(targets []string, wantActive int) *postAliveConcurrencyRunner {
	return &postAliveConcurrencyRunner{
		targets:    targets,
		wantActive: wantActive,
		release:    make(chan struct{}),
	}
}

func (r *postAliveConcurrencyRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	if binary == "/opt/nmap" && len(args) > 0 && args[0] == "-sn" {
		r.mu.Lock()
		r.aliveCalls++
		r.mu.Unlock()
		return []byte(aliveSweepXML(r.targets...)), nil
	}
	if binary != "/opt/rustscan" {
		return nil, fmt.Errorf("unexpected command %s %v", binary, args)
	}

	r.mu.Lock()
	r.rustscanCalls++
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	if r.maxActive >= r.wantActive && !r.released {
		close(r.release)
		r.released = true
	}
	r.mu.Unlock()

	select {
	case <-r.release:
	case <-time.After(time.Second):
	}

	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return []byte("127.0.0.1 -> []\n"), nil
}

type failFirstRunner struct {
	mu      sync.Mutex
	outputs [][]byte
	index   int
	failed  bool
}

func (r *failFirstRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if binary == "/opt/nmap" && len(args) > 0 && args[0] == "-sn" {
		return []byte(aliveSweepXML("192.168.1.10", "192.168.1.11")), nil
	}
	if !r.failed {
		r.failed = true
		return nil, fmt.Errorf("boom")
	}
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

type failRunner struct {
	err error
}

func (r failRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	if binary == "/opt/nmap" && len(args) > 0 && args[0] == "-sn" {
		return []byte(aliveSweepXML("192.168.1.10", "192.168.1.11")), nil
	}
	return nil, r.err
}

func aliveSweepXML(targets ...string) string {
	var b strings.Builder
	b.WriteString("<nmaprun>")
	for _, target := range targets {
		b.WriteString(`<host><status state="up"/><address addr="`)
		b.WriteString(target)
		b.WriteString(`" addrtype="ipv4"/></host>`)
	}
	b.WriteString("</nmaprun>")
	return b.String()
}

type emptyPortRunner struct {
	nmapCalls int
}

func (r *emptyPortRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	switch binary {
	case "/opt/nmap":
		if len(args) > 0 && args[0] == "-sn" {
			return aliveNmapXML, nil
		}
		r.nmapCalls++
		return nil, fmt.Errorf("nmap should not run without open ports")
	case "/opt/rustscan":
		return []byte("172.22.0.7 -> []\n"), nil
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type downHostRunner struct {
	rustscanCalls int
}

func (r *downHostRunner) Run(_ context.Context, binary string, _ []string) ([]byte, error) {
	switch binary {
	case "/opt/nmap":
		return []byte(`<nmaprun><host><status state="down"/></host></nmaprun>`), nil
	case "/opt/rustscan":
		r.rustscanCalls++
		return nil, fmt.Errorf("rustscan should not run for down host")
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type aliveSweepRunner struct {
	commands [][]string
}

func (r *aliveSweepRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
	switch {
	case binary == "/opt/nmap":
		return []byte(`<nmaprun><host><status state="up"/><address addr="172.22.0.1" addrtype="ipv4"/></host><host><status state="up"/><address addr="172.22.0.2" addrtype="ipv4"/></host></nmaprun>`), nil
	case binary == "/opt/rustscan" && len(args) >= 2 && args[1] == "172.22.0.1":
		return []byte("172.22.0.1 -> []\n"), nil
	case binary == "/opt/rustscan" && len(args) >= 2 && args[1] == "172.22.0.2":
		return []byte("172.22.0.2 -> []\n"), nil
	default:
		return nil, fmt.Errorf("unexpected command %s %v", binary, args)
	}
}

func containsEvent(events []store.ScanEvent, level string, stage string, target string) bool {
	for _, event := range events {
		if event.Level == level && event.Stage == stage && strings.Contains(event.Message, target) {
			return true
		}
	}
	return false
}
