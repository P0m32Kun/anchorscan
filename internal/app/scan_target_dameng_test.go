package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// fakeDamengAuthChecker implements the DamengAuthChecker interface for scan-level tests.
type fakeDamengAuthChecker struct {
	ok         bool
	out        string
	err        error
	panicValue any
	waitForCtx bool
	calls      int
}

func (f *fakeDamengAuthChecker) Check(ctx context.Context, host string, port int, username, password string) (bool, string, error) {
	f.calls++
	if f.waitForCtx {
		<-ctx.Done()
		return false, "", ctx.Err()
	}
	if f.panicValue != nil {
		panic(f.panicValue)
	}
	return f.ok, f.out, f.err
}

// TestRunScanTriggersDamengFinding verifies that when a fingerprint is already
// normalized to "dameng" and the dameng detector is enabled, the dameng engine
// runs, records a completed detection check, and persists a high-severity
// finding for the default password.
func TestRunScanTriggersDamengFinding(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.0.2.10 -> [5236]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="5236"><state state="open"/><service name="padl2sim"/></port></ports></host></nmaprun>`),
		[]byte(`{"template-id":"dameng-detect","ip":"192.0.2.10","port":"5236","extracted-results":["8.1.2.128"]}`),
	}}
	scanStore := newScanStore(t)

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-dameng-vulnerable",
		Targets:        []string{"192.0.2.10"},
		Ports:          "5236",
		DiscoveryMode:  DiscoveryAssumeUp,
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled", Nuclei: "nuclei", NucleiTemplates: "templates"},
		DamengChecker:  &fakeDamengAuthChecker{ok: true},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	findings, err := scanStore.ListFindings("run-dameng-vulnerable")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 dameng finding, got %#v", findings)
	}
	f := findings[0]
	if f.Source != "dameng" || f.ID != "dameng-default-password" || f.Severity != "high" {
		t.Fatalf("unexpected finding: %#v", f)
	}
	if f.Summary != "Dameng Database Default Password (SYSDBA/SYSDBA)" || !strings.Contains(f.Output, "SYSDBA/SYSDBA") {
		t.Fatalf("finding should expose matched credential: %#v", f)
	}

	checks, err := scanStore.ListDetectionChecks("run-dameng-vulnerable")
	if err != nil {
		t.Fatal(err)
	}
	checkByEngine := map[string]store.DetectionCheck{}
	for _, c := range checks {
		checkByEngine[c.Engine] = c
	}
	if c, ok := checkByEngine["dameng"]; !ok || c.Status != "completed" {
		t.Fatalf("expected dameng completed, got %#v", checkByEngine["dameng"])
	}

	fingerprints, err := scanStore.ListFingerprints("run-dameng-vulnerable")
	if err != nil {
		t.Fatal(err)
	}
	if len(fingerprints) != 1 {
		t.Fatalf("expected 1 fingerprint, got %#v", fingerprints)
	}
	fp := fingerprints[0]
	if fp.Service != "dameng" || fp.Product != "Dameng Database" || fp.Normalized != "dameng" || fp.Version != "8.1.2.128" {
		t.Fatalf("unexpected Dameng fingerprint: %#v", fp)
	}
}

func TestRunScanRecordsDamengPanicAsCompletedWithErrors(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.0.2.10 -> [5236]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="5236"><state state="open"/><service name="dameng" product="Dameng DB"/></port></ports></host></nmaprun>`),
		[]byte(`{"template-id":"dameng-detect","ip":"192.0.2.10","port":"5236"}`),
	}}
	scanStore := newScanStore(t)

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-dameng-panic",
		Targets:        []string{"192.0.2.10"},
		Ports:          "5236",
		DiscoveryMode:  DiscoveryAssumeUp,
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled", Nuclei: "nuclei", NucleiTemplates: "templates"},
		DamengChecker:  &fakeDamengAuthChecker{panicValue: "driver index out of range"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	run, err := scanStore.GetScanRun("run-dameng-panic")
	if err != nil || run.Status != "completed_with_errors" {
		t.Fatalf("run = %#v, %v", run, err)
	}
	findings, err := scanStore.ListFindings("run-dameng-panic")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %#v, want none", findings)
	}
	checks, err := scanStore.ListDetectionChecks("run-dameng-panic")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range checks {
		if check.Engine == "dameng" && check.Status == "failed" && check.ReasonCode == "command_failed" {
			if !strings.Contains(check.Detail, "dameng driver panic") {
				t.Fatalf("Dameng failure detail = %q, want driver panic diagnostic", check.Detail)
			}
			return
		}
	}
	t.Fatalf("expected failed Dameng check, got %#v", checks)
}

