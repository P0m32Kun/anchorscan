package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

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

// TestScanTargetEmitsFathomFindingEvent locks the live-terminal hit line for
// fathom findings: a vulnerable check must surface as an info/fathom progress
// event carrying an uppercase hit marker (REDIS-UNAUTH) plus the original
// check id, so the frontend's per-line green hit coloring can pick it up.
func TestScanTargetEmitsFathomFindingEvent(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte(`{"host":"127.0.0.1","port":6379,"service":"redis","product":"Redis","version":"7.4.9","checks":[{"id":"redis-unauth","verdict":"vulnerable","proof":"redis_version:7.4.9"}]}` + "\n"),
	}}
	progress := &recordingProgress{}
	if _, err := scanTarget(context.Background(), runner, ScanOptions{
		RunID: "run-fathom-finding",
		Ports: "6379",
		Tools: ToolPaths{Fathom: "/opt/fathom"},
	}, "127.0.0.1", t.TempDir(), progress); err != nil {
		t.Fatalf("scanTarget returned error: %v", err)
	}
	for _, event := range progress.events {
		if strings.Contains(event, "REDIS-UNAUTH") && strings.Contains(event, "fathom redis-unauth") {
			return
		}
	}
	t.Fatalf("expected a fathom finding progress event, got %#v", progress.events)
}

// TestScanTargetReturnsFingerprintsAndOpenPorts drives scanTarget directly with a
// fake runner and a recording Progress — no *store.Store involved. It proves the
// per-target pipeline is testable through its narrow interface after stage 2.
func TestScanTargetReturnsFingerprintsAndOpenPorts(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		fathomJSONL("192.168.1.10", 22, "ssh", "OpenSSH", ""), // fathom: one open port
	}}
	opts := ScanOptions{
		RunID:   "run-direct",
		Targets: []string{"192.168.1.10"},
		Ports:   "22",
		Tools:   ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
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
// fathom finds no open ports (empty output), so httpx/NSE/nuclei never run.
func TestScanTargetSkipsFingerprintWhenNoOpenPorts(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		nil, // fathom: no open ports
	}}
	opts := ScanOptions{
		RunID:   "run-empty",
		Targets: []string{"192.168.1.10"},
		Ports:   "22",
		Tools:   ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
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
	// Only fathom should have run.
	if len(runner.commands) != 1 {
		t.Fatalf("tool commands = %d, want 1 (fathom only): %v", len(runner.commands), runner.commands)
	}
}

func TestScanTargetRecordsNSESkipReasons(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint []byte
		tools       ToolPaths
		rules       map[string][]string
		wantReason  string
	}{
		{
			name:        "web service skips matching nse rule",
			fingerprint: fathomJSONL("192.168.1.10", 80, "http", "nginx", ""),
			tools:       ToolPaths{Fathom: "/opt/fathom", Nmap: "/opt/nmap"},
			rules:       map[string][]string{"http": {"http-title"}},
			wantReason:  "no_matching_rule",
		},
		{
			name:        "unconfigured nmap wins over no matching nse rule",
			fingerprint: fathomJSONL("192.168.1.10", 22, "ssh", "OpenSSH", ""),
			tools:       ToolPaths{Fathom: "/opt/fathom"},
			rules:       map[string][]string{"mysql": {"mysql-info"}},
			wantReason:  "tool_unconfigured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingSequenceRunner{outputs: [][]byte{
				tt.fingerprint,
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
					if len(runner.commands) != 1 {
						t.Fatalf("NSE should not run, commands=%#v", runner.commands)
					}
					return
				}
			}
			t.Fatalf("NSE detection check not recorded: %#v", checks)
		})
	}
}

func TestScanTargetSkipsNucleiDamengIdentifyForIdentifiedService(t *testing.T) {
	// Regression: a port fathom identified by protocol handshake (redis here)
	// cannot be Dameng, so the nuclei dameng-detect round trip must not run.
	// Previously every non-web port (including redis/mysql/ssh) triggered
	// dameng-identify, wasting a nuclei invocation per identified service.
	runner := &recordingSequenceRunner{outputs: [][]byte{
		fathomJSONL("192.0.2.10", 6379, "redis", "Redis", "7.4.9"),
	}}
	checker := &fakeDamengAuthChecker{}
	_, err := scanTarget(context.Background(), runner, ScanOptions{
		RunID:         "run-redis-no-dameng",
		Ports:         "6379",
		Tools:         ToolPaths{Fathom: "fathom", Nmap: "nmap", Nuclei: "nuclei", NucleiTemplates: "templates", Dameng: "enabled"},
		DamengChecker: checker,
	}, "192.0.2.10", t.TempDir(), &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if checker.calls != 0 {
		t.Fatalf("Dameng authentication calls = %d, want 0 (redis is not a Dameng candidate)", checker.calls)
	}
	for _, cmd := range runner.commands {
		if len(cmd) > 0 && cmd[0] == "nuclei" {
			t.Fatalf("nuclei dameng-identify invoked for identified redis service: %#v", cmd)
		}
	}
}

func TestScanTargetDamengNucleiGate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		nucleiOut []byte
		wantCalls int
	}{
		{name: "no template match", wantCalls: 0},
		{name: "dameng template match on custom port", nucleiOut: []byte(`{"template-id":"dameng-detect","ip":"192.0.2.10","port":"10198"}`), wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recordingSequenceRunner{outputs: [][]byte{
				fathomJSONL("192.0.2.10", 10198, "unknown", "", ""),
				tc.nucleiOut,
			}}
			checker := &fakeDamengAuthChecker{}
			_, err := scanTarget(context.Background(), runner, ScanOptions{
				RunID:         "run-dameng-gate",
				Ports:         "10198",
				Tools:         ToolPaths{Fathom: "fathom", Nmap: "nmap", Nuclei: "nuclei", NucleiTemplates: "templates", Dameng: "enabled"},
				DamengChecker: checker,
			}, "192.0.2.10", t.TempDir(), &recordingProgress{})
			if err != nil {
				t.Fatal(err)
			}
			if checker.calls != tc.wantCalls {
				t.Fatalf("Dameng authentication calls = %d, want %d", checker.calls, tc.wantCalls)
			}
		})
	}
}
