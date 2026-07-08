package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

type toolRunnerFunc func(binary string, args []string) ([]byte, error)

func (f toolRunnerFunc) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	return f(binary, args)
}

func newToolRunStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "scans.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestRunToolRustscanSavesOpenPorts(t *testing.T) {
	st := newToolRunStore(t)
	jsonPath := filepath.Join(t.TempDir(), "report.json")
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte("[80,443]"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-rustscan", Tool: "rustscan", Target: "192.0.2.10", Ports: "80,443", Tools: ToolPaths{Rustscan: "rustscan"}, JSONReportPath: jsonPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	fps, err := st.ListFingerprints("run-rustscan")
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 2 || fps[0].Port != 80 || fps[1].Port != 443 {
		t.Fatalf("fingerprints = %#v", fps)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
}

func TestRunToolNmapServiceSavesFingerprints(t *testing.T) {
	st := newToolRunStore(t)
	xml := `<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH" version="9.6"/></port></ports></host></nmaprun>`
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) { return []byte(xml), nil })

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nmap", Tool: "nmap", Mode: "service", Target: "192.0.2.10", Ports: "22", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fps, err := st.ListFingerprints("run-nmap")
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 1 || fps[0].Service != "ssh" || fps[0].Product != "OpenSSH" {
		t.Fatalf("fingerprints = %#v", fps)
	}
}

func TestRunToolNmapAliveSavesInfoFinding(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-alive", Tool: "nmap", Mode: "alive", Target: "192.0.2.10", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := st.ListFindings("run-alive")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "host-alive" || !strings.Contains(findings[0].Summary, "alive") {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestRunToolHttpxSavesWebFingerprint(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte(`{"url":"http://192.0.2.10:8080","status-code":200,"title":"Lab","tech":["nginx"]}` + "\n"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-httpx", Tool: "httpx", URL: "http://192.0.2.10:8080", Tools: ToolPaths{Httpx: "httpx"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fps, err := st.ListFingerprints("run-httpx")
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 1 || !fps[0].IsWeb || fps[0].Port != 8080 || fps[0].URL != "http://192.0.2.10:8080" {
		t.Fatalf("fingerprints = %#v", fps)
	}
}

func TestRunToolNucleiSavesFindings(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte(`{"template-id":"cve-test","info":{"name":"Example CVE","severity":"high"},"matched-at":"http://192.0.2.10:8080"}` + "\n"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nuclei", Tool: "nuclei", URL: "http://192.0.2.10:8080", Tags: []string{"tomcat"}, Tools: ToolPaths{Nuclei: "nuclei"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := st.ListFindings("run-nuclei")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Source != "nuclei" || findings[0].ID != "cve-test" || findings[0].Severity != "high" {
		t.Fatalf("findings = %#v", findings)
	}
}
