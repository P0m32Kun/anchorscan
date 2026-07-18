package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

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
			{Name: "ssh", Service: []string{"ssh"}, NucleiTags: []string{"ssh"}, ExcludeTags: []string{"default-login"}, Target: "hostport"},
		},
	}
	scanStore := newScanStore(t)
	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	// NSE: nmap must be invoked with --script for both ssh scripts on port 22.
	if !runner.hasArgs("/opt/nmap", "--script", "ssh2-enum-algos,ssh-hostkey", "-p", "22") {
		t.Fatalf("expected nmap NSE invocation with ssh scripts, commands=%#v", runner.commands)
	}
	// Nuclei: must be invoked with -tags ssh and -etags default-login (exclude official
	// brute-force templates; the mini-brute template carries ssh tag but not default-login),
	// targeting IP:22, jsonl output.
	if !runner.hasArgs("/opt/nuclei", "-tags", "ssh", "-etags", "default-login", "-target", "192.168.1.10:22", "-jsonl") {
		t.Fatalf("expected nuclei invocation with ssh tags and default-login etags, commands=%#v", runner.commands)
	}
	checks, err := scanStore.ListDetectionChecks("run-ssh-dual")
	if err != nil || len(checks) != 2 || checks[0].Status != "completed" || checks[1].Status != "completed" {
		t.Fatalf("detection checks = %#v, %v", checks, err)
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
			{Name: "redis", Service: []string{"redis"}, NucleiTags: []string{"redis"}, Target: "hostport"},
		},
	}
	if err := RunScan(context.Background(), runner, newScanStore(t), opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if !runner.hasArgs("/opt/nmap", "--script", "redis-info", "-p", "6379") {
		t.Fatalf("expected nmap NSE invocation with redis-info, commands=%#v", runner.commands)
	}
	if !runner.hasArgs("/opt/nuclei", "-tags", "redis", "-target", "192.168.1.10:6379", "-jsonl") {
		t.Fatalf("expected nuclei invocation with redis tags, commands=%#v", runner.commands)
	}
}

func TestRunScanContinuesAfterNSEFailure(t *testing.T) {
	runner := &recordingSequenceRunner{
		outputs: [][]byte{
			aliveNmapXML,
			[]byte("192.168.1.10 -> [22]\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
			[]byte(`<nmaprun/>`),
			[]byte("{\"template-id\":\"ssh-server-info\",\"info\":{\"name\":\"SSH Server Info\",\"severity\":\"info\"},\"matched-at\":\"192.168.1.10:22\"}\n"),
		},
		errors: []error{nil, nil, nil, errors.New("nse unavailable"), nil},
	}
	scanStore := newScanStore(t)
	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-nse-failed",
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Nuclei: "/opt/nuclei"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		NSERules:       map[string][]string{"ssh": {"ssh2-enum-algos"}},
		TagRules:       []TagRule{{Name: "ssh", Service: []string{"ssh"}, NucleiTags: []string{"ssh"}, Target: "hostport"}},
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	run, err := scanStore.GetScanRun("run-nse-failed")
	if err != nil || run.Status != "completed_with_errors" {
		t.Fatalf("run = %#v, %v", run, err)
	}
	fingerprints, err := scanStore.ListFingerprints("run-nse-failed")
	if err != nil || len(fingerprints) != 1 {
		t.Fatalf("fingerprints = %#v, %v", fingerprints, err)
	}
	findings, err := scanStore.ListFindings("run-nse-failed")
	if err != nil || len(findings) != 1 || findings[0].Source != "nuclei" {
		t.Fatalf("findings = %#v, %v", findings, err)
	}
	checks, err := scanStore.ListDetectionChecks("run-nse-failed")
	if err != nil || len(checks) != 2 || checks[0].Engine != "nse" || checks[0].Status != "failed" || checks[1].Engine != "nuclei" || checks[1].Status != "completed" {
		t.Fatalf("checks = %#v, %v", checks, err)
	}
}

func TestRunScanContinuesAfterHTTPXFailure(t *testing.T) {
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
			return []byte("httpx unavailable"), errors.New("exit status 1")
		case binary == "nuclei":
			return []byte("{\"template-id\":\"demo\",\"info\":{\"name\":\"demo\",\"severity\":\"medium\"},\"matched-at\":\"http://127.0.0.1:80\"}\n"), nil
		default:
			return nil, fmt.Errorf("unexpected command %s %s", binary, joined)
		}
	})
	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-httpx-failed",
		Targets:        []string{"127.0.0.1"},
		Ports:          "80",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Httpx: "httpx", Nuclei: "nuclei"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		TagRules:       []TagRule{{Name: "nginx", Service: []string{"http"}, Product: []string{"nginx"}, NucleiTags: []string{"demo"}, Target: "url"}},
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	run, err := scanStore.GetScanRun("run-httpx-failed")
	if err != nil || run.Status != "completed_with_errors" {
		t.Fatalf("run = %#v, %v", run, err)
	}
	fingerprints, err := scanStore.ListFingerprints("run-httpx-failed")
	if err != nil || len(fingerprints) != 1 {
		t.Fatalf("fingerprints = %#v, %v", fingerprints, err)
	}
	findings, err := scanStore.ListFindings("run-httpx-failed")
	if err != nil || len(findings) != 1 || findings[0].Source != "nuclei" {
		t.Fatalf("findings = %#v, %v", findings, err)
	}
}

