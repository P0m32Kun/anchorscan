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
	HadErrors    bool
}

// scanTarget runs the per-target pipeline (rustscan → nmap → httpx → NSE/nuclei)
// and returns everything it discovered as a TargetScan. It persists durable
// facts through the option seams while retaining them for the JSON report.
func scanTarget(ctx context.Context, runner tools.Runner, opts ScanOptions, target string, artifactDir string, progress Progress) (TargetScan, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	logf(opts, "target %s", target)
	progress.Emit("info", "rustscan", "rustscan %s ports=%s", target, opts.Ports)
	toolCtx, cancel := toolContext(ctx, opts.Timeouts.Rustscan)
	ports, out, err := tools.DiscoverPortsWithOutput(toolCtx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
	normalizedErr := normalizeToolError(toolCtx, err)
	cancel()
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("rustscan", target, "ports")+".txt", out); writeErr != nil {
		return TargetScan{}, writeErr
	}
	if err != nil {
		return TargetScan{}, normalizedErr
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
	toolCtx, cancel = toolContext(ctx, opts.Timeouts.Nmap)
	fingerprints, out, err := tools.FingerprintWithOutput(toolCtx, runner, opts.Tools.Nmap, target, ports, opts.ExtraArgs.Nmap)
	normalizedErr = normalizeToolError(toolCtx, err)
	cancel()
	close(done)
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nmap-service", target)+".xml", out); writeErr != nil {
		return TargetScan{}, writeErr
	}
	if err != nil {
		return TargetScan{}, normalizedErr
	}
	progress.Emit("info", "nmap", "nmap %s services=%d elapsed=%s", target, len(fingerprints), time.Since(started).Round(time.Second))

	result := TargetScan{Target: target, OpenPorts: openPorts}
	for _, fp := range fingerprints {
		if opts.PersistFingerprint != nil {
			if err := opts.PersistFingerprint(fp); err != nil {
				return result, err
			}
		}
		httpResult := tools.HTTPResult{}
		if fp.IsWeb && opts.Tools.Httpx != "" {
			progress.Emit("info", "httpx", "httpx %s", fp.URL)
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Httpx)
			httpResult, out, err = tools.EnrichWebWithOutput(toolCtx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("httpx", fp.IP, strconv.Itoa(fp.Port))+".jsonl", out); writeErr != nil {
				result.HadErrors = true
				progress.Emit("error", "httpx", "httpx %s artifact failed: %v", fp.URL, writeErr)
			}
			if err != nil {
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				progress.Emit("error", "httpx", "httpx %s failed: %v", fp.URL, err)
			}
			if httpResult.URL != "" {
				fp.URL = httpResult.URL
			}
		}
		if opts.PersistFingerprint != nil {
			if err := opts.PersistFingerprint(fp); err != nil {
				return result, err
			}
		}

		allFingerprints = append(allFingerprints, fp)

		for _, finding := range ManualReviewFindings(fp) {
			if err := persistFinding(opts, finding); err != nil {
				return result, err
			}
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
			stageFailed := false
			if err := recordDetectionCheck(opts, fp, "nse", "running", "", "", started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.NSE)
			nseResults, out, err := tools.RunNSEWithOutput(toolCtx, runner, opts.Tools.Nmap, fp.IP, fp.Port, scripts, opts.ExtraArgs.Nmap)
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nse", fp.IP, strconv.Itoa(fp.Port), strings.Join(scripts, ","))+".xml", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "nse", "failed", "artifact_failed", writeErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nse", "nse %s:%d artifact failed: %v", fp.IP, fp.Port, writeErr)
			}
			if err != nil {
				status, reason := "failed", "command_failed"
				if operatorCanceled {
					status, reason = "canceled", "run_canceled"
				}
				_ = recordDetectionCheck(opts, fp, "nse", status, reason, err.Error(), started, time.Now())
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nse", "nse %s:%d failed: %v", fp.IP, fp.Port, err)
			}
			if err == nil {
				for _, check := range nseResults {
					finding := report.Finding{IP: fp.IP, Port: fp.Port, Protocol: fp.Protocol, Source: "nse", ID: check.ID, Severity: "info", Summary: check.ID, Target: fp.IP, Output: check.Output}
					if err := persistFinding(opts, finding); err != nil {
						_ = recordDetectionCheck(opts, fp, "nse", "failed", "persistence_failed", err.Error(), started, time.Now())
						return result, err
					}
					allFindings = append(allFindings, finding)
				}
			}
			if !stageFailed {
				if err := recordDetectionCheck(opts, fp, "nse", "completed", "", "", started, time.Now()); err != nil {
					return result, err
				}
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
			stageFailed := false
			if err := recordDetectionCheck(opts, fp, "nuclei", "running", "", "", started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Nuclei)
			out, err := tools.RunNuclei(toolCtx, runner, opts.Tools.Nuclei, match.Address, match.Tags, match.ExcludeTags, opts.ExtraArgs.Nuclei)
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nuclei", fp.IP, strconv.Itoa(fp.Port), strings.Join(match.Tags, ","))+".jsonl", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "artifact_failed", writeErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nuclei", "nuclei %s artifact failed: %v", match.Address, writeErr)
			}
			if err != nil {
				status, reason := "failed", "command_failed"
				if operatorCanceled {
					status, reason = "canceled", "run_canceled"
				}
				_ = recordDetectionCheck(opts, fp, "nuclei", status, reason, err.Error(), started, time.Now())
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nuclei", "nuclei %s failed: %v", match.Address, err)
			}
			nucleiFindings, parseErr := tools.ParseNucleiJSONL(out)
			if err == nil && parseErr != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "invalid_output", parseErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nuclei", "nuclei %s returned invalid output: %v", match.Address, parseErr)
			}
			if err == nil && parseErr == nil {
				for _, nucleiResult := range nucleiFindings {
					finding := findingFromNuclei(nucleiResult, fp, allFingerprints)
					if err := persistFinding(opts, finding); err != nil {
						_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "persistence_failed", err.Error(), started, time.Now())
						return result, err
					}
					allFindings = append(allFindings, finding)
				}
			}
			if !stageFailed {
				if err := recordDetectionCheck(opts, fp, "nuclei", "completed", "", "", started, time.Now()); err != nil {
					return TargetScan{}, err
				}
			}
		}
	}

	return TargetScan{Target: target, Fingerprints: allFingerprints, Findings: allFindings, OpenPorts: openPorts, HadErrors: result.HadErrors}, nil
}

func recordDetectionCheck(opts ScanOptions, fp fingerprint.ServiceFingerprint, engine, status, reasonCode, detail string, startedAt, finishedAt time.Time) error {
	if opts.RecordDetectionCheck == nil || opts.RunID == "" {
		return nil
	}
	return opts.RecordDetectionCheck(store.DetectionCheck{RunID: opts.RunID, IP: fp.IP, Port: fp.Port, Protocol: fp.Protocol, Engine: engine, Status: status, ReasonCode: reasonCode, Detail: detail, StartedAt: startedAt, FinishedAt: finishedAt})
}

func persistFinding(opts ScanOptions, finding report.Finding) error {
	if opts.PersistFinding == nil || opts.RunID == "" {
		return nil
	}
	return opts.PersistFinding(finding)
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
