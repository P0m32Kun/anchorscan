package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

var rdpNmapXML = []byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="3389"><state state="open"/><service name="ms-wbt-server" product="Microsoft Terminal Services"/></port></ports></host></nmaprun>`)

func rdpscanCheck(t *testing.T, checks []store.DetectionCheck) store.DetectionCheck {
	t.Helper()
	for _, c := range checks {
		if c.Engine == "rdpscan" {
			return c
		}
	}
	t.Fatalf("no rdpscan check in %#v", checks)
	return store.DetectionCheck{}
}

func rdpscanFinding(findings []report.Finding) (report.Finding, bool) {
	for _, f := range findings {
		if f.Source == "rdpscan" {
			return f, true
		}
	}
	return report.Finding{}, false
}

func TestRunScanTriggersRdpscanForRDP(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [3389]\n"),
		rdpNmapXML,
		[]byte("192.168.1.10:3389 - VULNERABLE\n"),
	}}
	scanStore := newScanStore(t)
	if err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-rdp-bluekeep",
		Targets:        []string{"192.168.1.10"},
		Ports:          "3389",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Rdpscan: "/opt/rdpscan"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if !runner.hasArgs("/opt/rdpscan", "192.168.1.10:3389") {
		t.Fatalf("expected rdpscan invocation, commands=%#v", runner.commands)
	}

	findings, err := scanStore.ListFindings("run-rdp-bluekeep")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	finding, ok := rdpscanFinding(findings)
	if !ok {
		t.Fatalf("expected rdpscan finding, got %#v", findings)
	}
	if finding.ID != "CVE-2019-0708" || finding.Severity != "critical" {
		t.Fatalf("finding = %#v", finding)
	}

	checks, err := scanStore.ListDetectionChecks("run-rdp-bluekeep")
	if err != nil {
		t.Fatalf("ListDetectionChecks returned error: %v", err)
	}
	check := rdpscanCheck(t, checks)
	if check.Status != "completed" {
		t.Fatalf("rdpscan check status = %q, want completed", check.Status)
	}
}

func TestRunScanRdpscanSafeDoesNotCreateFinding(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [3389]\n"),
		rdpNmapXML,
		[]byte("192.168.1.10:3389 - SAFE - target appears patched\n"),
	}}
	scanStore := newScanStore(t)
	if err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-rdp-safe",
		Targets:        []string{"192.168.1.10"},
		Ports:          "3389",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Rdpscan: "/opt/rdpscan"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	findings, err := scanStore.ListFindings("run-rdp-safe")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if _, ok := rdpscanFinding(findings); ok {
		t.Fatalf("rdpscan SAFE must not create a finding, got %#v", findings)
	}

	checks, err := scanStore.ListDetectionChecks("run-rdp-safe")
	if err != nil {
		t.Fatalf("ListDetectionChecks returned error: %v", err)
	}
	check := rdpscanCheck(t, checks)
	if check.Status != "completed" {
		t.Fatalf("rdpscan check status = %q, want completed", check.Status)
	}
}

func TestRunScanRdpscanUnknownDoesNotCreateFinding(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [3389]\n"),
		rdpNmapXML,
		[]byte("192.168.1.10:3389 - UNKNOWN - NLA required\n"),
	}}
	scanStore := newScanStore(t)
	if err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-rdp-unknown",
		Targets:        []string{"192.168.1.10"},
		Ports:          "3389",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Rdpscan: "/opt/rdpscan"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	findings, err := scanStore.ListFindings("run-rdp-unknown")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if _, ok := rdpscanFinding(findings); ok {
		t.Fatalf("rdpscan UNKNOWN must not create a finding, got %#v", findings)
	}

	checks, err := scanStore.ListDetectionChecks("run-rdp-unknown")
	if err != nil {
		t.Fatalf("ListDetectionChecks returned error: %v", err)
	}
	check := rdpscanCheck(t, checks)
	if check.Status != "completed" {
		t.Fatalf("rdpscan check status = %q, want completed", check.Status)
	}
}

func TestRunScanSkipsRdpscanWhenUnconfigured(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [3389]\n"),
		rdpNmapXML,
	}}
	scanStore := newScanStore(t)
	if err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-rdp-unconfigured",
		Targets:        []string{"192.168.1.10"},
		Ports:          "3389",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if runner.hasArgs("/opt/rdpscan") {
		t.Fatalf("rdpscan should not be invoked when unconfigured, commands=%#v", runner.commands)
	}

	checks, err := scanStore.ListDetectionChecks("run-rdp-unconfigured")
	if err != nil {
		t.Fatalf("ListDetectionChecks returned error: %v", err)
	}
	check := rdpscanCheck(t, checks)
	if check.Status != "skipped" || check.ReasonCode != "tool_unconfigured" {
		t.Fatalf("rdpscan check = %#v, want skipped/tool_unconfigured", check)
	}
}

func TestRunScanSkipsRdpscanForNonRDP(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		aliveNmapXML,
		[]byte("192.168.1.10 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
	}}
	scanStore := newScanStore(t)
	if err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-ssh-rdpscan",
		Targets:        []string{"192.168.1.10"},
		Ports:          "22",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Rdpscan: "/opt/rdpscan"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if runner.hasArgs("/opt/rdpscan") {
		t.Fatalf("rdpscan should not be invoked for non-RDP service, commands=%#v", runner.commands)
	}

	checks, err := scanStore.ListDetectionChecks("run-ssh-rdpscan")
	if err != nil {
		t.Fatalf("ListDetectionChecks returned error: %v", err)
	}
	check := rdpscanCheck(t, checks)
	if check.Status != "skipped" || check.ReasonCode != "no_matching_rule" {
		t.Fatalf("rdpscan check = %#v, want skipped/no_matching_rule", check)
	}
}
