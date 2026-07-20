package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseRdpscanOutputVulnerable(t *testing.T) {
	out := []byte("192.168.1.10:3389 - VULNERABLE\n")
	if got := ParseRdpscanOutput(out); got != RdpscanVulnerable {
		t.Fatalf("ParseRdpscanOutput = %q, want %q", got, RdpscanVulnerable)
	}
}

func TestParseRdpscanOutputSafe(t *testing.T) {
	out := []byte("192.168.1.10:3389 - SAFE - target appears patched\n")
	if got := ParseRdpscanOutput(out); got != RdpscanSafe {
		t.Fatalf("ParseRdpscanOutput = %q, want %q", got, RdpscanSafe)
	}
}

func TestParseRdpscanOutputUnknown(t *testing.T) {
	cases := [][]byte{
		[]byte("192.168.1.10:3389 - UNKNOWN - connection reset by peer\n"),
		[]byte(""),
		[]byte("rdpscan: error: target unreachable\n"),
	}
	for _, out := range cases {
		if got := ParseRdpscanOutput(out); got != RdpscanUnknown {
			t.Fatalf("ParseRdpscanOutput(%q) = %q, want %q", out, got, RdpscanUnknown)
		}
	}
}

// VULNERABLE wins when mixed with SAFE lines: a multi-line output mentioning
// both must never be downgraded.
func TestParseRdpscanOutputVulnerableWinsOverSafe(t *testing.T) {
	out := []byte("192.168.1.10:3389 - SAFE - checking\n192.168.1.10:3389 - VULNERABLE\n")
	if got := ParseRdpscanOutput(out); got != RdpscanVulnerable {
		t.Fatalf("ParseRdpscanOutput = %q, want %q", got, RdpscanVulnerable)
	}
}

func TestRunRdpscanPassesEndpointArgument(t *testing.T) {
	runner := &fakeRunner{output: []byte("192.168.1.10:3389 - SAFE\n")}
	if _, err := RunRdpscan(context.Background(), runner, "/opt/rdpscan", "192.168.1.10", 3389); err != nil {
		t.Fatalf("RunRdpscan returned error: %v", err)
	}
	want := "/opt/rdpscan 192.168.1.10:3389"
	if got := strings.Join(runner.args, " "); got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestRunRdpscanWrapsErrorWithOutput(t *testing.T) {
	runner := &fakeRunner{output: []byte("boom"), err: errors.New("exit status 1")}
	out, err := RunRdpscan(context.Background(), runner, "/opt/rdpscan", "192.168.1.10", 3389)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want wrapped error containing output", err)
	}
	if string(out) != "boom" {
		t.Fatalf("out = %q, want raw output preserved", out)
	}
}
