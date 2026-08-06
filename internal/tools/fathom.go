package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

// Fathom integration (M4.1).
//
// fathom is a self-contained Rust recon binary (~/DEV/fathom) that folds the
// alive / port / fingerprint stages into a single `fathom scan --json` call and
// additionally runs fingerprint-gated high-risk checks. This package executes
// it and maps its JSONL output onto the existing fingerprint/report shapes.
//
// JSONL schema (confirmed from fathom source + a live run, see
// docs/reports/fathom-m41-report.md for the per-field provenance):
//
//	fingerprint line  -> {"host","port","service","product","version","checks"?}
//	check element     -> {"id","verdict"("vulnerable"|"safe"|"unknown"),"proof"}
//
// Note: the per-port `connections` counter exists in the human-readable text
// output only and is NOT emitted in JSON. fathom probes TCP only, so every
// fingerprint maps to protocol "tcp".

// FathomScanResult bundles the parsed outputs of a single fathom scan: the
// service fingerprints, the fathom-sourced findings (verdict=vulnerable only),
// the detection-check audit trail (one row per fathom check) and the raw
// JSONL for artifact persistence.
type FathomScanResult struct {
	Fingerprints []fingerprint.ServiceFingerprint
	Findings     []report.Finding
	Checks       []report.DetectionCheck
	Output       []byte
}

// RunFathomScan executes `fathom scan --json <ip> -p <ports>` and parses the
// JSONL output into fingerprints, findings and detection checks.
//
// M4.1 calls fathom once per target IP (spec: "每 target 一次 fathom 调用完成
// alive→port→fingerprint"). fathom scan itself accepts comma-separated
// IPs/CIDRs and -l target files; batch/IPv6 handling is deferred to M4.2.
func RunFathomScan(ctx context.Context, runner Runner, binary, ip string, ports []int) (FathomScanResult, error) {
	var result FathomScanResult
	args := []string{"scan", "--json", ip, "-p", joinPorts(ports)}

	out, err := runner.Run(ctx, binary, args)
	result.Output = out
	if err != nil {
		return result, withOutputError(err, out)
	}

	fps, findings, checks, perr := parseFathomJSONL(out, ip)
	if perr != nil {
		return result, perr
	}
	result.Fingerprints = fps
	result.Findings = findings
	result.Checks = checks
	return result, nil
}

// fathomRecord mirrors the JSON fathom emits per open port. Unknown keys (e.g.
// the "discover" object attached under --discover) are ignored by the decoder,
// so the same type parses both discover-on and discover-off output.
type fathomRecord struct {
	Host    string        `json:"host"`
	Port    int           `json:"port"`
	Service string        `json:"service"`
	Product string        `json:"product"`
	Version string        `json:"version"`
	Checks  []fathomCheck `json:"checks,omitempty"`
}

// fathomCheck mirrors a single check object in a fingerprint line's checks
// array. Verdict is one of "vulnerable" | "safe" | "unknown" (src/checks.rs
// Verdict::as_str).
type fathomCheck struct {
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	Proof   string `json:"proof"`
}

func parseFathomJSONL(data []byte, targetIP string) ([]fingerprint.ServiceFingerprint, []report.Finding, []report.DetectionCheck, error) {
	var fps []fingerprint.ServiceFingerprint
	var findings []report.Finding
	var checks []report.DetectionCheck

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || !strings.HasPrefix(line, "{") {
			// Skip blank lines and any non-JSON noise (fathom --json writes one
			// JSON object per line; stderr banner/diagnostic lines, if merged
			// by the runner, do not start with '{').
			continue
		}
		var record fathomRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, nil, nil, fmt.Errorf("invalid fathom JSONL line %q: %w", line, err)
		}

		fp := fingerprint.ServiceFingerprint{
			IP:       record.Host,
			Port:     record.Port,
			Protocol: "tcp",
			Service:  record.Service,
			Product:  record.Product,
			Version:  record.Version,
			// CPE intentionally empty: fathom does not emit CPE (spec decision
			// 3 — degradation accepted). IsWeb/URL are derived by Classify.
		}
		if fp.IP == "" {
			fp.IP = targetIP
		}
		fp = fingerprint.Classify(fp)
		fps = append(fps, fp)

		for _, check := range record.Checks {
			checks = append(checks, report.DetectionCheck{
				IP:         fp.IP,
				Port:       fp.Port,
				Protocol:   fp.Protocol,
				Engine:     "fathom",
				Status:     "completed",
				ReasonCode: check.Verdict, // vulnerable | safe | unknown
				Detail:     check.Proof,
			})
			if check.Verdict != "vulnerable" {
				// safe/unknown are audit-only: no Finding (no compromise proven).
				continue
			}
			findings = append(findings, report.Finding{
				IP:       fp.IP,
				Port:     fp.Port,
				Protocol: fp.Protocol,
				Source:   "fathom",
				ID:       check.ID,
				Severity: fathomSeverity(check.ID),
				Summary:  fmt.Sprintf("fathom %s: %s", check.ID, firstLine(check.Proof)),
				Target:   net.JoinHostPort(fp.IP, strconv.Itoa(fp.Port)),
				Output:   check.Proof,
			})
		}
	}
	return fps, findings, checks, nil
}

func firstLine(value string) string {
	if idx := strings.IndexAny(value, "\r\n"); idx >= 0 {
		return value[:idx]
	}
	return value
}

// fathomCheckSeverity maps a fathom check id to its severity. Every fathom
// check clears the ~/DEV/fathom CHECKS.md admission bar (maximum severity —
// unauthorized access / weak password with a proof of compromise), so the table
// is exhaustive and the fallback stays "high".
var fathomCheckSeverity = map[string]string{
	"redis-unauth":  "high",
	"redis-weak":    "high",
	"mysql-weak":    "high",
	"mssql-weak":    "high",
	"postgres-weak": "high",
	"ssh-weak":      "high",
	"zk-unauth":     "high",
	"mongo-unauth":  "high",
	"es-unauth":     "high",
	"docker-unauth": "high",
}

func fathomSeverity(checkID string) string {
	if sev, ok := fathomCheckSeverity[checkID]; ok {
		return sev
	}
	return "high"
}

// TLSWebCandidatePorts lists common TLS-only web ports (spec decision 2).
//
// fathom's http probe sends a plaintext `GET /` (~/DEV/fathom src/fingerprint.rs
// http()) and therefore cannot complete a TLS handshake on these ports: the
// fingerprint engine gives up and reports service "unknown". When a fathom
// fingerprint is "unknown" on one of these ports it should still be routed to
// httpx for TLS web enhancement.
//
// M4.1 ships the data structure and predicate only (no httpx trigger); the
// trigger is wired in M4.2's scan_target front stage. The root cause — fathom
// not doing a TLS ClientHello probe — is a later fathom milestone.
var TLSWebCandidatePorts = map[int]bool{
	443:  true,
	8443: true,
	9443: true,
	4443: true,
	8843: true,
}

// NeedsTLSWebEnhancement reports whether a fathom fingerprint should be handed
// to httpx for TLS web enhancement. service is the normalized service name
// (fingerprint.ServiceFingerprint.Normalized); fathom emits "unknown" for ports
// it could not identify.
func NeedsTLSWebEnhancement(service string, port int) bool {
	return strings.EqualFold(strings.TrimSpace(service), "unknown") && TLSWebCandidatePorts[port]
}
