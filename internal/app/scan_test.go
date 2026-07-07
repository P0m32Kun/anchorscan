package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

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

func containsLogSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