func TestRunScanRecordsDamengDeadlineAsCompletedWithErrors(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.0.2.11 -> [5236]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.11" addrtype="ipv4"/><ports><port protocol="tcp" portid="5236"><state state="open"/><service name="dameng" product="Dameng DB"/></port></ports></host></nmaprun>`),
		[]byte(`{"template-id":"dameng-detect","ip":"192.0.2.11","port":"5236"}`),
	}}
	scanStore := newScanStore(t)

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-dameng-deadline",
		Targets:        []string{"192.0.2.11"},
		Ports:          "5236",
		DiscoveryMode:  DiscoveryAssumeUp,
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled", Nuclei: "nuclei", NucleiTemplates: "templates"},
		Timeouts:       ToolTimeouts{Dameng: time.Millisecond},
		DamengChecker:  &fakeDamengAuthChecker{waitForCtx: true},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	run, err := scanStore.GetScanRun("run-dameng-deadline")
	if err != nil || run.Status != "completed_with_errors" {
		t.Fatalf("run = %#v, %v", run, err)
	}
	findings, err := scanStore.ListFindings("run-dameng-deadline")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %#v, want none", findings)
	}
	checks, err := scanStore.ListDetectionChecks("run-dameng-deadline")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range checks {
		if check.Engine == "dameng" && check.Status == "failed" && check.ReasonCode == "command_failed" {
			if !strings.Contains(check.Detail, "deadline exceeded") {
				t.Fatalf("Dameng failure detail = %q, want deadline diagnostic", check.Detail)
			}
			return
		}
	}
	t.Fatalf("expected failed Dameng check, got %#v", checks)
}

// TestRunScanSkipsDamengWhenToolUnconfigured verifies that a dameng-normalized
// fingerprint still records a detection check when the detector is disabled.
func TestRunScanSkipsDamengWhenToolUnconfigured(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.0.2.10 -> [5236]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="5236"><state state="open"/><service name="dameng"/></port></ports></host></nmaprun>`),
	}}
	scanStore := newScanStore(t)

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-dameng-unconfigured",
		Targets:        []string{"192.0.2.10"},
		Ports:          "5236",
		DiscoveryMode:  DiscoveryAssumeUp,
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: ""},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	findings, err := scanStore.ListFindings("run-dameng-unconfigured")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings when dameng disabled, got %#v", findings)
	}

	checks, err := scanStore.ListDetectionChecks("run-dameng-unconfigured")
	if err != nil {
		t.Fatal(err)
	}
	checkByEngine := map[string]store.DetectionCheck{}
	for _, c := range checks {
		checkByEngine[c.Engine] = c
	}
	if c, ok := checkByEngine["dameng"]; !ok || c.Status != "skipped" || c.ReasonCode != "tool_unconfigured" {
		t.Fatalf("expected dameng skipped tool_unconfigured, got %#v", checkByEngine["dameng"])
	}
}

// TestRunScanSkipsDamengWhenNoMatchingRule verifies that non-dameng services
// record a skipped dameng detection check with reason no_matching_rule.
func TestRunScanSkipsDamengWhenNoMatchingRule(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.0.2.10 -> [3306]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="3306"><state state="open"/><service name="mysql" product="MySQL"/></port></ports></host></nmaprun>`),
	}}
	scanStore := newScanStore(t)

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-dameng-no-match",
		Targets:        []string{"192.0.2.10"},
		Ports:          "3306",
		DiscoveryMode:  DiscoveryAssumeUp,
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled", Nuclei: "nuclei", NucleiTemplates: "templates"},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	checks, err := scanStore.ListDetectionChecks("run-dameng-no-match")
	if err != nil {
		t.Fatal(err)
	}
	checkByEngine := map[string]store.DetectionCheck{}
	for _, c := range checks {
		checkByEngine[c.Engine] = c
	}
	if c, ok := checkByEngine["dameng"]; !ok || c.Status != "skipped" || c.ReasonCode != "no_matching_rule" {
		t.Fatalf("expected dameng skipped no_matching_rule, got %#v", checkByEngine["dameng"])
	}
}
