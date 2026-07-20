package tools

import (
	"context"
	"strconv"
	"strings"
)

// RdpscanVerdict is the three-state outcome of an rdpscan run against one RDP
// endpoint. rdpscan (robertdavidgraham/rdpscan) implements the safe
// CVE-2019-0708 (BlueKeep) check derived from zerosum0x0's research — the same
// detection logic as Metasploit's auxiliary scanner — as a single-purpose
// binary, which keeps the RDP protocol stack out of anchorscan itself.
type RdpscanVerdict string

const (
	RdpscanVulnerable RdpscanVerdict = "vulnerable"
	RdpscanSafe       RdpscanVerdict = "safe"
	RdpscanUnknown    RdpscanVerdict = "unknown"
)

// RunRdpscan executes rdpscan against a single ip:port and returns its raw
// output. The caller persists the output as an artifact and classifies it
// with ParseRdpscanOutput.
func RunRdpscan(ctx context.Context, runner Runner, binaryPath string, ip string, port int) ([]byte, error) {
	target := ip + ":" + strconv.Itoa(port)
	out, err := runner.Run(ctx, binaryPath, []string{target})
	if err != nil {
		return out, withOutputError(err, out)
	}
	return out, nil
}

// ParseRdpscanOutput classifies rdpscan's human-readable output. Any
// VULNERABLE line wins; otherwise a SAFE line means the target answered the
// handshake without vulnerable indicators; anything else (connection errors,
// NLA-only targets, garbage) is unknown. Unknown must never be treated as a
// negative result — it carries no verdict.
//
// ponytail: substring match over "contains the status token" instead of a
// strict line grammar. rdpscan's format is stable since 2019; if a future
// version rephrases statuses, the worst case is a silent unknown — acceptable
// for an optional engine.
func ParseRdpscanOutput(input []byte) RdpscanVerdict {
	hasSafe := false
	for _, line := range strings.Split(string(input), "\n") {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "VULNERABLE") {
			return RdpscanVulnerable
		}
		if strings.Contains(upper, "SAFE") {
			hasSafe = true
		}
	}
	if hasSafe {
		return RdpscanSafe
	}
	return RdpscanUnknown
}
