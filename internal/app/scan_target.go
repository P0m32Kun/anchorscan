package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

var nmapHeartbeatEvery = 30 * time.Second

// TargetScan is the result bundle produced by scanning a single Target: the
// service fingerprints, derived findings, and discovered open ports. It is the
// named shape behind what scanTarget returns (previously a positional 4-tuple).
type TargetScan struct {
	Target       string
	Fingerprints []fingerprint.ServiceFingerprint
	Findings     []report.Finding
	OpenPorts    []int
}

// scanTarget runs the per-target pipeline (rustscan → nmap → httpx → NSE/nuclei)
// and returns everything it discovered as a TargetScan. It is a pure pipeline:
// it does not persist results or write progress events itself — it reports
// progress through the Progress seam, and the fan-out (scanTargets) owns
// persisting the returned TargetScan to the store.
func scanTarget(ctx context.Context, runner tools.Runner, opts ScanOptions, target string, artifactDir string, progress Progress) (TargetScan, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	logf(opts, "target %s", target)
	progress.Emit("info", "rustscan", "rustscan %s ports=%s", target, opts.Ports)
	ports, out, err := tools.DiscoverPortsWithOutput(ctx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("rustscan", target, "ports")+".txt", out); writeErr != nil {
		return TargetScan{}, writeErr
	}
	if err != nil {
		return TargetScan{}, normalizeToolError(ctx, err)
	}
	progress.Emit("info", "rustscan", "rustscan %s open=%v", target, ports)
	openPorts := append([]int(nil), ports...)
	if len(ports) == 0 {
		progress.Emit("info", "target", "target %s has no open ports; skip fingerprint and vulnerability checks", target)
		return TargetScan{Target: target, OpenPorts: openPorts}, nil
	}

	progress.Emit("info", "nmap", "nmap %s ports=%v (service detection may be slow)", target, ports)
	started := time.Now()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(nmapHeartbeatEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logf(opts, "nmap %s still running elapsed=%s", target, time.Since(started).Round(time.Second))
			case <-done:
				return
			}
		}
	}()
	fingerprints, out, err := tools.FingerprintWithOutput(ctx, runner, opts.Tools.Nmap, target, ports, opts.ExtraArgs.Nmap)
	close(done)
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nmap-service", target)+".xml", out); writeErr != nil {
		return TargetScan{}, writeErr
	}
	if err != nil {
		return TargetScan{}, normalizeToolError(ctx, err)
	}
	progress.Emit("info", "nmap", "nmap %s services=%d elapsed=%s", target, len(fingerprints), time.Since(started).Round(time.Second))

	for _, fp := range fingerprints {
		httpResult := tools.HTTPResult{}
		if fp.IsWeb && opts.Tools.Httpx != "" {
			progress.Emit("info", "httpx", "httpx %s", fp.URL)
			httpResult, out, err = tools.EnrichWebWithOutput(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("httpx", fp.IP, strconv.Itoa(fp.Port))+".jsonl", out); writeErr != nil {
				return TargetScan{}, writeErr
			}
			if err != nil {
				return TargetScan{}, normalizeToolError(ctx, err)
			}
			if httpResult.URL != "" {
				fp.URL = httpResult.URL
			}
		}

		allFingerprints = append(allFingerprints, fp)

		for _, finding := range ManualReviewFindings(fp) {
			allFindings = append(allFindings, finding)
		}

		scripts := vuln.MatchNSE(fp, opts.NSERules)
		switch {
		case len(scripts) == 0:
			if err := recordDetectionCheck(opts, fp, "nse", "skipped", "no_matching_rule", "", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case fp.IsWeb:
			if err := recordDetectionCheck(opts, fp, "nse", "skipped", "not_applicable", "NSE checks are not run for web services", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case opts.Tools.Nmap == "":
			if err := recordDetectionCheck(opts, fp, "nse", "skipped", "tool_unconfigured", "nmap is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		default:
			progress.Emit("info", "nse", "nse %s:%d scripts=%v", fp.IP, fp.Port, scripts)
			started := time.Now()
			if err := recordDetectionCheck(opts, fp, "nse", "running", "", "", started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			nseResults, out, err := tools.RunNSEWithOutput(ctx, runner, opts.Tools.Nmap, fp.IP, fp.Port, scripts, opts.ExtraArgs.Nmap)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nse", fp.IP, strconv.Itoa(fp.Port), strings.Join(scripts, ","))+".xml", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "nse", "failed", "artifact_failed", writeErr.Error(), started, time.Now())
				return TargetScan{}, writeErr
			}
			if err != nil {
				_ = recordDetectionCheck(opts, fp, "nse", detectionCheckFailureStatus(ctx), "command_failed", err.Error(), started, time.Now())
				return TargetScan{}, normalizeToolError(ctx, err)
			}
			if err := recordDetectionCheck(opts, fp, "nse", "completed", "", "", started, time.Now()); err != nil {
				return TargetScan{}, err
			}
			for _, result := range nseResults {
				finding := report.Finding{
					IP:       fp.IP,
					Port:     fp.Port,
					Protocol: fp.Protocol,
					Source:   "nse",
					ID:       result.ID,
					Severity: "info",
					Summary:  result.ID,
					Target:   fp.IP,
					Output:   result.Output,
				}
				allFindings = append(allFindings, finding)
			}
		}

		match := vuln.MatchNucleiTags(fp, vuln.HTTPResult{URL: fp.URL, Tech: httpResult.Tech}, opts.TagRules)
		switch {
		case len(match.Tags) == 0:
			if err := recordDetectionCheck(opts, fp, "nuclei", "skipped", "no_matching_rule", "", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case match.Address == "":
			if err := recordDetectionCheck(opts, fp, "nuclei", "skipped", "missing_target", "nuclei target is empty", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case opts.Tools.Nuclei == "":
			if err := recordDetectionCheck(opts, fp, "nuclei", "skipped", "tool_unconfigured", "nuclei is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		default:
			progress.Emit("info", "nuclei", "nuclei %s tags=%v", match.Address, match.Tags)
			started := time.Now()
			if err := recordDetectionCheck(opts, fp, "nuclei", "running", "", "", started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			out, err := tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, match.Address, match.Tags, match.ExcludeTags, opts.ExtraArgs.Nuclei)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nuclei", fp.IP, strconv.Itoa(fp.Port), strings.Join(match.Tags, ","))+".jsonl", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "artifact_failed", writeErr.Error(), started, time.Now())
				return TargetScan{}, writeErr
			}
			if err != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", detectionCheckFailureStatus(ctx), "command_failed", err.Error(), started, time.Now())
				return TargetScan{}, normalizeToolError(ctx, err)
			}
			nucleiFindings, err := tools.ParseNucleiJSONL(out)
			if err != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "invalid_output", err.Error(), started, time.Now())
				return TargetScan{}, err
			}
			if err := recordDetectionCheck(opts, fp, "nuclei", "completed", "", "", started, time.Now()); err != nil {
				return TargetScan{}, err
			}
			for _, result := range nucleiFindings {
				finding := findingFromNuclei(result, fp, allFingerprints)
				allFindings = append(allFindings, finding)
			}
		}
	}

	return TargetScan{Target: target, Fingerprints: allFingerprints, Findings: allFindings, OpenPorts: openPorts}, nil
}

func recordDetectionCheck(opts ScanOptions, fp fingerprint.ServiceFingerprint, engine, status, reasonCode, detail string, startedAt, finishedAt time.Time) error {
	if opts.RecordDetectionCheck == nil || opts.RunID == "" {
		return nil
	}
	return opts.RecordDetectionCheck(store.DetectionCheck{RunID: opts.RunID, IP: fp.IP, Port: fp.Port, Protocol: fp.Protocol, Engine: engine, Status: status, ReasonCode: reasonCode, Detail: detail, StartedAt: startedAt, FinishedAt: finishedAt})
}

func detectionCheckFailureStatus(ctx context.Context) string {
	if ctx.Err() != nil {
		return "canceled"
	}
	return "failed"
}

func formatNucleiEvidence(result tools.NucleiFinding) string {
	var lines []string
	if result.MatchedAt != "" {
		lines = append(lines, "matched-at: "+result.MatchedAt)
	}
	if result.MatcherName != "" {
		lines = append(lines, "matcher-name: "+result.MatcherName)
	}
	if len(result.ExtractedResults) > 0 {
		lines = append(lines, "extracted-results: "+strings.Join(result.ExtractedResults, ", "))
	}
	if result.CurlCommand != "" {
		lines = append(lines, "curl-command: "+result.CurlCommand)
	}
	if result.Raw != "" {
		lines = append(lines, "", result.Raw)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func findingFromNuclei(result tools.NucleiFinding, fallback fingerprint.ServiceFingerprint, fingerprints []fingerprint.ServiceFingerprint) report.Finding {
	ip, port := result.Endpoint(fallback.IP, fallback.Port)
	protocol := fallback.Protocol
	for _, fp := range fingerprints {
		if fp.IP == ip && fp.Port == port {
			protocol = fp.Protocol
			break
		}
	}
	return report.Finding{
		IP:       ip,
		Port:     port,
		Protocol: protocol,
		Source:   "nuclei",
		ID:       result.TemplateID,
		Severity: result.Severity,
		Summary:  result.Name,
		Target:   result.MatchedAt,
		Output:   formatNucleiEvidence(result),
	}
}
