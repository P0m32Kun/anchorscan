package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// fakeDamengAuthChecker implements the DamengAuthChecker interface for scan-level tests.
type fakeDamengAuthChecker struct {
	ok  bool
	out string
	err error
}

func (f *fakeDamengAuthChecker) Check(ctx context.Context, host string, port int, username, password string) (bool, string, error) {
	return f.ok, f.out, f.err
}

// TestRunScanTriggersDamengFinding verifies that when a fingerprint is already
// normalized to "dameng" and the dameng detector is enabled, the dameng engine
// runs, records a completed detection check, and persists a high-severity
// finding for the default password.
func TestRunScanTriggersDamengFinding(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.0.2.10 -> [5236]\n"),
		[]byte(`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="5236"><state state="open"/><service name="dameng" product="Dameng DB"/></port></ports></host></nmaprun>`),
	}}
	scanStore := newScanStore(t)

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-dameng-vulnerable",
		Targets:        []string{"192.0.2.10"},
		Ports:          "5236",
		DiscoveryMode:  DiscoveryAssumeUp,
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled"},
		DamengChecker:  &fakeDamengAuthChecker{ok: true, out: "default password accepted"},
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
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled"},
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