func TestRunScanKeepsEarlierFindingWhenLaterStageIsCanceled(t *testing.T) {
	scanStore := newScanStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := runnerFunc(func(ctx context.Context, binary string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case binary == "nmap" && strings.Contains(joined, "-sn"):
			return []byte(`<nmaprun><host><status state="up"/><address addr="192.0.2.10"/></host></nmaprun>`), nil
		case binary == "rustscan":
			return []byte("192.0.2.10 -> [3389,6379]\n"), nil
		case binary == "nmap" && strings.Contains(joined, "-sV"):
			return []byte(`<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="3389"><state state="open"/><service name="ms-wbt-server" product="Microsoft Terminal Services"/></port><port protocol="tcp" portid="6379"><state state="open"/><service name="redis" product="Redis"/></port></ports></host></nmaprun>`), nil
		case binary == "nmap" && strings.Contains(joined, "--script"):
			cancel()
			return nil, ctx.Err()
		default:
			return nil, fmt.Errorf("unexpected command %s %s", binary, joined)
		}
	})
	err := RunScan(ctx, runner, scanStore, ScanOptions{
		RunID:          "run-canceled-after-finding",
		Targets:        []string{"192.0.2.10"},
		Ports:          "3389,6379",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		NSERules:       map[string][]string{"redis": {"redis-info"}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunScan error = %v, want context canceled", err)
	}
	findings, err := scanStore.ListFindings("run-canceled-after-finding")
	if err != nil || len(findings) != 1 || findings[0].ID != "manual-review:CVE-2019-0708" {
		t.Fatalf("findings = %#v, %v", findings, err)
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
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	run, err := scanStore.GetScanRun("run-failed-nuclei")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.Status != "completed_with_errors" {
		t.Fatalf("run status = %q, want completed_with_errors", run.Status)
	}

	entries, readErr := os.ReadDir(filepath.Join(dir, "run-failed-nuclei"))
	if readErr != nil {
		t.Fatalf("ReadDir returned error: %v", readErr)
	}
	foundArtifact := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "nuclei-127.0.0.1-80-demo") {
			foundArtifact = true
		}
	}
	if !foundArtifact {
		t.Fatalf("expected failed nuclei artifact, got %#v", entries)
	}
	checks, err := scanStore.ListDetectionChecks("run-failed-nuclei")
	if err != nil {
		t.Fatalf("ListDetectionChecks returned error: %v", err)
	}
	if len(checks) != 2 || checks[1].Engine != "nuclei" || checks[1].Status != "failed" || checks[1].ReasonCode != "command_failed" {
		t.Fatalf("detection checks = %#v, want failed nuclei command check", checks)
	}
}
