package app

import (
	"context"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/target"
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
				progress.Emit("info", "heartbeat", "nmap %s still running elapsed=%s", target, time.Since(started).Round(time.Second))
			case <-done:
				return
			}
		}
	}()
	toolCtx, cancel = toolContext(ctx, opts.Timeouts.Nmap)
	fingerprints, out, err := tools.FingerprintWithOutput(toolCtx, runner, opts.Tools.Nmap, target, ports, opts.ExtraArgs.Nmap)
	if !opts.Scope.IsZero() {
		fingerprints = filterScopeFingerprints(opts.Scope, fingerprints)
	}
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
			if httpResult.URL != "" && (opts.Scope.IsZero() || scopeAllowsURL(opts.Scope, httpResult.URL)) {
				fp.URL = httpResult.URL
			}
		}
		if opts.PersistFingerprint != nil {
			if err := opts.PersistFingerprint(fp); err != nil {
				return result, err
			}
		}

		// Nuclei is the sole Dameng protocol authority. Nmap labels and ports
		// are candidates only and never authorize a credential attempt.
		damengMatched := false
		var damengIdentifyErr error
		if !fp.IsWeb && opts.Tools.Dameng != "" && opts.Tools.Nuclei != "" {
			started := time.Now()
			progress.Emit("info", "dameng-identify", "dameng-identify %s:%d template=dameng-detect", fp.IP, fp.Port)
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Dameng)
			out, err := tools.RunNucleiTemplate(toolCtx, runner, opts.Tools.Nuclei, net.JoinHostPort(fp.IP, strconv.Itoa(fp.Port)), opts.Tools.DamengTemplatePath(), nil)
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("dameng-identify", fp.IP, strconv.Itoa(fp.Port))+".jsonl", out); writeErr != nil {
				damengIdentifyErr = writeErr
			} else if err != nil {
				damengIdentifyErr = err
				if operatorCanceled {
					return result, context.Canceled
				}
			} else if findings, parseErr := tools.ParseNucleiJSONL(out); parseErr != nil {
				damengIdentifyErr = parseErr
			} else {
				for _, finding := range findings {
					if finding.TemplateID == "dameng-detect" {
						damengMatched = true
						break
					}
				}
			}
			if damengIdentifyErr != nil {
				progress.Emit("error", "dameng-identify", "dameng-identify %s:%d failed after %s: %v", fp.IP, fp.Port, time.Since(started).Round(time.Second), damengIdentifyErr)
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
		case opts.Tools.Nmap == "":
			if err := recordDetectionCheck(opts, fp, "nse", "skipped", "tool_unconfigured", "nmap is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case fp.IsWeb:
			if err := recordDetectionCheck(opts, fp, "nse", "skipped", "no_matching_rule", "NSE checks are not run for web services", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case len(scripts) == 0:
			if err := recordDetectionCheck(opts, fp, "nse", "skipped", "no_matching_rule", "", time.Now(), time.Now()); err != nil {
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
		// MatchNucleiTags returns a zero-value MatchResult (empty Address) when no
		// rule matches. Address remains the match sentinel because tags may be
		// intentionally empty in a user-provided rule.
		case match.Address == "":
			if err := recordDetectionCheck(opts, fp, "nuclei", "skipped", "no_matching_rule", "", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case opts.Tools.Nuclei == "":
			if err := recordDetectionCheck(opts, fp, "nuclei", "skipped", "tool_unconfigured", "tags="+strings.Join(match.Tags, ",")+": nuclei is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		// NSE and nuclei DetectionCheck tails are kept separate on purpose:
		// nuclei has an extra ParseNucleiJSONL + invalid_output stage that NSE lacks,
		// so a shared helper would parameterize (and blur) the state machine.
		default:
			started := time.Now()
			stageFailed := false
			nucleiDetail := "tags=" + strings.Join(match.Tags, ",")
			if err := recordDetectionCheck(opts, fp, "nuclei", "running", "", nucleiDetail, started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Nuclei)
			var out []byte
			var err error
			var artifactKey string
			progress.Emit("info", "nuclei", "nuclei %s tags=%v", match.Address, match.Tags)
			out, err = tools.RunNuclei(toolCtx, runner, opts.Tools.Nuclei, match.Address, match.Tags, match.ExcludeTags, opts.ExtraArgs.Nuclei)
			artifactKey = strings.Join(match.Tags, ",")
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nuclei", fp.IP, strconv.Itoa(fp.Port), artifactKey)+".jsonl", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "artifact_failed", nucleiDetail+": "+writeErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nuclei", "nuclei %s artifact failed: %v", match.Address, writeErr)
			}
			if err != nil {
				status, reason := "failed", "command_failed"
				if operatorCanceled {
					status, reason = "canceled", "run_canceled"
				}
				_ = recordDetectionCheck(opts, fp, "nuclei", status, reason, nucleiDetail+": "+err.Error(), started, time.Now())
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nuclei", "nuclei %s failed: %v", match.Address, err)
			}
			nucleiFindings, parseErr := tools.ParseNucleiJSONL(out)
			if err == nil && parseErr != nil {
				_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "invalid_output", nucleiDetail+": "+parseErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "nuclei", "nuclei %s returned invalid output: %v", match.Address, parseErr)
			}
			if err == nil && parseErr == nil {
				for _, nucleiResult := range nucleiFindings {
					finding := findingFromNuclei(nucleiResult, fp, allFingerprints)
					if !scopeAllowsNucleiFinding(opts.Scope, nucleiResult, finding) {
						progress.Emit("warning", "nuclei", "discarded out-of-scope finding for %s", finding.IP)
						continue
					}
					if err := persistFinding(opts, finding); err != nil {
						_ = recordDetectionCheck(opts, fp, "nuclei", "failed", "persistence_failed", nucleiDetail+": "+err.Error(), started, time.Now())
						return result, err
					}
					allFindings = append(allFindings, finding)
				}
			}
			if !stageFailed {
				if err := recordDetectionCheck(opts, fp, "nuclei", "completed", "", nucleiDetail, started, time.Now()); err != nil {
					return TargetScan{}, err
				}
			}
		}

		// rdpscan (BlueKeep, CVE-2019-0708) — optional third engine. It mirrors the
		// nuclei DetectionCheck tail shape but with a three-state verdict instead of
		// template findings. Execution only happens when the service fingerprint is
		// normalized to "rdp"; port 3389 is not enough on its own, so non-standard
		// RDP ports are also covered.
		switch {
		case fp.Normalized != "rdp":
			if err := recordDetectionCheck(opts, fp, "rdpscan", "skipped", "no_matching_rule", "", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case opts.Tools.Rdpscan == "":
			if err := recordDetectionCheck(opts, fp, "rdpscan", "skipped", "tool_unconfigured", "rdpscan is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		default:
			progress.Emit("info", "rdpscan", "rdpscan %s:%d (CVE-2019-0708 active probe; extremely low risk of target BSOD)", fp.IP, fp.Port)
			started := time.Now()
			stageFailed := false
			if err := recordDetectionCheck(opts, fp, "rdpscan", "running", "", "", started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Rdpscan)
			out, err := tools.RunRdpscan(toolCtx, runner, opts.Tools.Rdpscan, fp.IP, fp.Port)
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("rdpscan", fp.IP, strconv.Itoa(fp.Port))+".txt", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "rdpscan", "failed", "artifact_failed", writeErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "rdpscan", "rdpscan %s:%d artifact failed: %v", fp.IP, fp.Port, writeErr)
			}
			if err != nil {
				status, reason := "failed", "command_failed"
				if operatorCanceled {
					status, reason = "canceled", "run_canceled"
				}
				_ = recordDetectionCheck(opts, fp, "rdpscan", status, reason, err.Error(), started, time.Now())
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "rdpscan", "rdpscan %s:%d failed: %v", fp.IP, fp.Port, err)
			}
			if err == nil {
				verdict := tools.ParseRdpscanOutput(out)
				switch verdict {
				case tools.RdpscanVulnerable:
					finding := report.Finding{
						IP:       fp.IP,
						Port:     fp.Port,
						Protocol: fp.Protocol,
						Source:   "rdpscan",
						ID:       "CVE-2019-0708",
						Severity: "critical",
						Summary:  "Microsoft Remote Desktop Services RCE (BlueKeep, CVE-2019-0708)",
						Target:   fp.IP,
						Output:   strings.TrimSpace(string(out)),
					}
					if err := persistFinding(opts, finding); err != nil {
						_ = recordDetectionCheck(opts, fp, "rdpscan", "failed", "persistence_failed", err.Error(), started, time.Now())
						return result, err
					}
					allFindings = append(allFindings, finding)
					progress.Emit("info", "rdpscan", "rdpscan %s:%d VULNERABLE CVE-2019-0708", fp.IP, fp.Port)
				case tools.RdpscanSafe:
					progress.Emit("info", "rdpscan", "rdpscan %s:%d SAFE", fp.IP, fp.Port)
				default:
					progress.Emit("info", "rdpscan", "rdpscan %s:%d UNKNOWN (no conclusion, see artifact)", fp.IP, fp.Port)
				}
			}
			if !stageFailed {
				if err := recordDetectionCheck(opts, fp, "rdpscan", "completed", "", "", started, time.Now()); err != nil {
					return TargetScan{}, err
				}
			}
		}

		// Default credentials are attempted only after the configured community
		// Nuclei template produced a dameng-detect finding for this endpoint.
		switch {
		case opts.Tools.Dameng == "":
			if err := recordDetectionCheck(opts, fp, "dameng", "skipped", "tool_unconfigured", "dameng detector is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case opts.Tools.Nuclei == "" || strings.TrimSpace(opts.Tools.NucleiTemplates) == "":
			if err := recordDetectionCheck(opts, fp, "dameng", "skipped", "tool_unconfigured", "nuclei or nuclei_templates is not configured", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		case damengIdentifyErr != nil:
			if err := recordDetectionCheck(opts, fp, "dameng", "failed", "command_failed", damengIdentifyErr.Error(), time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
			result.HadErrors = true
		case !damengMatched:
			if err := recordDetectionCheck(opts, fp, "dameng", "skipped", "no_matching_rule", "dameng-detect template did not match", time.Now(), time.Now()); err != nil {
				return TargetScan{}, err
			}
		default:
			progress.Emit("info", "dameng", "dameng %s:%d default password check", fp.IP, fp.Port)
			started := time.Now()
			stageFailed := false
			if err := recordDetectionCheck(opts, fp, "dameng", "running", "", "", started, time.Time{}); err != nil {
				return TargetScan{}, err
			}
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Dameng)
			checker := opts.DamengChecker
			var damengResult tools.DamengResult
			if checker == nil {
				executable, executableErr := os.Executable()
				if executableErr != nil {
					err = executableErr
				} else {
					damengResult, err = tools.RunDamengHelper(toolCtx, runner, executable, fp.IP, fp.Port)
				}
			} else {
				damengResult, err = tools.RunDamengDefaultPassword(toolCtx, checker, fp.IP, fp.Port)
			}
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			out := []byte(damengResult.Output)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("dameng", fp.IP, strconv.Itoa(fp.Port))+".txt", out); writeErr != nil {
				_ = recordDetectionCheck(opts, fp, "dameng", "failed", "artifact_failed", writeErr.Error(), started, time.Now())
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "dameng", "dameng %s:%d artifact failed: %v", fp.IP, fp.Port, writeErr)
			}
			if err != nil {
				status, reason := "failed", "command_failed"
				if operatorCanceled {
					status, reason = "canceled", "run_canceled"
				}
				_ = recordDetectionCheck(opts, fp, "dameng", status, reason, err.Error(), started, time.Now())
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				stageFailed = true
				progress.Emit("error", "dameng", "dameng %s:%d failed: %v", fp.IP, fp.Port, err)
			}
			if err == nil {
				switch damengResult.Verdict {
				case tools.DamengVulnerable:
					finding := report.Finding{
						IP:       fp.IP,
						Port:     fp.Port,
						Protocol: fp.Protocol,
						Source:   "dameng",
						ID:       "dameng-default-password",
						Severity: "high",
						Summary:  "Dameng Database Default Password (SYSDBA/SYSDBA)",
						Target:   fp.IP,
						Output:   strings.TrimSpace(string(out)),
					}
					if err := persistFinding(opts, finding); err != nil {
						_ = recordDetectionCheck(opts, fp, "dameng", "failed", "persistence_failed", err.Error(), started, time.Now())
						return result, err
					}
					allFindings = append(allFindings, finding)
					progress.Emit("info", "dameng", "dameng %s:%d VULNERABLE default password", fp.IP, fp.Port)
				case tools.DamengSafe:
					progress.Emit("info", "dameng", "dameng %s:%d SAFE default password changed", fp.IP, fp.Port)
				default:
					progress.Emit("info", "dameng", "dameng %s:%d UNKNOWN (no conclusion, see artifact)", fp.IP, fp.Port)
				}
			}
			if !stageFailed {
				if err := recordDetectionCheck(opts, fp, "dameng", "completed", "", "", started, time.Now()); err != nil {
					return TargetScan{}, err
				}
			}
		}
	}

	return TargetScan{Target: target, Fingerprints: allFingerprints, Findings: allFindings, OpenPorts: openPorts, HadErrors: result.HadErrors}, nil
}

func filterScopeFingerprints(scope target.Scope, fingerprints []fingerprint.ServiceFingerprint) []fingerprint.ServiceFingerprint {
	filtered := make([]fingerprint.ServiceFingerprint, 0, len(fingerprints))
	for _, fp := range fingerprints {
		if scope.Allows(fp.IP) {
			filtered = append(filtered, fp)
		}
	}
	return filtered
}

func scopeAllowsFinding(scope target.Scope, finding report.Finding) bool {
	return scope.IsZero() || scope.Allows(finding.IP)
}

func scopeAllowsNucleiFinding(scope target.Scope, result tools.NucleiFinding, finding report.Finding) bool {
	if !scopeAllowsFinding(scope, finding) {
		return false
	}
	for _, host := range result.EndpointHosts() {
		if !scope.Allows(host) {
			return false
		}
	}
	return true
}

func scopeAllowsURL(scope target.Scope, value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && scope.Allows(parsed.Hostname())
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
