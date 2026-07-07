package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

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
			[]byte("192.168.1.10 -> [8080]\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat" version="9.0.65"/></port></ports></host></nmaprun>`),
			[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
			[]byte(`<nmaprun><host><ports><port protocol="tcp" portid="8080"><script id="http-tomcat-manager" output="manager exposed"/></port></ports></host></nmaprun>`),
			[]byte("{\"template-id\":\"tomcat-default-login\",\"info\":{\"name\":\"Tomcat Default Login\",\"severity\":\"high\"},\"matched-at\":\"http://192.168.1.10:8080\"}\n"),
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
	if len(findings) != 2 {
		t.Fatalf("unexpected findings: %#v", findings)
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
	if len(decoded.Hosts) != 1 || len(decoded.Hosts[0].Ports[0].Findings) != 2 {
		t.Fatalf("unexpected report: %#v", decoded)
	}
}

func TestRunScanPersistsRunLifecycleAndEvents(t *testing.T) {
	runner := &sequenceRunner{outputs: [][]byte{
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

func TestRunScanMarksCanceledWhenContextCanceled(t *testing.T) {
	runner := &cancelRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := ScanOptions{
		RunID:          "run-1",
		ProfileName:    "normal",
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	}
	err = RunScan(ctx, runner, scanStore, opts)
	if err == nil {
		t.Fatal("expected error")
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
			[]byte("192.168.1.10 -> [6379,8080]\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="6379"><state state="open"/><service name="redis" product="Redis"/></port><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		},
		sleeps: []time.Duration{
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
		[]byte("192.168.1.10 -> [8080]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
		[]byte(`<nmaprun><host><ports><port protocol="tcp" portid="8080"><script id="http-tomcat-manager" output="manager exposed"/></port></ports></host></nmaprun>`),
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
	if !runner.hasArgs("/opt/nmap", "--script", "http-tomcat-manager", "-T3") {
		t.Fatalf("expected nmap NSE args in %#v", runner.commands)
	}
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

type cancelRunner struct{}

func (cancelRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	return nil, ctx.Err()
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
