package app

import (
	"context"
	"fmt"
	"testing"
)

// recordingProgress is a no-store Progress used to exercise scanTarget without a
// SQLite database — it records emitted events for assertion. This is the
// testability payoff of stage 2: the per-target pipeline depends on a one-method
// Progress seam, not on *store.Store.
type recordingProgress struct {
	events []string
}

func (r *recordingProgress) Emit(level, stage, format string, args ...any) {
	r.events = append(r.events, fmt.Sprintf("%s/%s %s", level, stage, fmt.Sprintf(format, args...)))
}

// TestScanTargetReturnsFingerprintsAndOpenPorts drives scanTarget directly with a
// fake runner and a recording Progress — no *store.Store involved. It proves the
// per-target pipeline is testable through its narrow interface after stage 2.
func TestScanTargetReturnsFingerprintsAndOpenPorts(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.168.1.10 -> [22]\n"), // rustscan: one open port
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`), // nmap service fingerprint
	}}
	opts := ScanOptions{
		RunID:   "run-direct",
		Targets: []string{"192.168.1.10"},
		Ports:   "22",
		Tools:   ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
	}
	progress := &recordingProgress{}

	ts, err := scanTarget(context.Background(), runner, opts, "192.168.1.10", t.TempDir(), progress)
	if err != nil {
		t.Fatalf("scanTarget returned error: %v", err)
	}

	if ts.Target != "192.168.1.10" {
		t.Errorf("Target = %q, want 192.168.1.10", ts.Target)
	}
	if len(ts.OpenPorts) != 1 || ts.OpenPorts[0] != 22 {
		t.Errorf("OpenPorts = %v, want [22]", ts.OpenPorts)
	}
	if len(ts.Fingerprints) != 1 {
		t.Fatalf("Fingerprints = %d, want 1: %+v", len(ts.Fingerprints), ts.Fingerprints)
	}
	fp := ts.Fingerprints[0]
	if fp.Port != 22 || fp.Service != "ssh" {
		t.Errorf("fingerprint = %+v, want port 22 service ssh", fp)
	}
	if len(ts.Findings) != 0 {
		t.Errorf("Findings = %d, want 0 (no NSE/nuclei rules configured)", len(ts.Findings))
	}
	if len(progress.events) == 0 {
		t.Error("expected progress events to be emitted through the Progress seam")
	}
}

// TestScanTargetSkipsFingerprintWhenNoOpenPorts covers the early-return branch:
// rustscan finds no open ports, so nmap/httpx/NSE/nuclei never run.
func TestScanTargetSkipsFingerprintWhenNoOpenPorts(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.168.1.10 -> [].\n"), // rustscan: no open ports
	}}
	opts := ScanOptions{
		RunID:   "run-empty",
		Targets: []string{"192.168.1.10"},
		Ports:   "22",
		Tools:   ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
	}

	ts, err := scanTarget(context.Background(), runner, opts, "192.168.1.10", t.TempDir(), &recordingProgress{})
	if err != nil {
		t.Fatalf("scanTarget returned error: %v", err)
	}
	if ts.Target != "192.168.1.10" {
		t.Errorf("Target = %q, want 192.168.1.10", ts.Target)
	}
	if len(ts.OpenPorts) != 0 {
		t.Errorf("OpenPorts = %v, want empty", ts.OpenPorts)
	}
	if len(ts.Fingerprints) != 0 {
		t.Errorf("Fingerprints = %d, want 0 (fingerprinting skipped)", len(ts.Fingerprints))
	}
	// Only rustscan should have run.
	if len(runner.commands) != 1 {
		t.Fatalf("tool commands = %d, want 1 (rustscan only): %v", len(runner.commands), runner.commands)
	}
}
