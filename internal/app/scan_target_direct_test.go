package app

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
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

func TestScanTargetRecordsNSESkipReasons(t *testing.T) {
	tests := []struct {
		name       string
		serviceXML []byte
		tools      ToolPaths
		rules      map[string][]string
		wantReason string
	}{
		{
			name:       "web service skips matching nse rule",
			serviceXML: []byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`),
			tools:      ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
			rules:      map[string][]string{"http": {"http-title"}},
			wantReason: "no_matching_rule",
		},
		{
			name:       "unconfigured nmap wins over no matching nse rule",
			serviceXML: []byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
			tools:      ToolPaths{Rustscan: "/opt/rustscan"},
			rules:      map[string][]string{"mysql": {"mysql-info"}},
			wantReason: "tool_unconfigured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingSequenceRunner{outputs: [][]byte{
				[]byte("192.168.1.10 -> [80]\\n"),
				tt.serviceXML,
			}}
			var checks []store.DetectionCheck
			opts := ScanOptions{
				RunID:                "run-nse-skip",
				Ports:                "1-65535",
				Tools:                tt.tools,
				NSERules:             tt.rules,
				RecordDetectionCheck: func(check store.DetectionCheck) error { checks = append(checks, check); return nil },
			}

			if _, err := scanTarget(context.Background(), runner, opts, "192.168.1.10", t.TempDir(), &recordingProgress{}); err != nil {
				t.Fatalf("scanTarget returned error: %v", err)
			}
			for _, check := range checks {
				if check.Engine == "nse" {
					if check.Status != "skipped" || check.ReasonCode != tt.wantReason {
						t.Fatalf("NSE check = %#v, want skipped/%s", check, tt.wantReason)
					}
					if len(runner.commands) != 2 {
						t.Fatalf("NSE should not run, commands=%#v", runner.commands)
					}
					return
				}
			}
			t.Fatalf("NSE detection check not recorded: %#v", checks)
		})
	}
}

func TestScanTargetHidesUnmatchedDamengProbe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		// Closing without a response is a deterministic non-Dameng reply.
	}()

	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte(fmt.Sprintf("127.0.0.1 -> [%d]\\n", port)),
		[]byte(fmt.Sprintf(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="%d"><state state="open"/><service name="tcpwrapped"/></port></ports></host></nmaprun>`, port)),
	}}
	progress := &recordingProgress{}

	if _, err := scanTarget(context.Background(), runner, ScanOptions{
		RunID: "run-dameng-probe-miss",
		Ports: "1",
		Tools: ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled"},
	}, "127.0.0.1", t.TempDir(), progress); err != nil {
		t.Fatalf("scanTarget returned error: %v", err)
	}
	for _, event := range progress.events {
		if strings.Contains(event, "dameng-probe") {
			t.Fatalf("unmatched Dameng probe emitted Console event: %q", event)
		}
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Dameng probe did not complete")
	}
}

func TestScanTargetShowsMatchedDamengProbe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 4096)
		if n, _ := conn.Read(buf); n == 0 {
			return
		}
		_, _ = conn.Write([]byte{8, 0, 0, 0, 0, 0, 0, 0})
	}()

	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte(fmt.Sprintf("127.0.0.1 -> [%d]\\n", port)),
		[]byte(fmt.Sprintf(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="%d"><state state="open"/><service name="tcpwrapped"/></port></ports></host></nmaprun>`, port)),
	}}
	progress := &recordingProgress{}

	scan, err := scanTarget(context.Background(), runner, ScanOptions{
		RunID:         "run-dameng-probe-match",
		Ports:         strconv.Itoa(port),
		Tools:         ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Dameng: "enabled"},
		DamengChecker: &fakeDamengAuthChecker{},
	}, "127.0.0.1", t.TempDir(), progress)
	if err != nil {
		t.Fatalf("scanTarget returned error: %v", err)
	}
	if len(scan.Fingerprints) != 1 || scan.Fingerprints[0].Normalized != "dameng" {
		t.Fatalf("fingerprints = %#v, want matched Dameng fingerprint", scan.Fingerprints)
	}
	for _, event := range progress.events {
		if strings.Contains(event, "dameng-probe 127.0.0.1") && strings.Contains(event, "matched") {
			<-done
			return
		}
	}
	t.Fatalf("matched Dameng probe did not emit Console event: %#v", progress.events)
}
