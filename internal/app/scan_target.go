package app

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
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

// scanTarget runs the per-target pipeline (fathom → httpx → NSE/nuclei)
// and returns everything it discovered as a TargetScan. It persists durable
// facts through the option seams while retaining them for the JSON report.
func scanTarget(ctx context.Context, runner tools.Runner, opts ScanOptions, target string, artifactDir string, progress Progress) (TargetScan, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	logf(opts, "target %s", target)
	progress.Emit("info", "fathom", "fathom %s ports=%s", target, opts.Ports)
	ports, err := parseScanPorts(opts.Ports)
	if err != nil {
		return TargetScan{}, err
	}
	toolCtx, cancel := toolContext(ctx, opts.Timeouts.Fathom)
	fathomResult, err := tools.RunFathomScan(toolCtx, runner, opts.Tools.Fathom, target, ports)
	normalizedErr := normalizeToolError(toolCtx, err)
	cancel()
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("fathom", target)+".jsonl", fathomResult.Output); writeErr != nil {
		return TargetScan{}, writeErr
	}
	if err != nil {
		return TargetScan{}, normalizedErr
	}
	// Scope filtering mirrors the pre-fathom pipeline: a zero scope (direct
	// RunScan callers, no PrepareScan) allows everything, and target.Scope.Allows
	// returns false for an empty include set, so the filter must stay conditional.
	fingerprints := fathomResult.Fingerprints
	if !opts.Scope.IsZero() {
		fingerprints = filterScopeFingerprints(opts.Scope, fathomResult.Fingerprints)
	}
	openPorts := make([]int, 0, len(fathomResult.Fingerprints))
	for _, fp := range fathomResult.Fingerprints {
		openPorts = append(openPorts, fp.Port)
	}
	progress.Emit("info", "fathom", "fathom %s services=%d", target, len(fingerprints))
	if len(fingerprints) == 0 {
		progress.Emit("info", "target", "target %s has no open ports; skip fingerprint and vulnerability checks", target)
		return TargetScan{Target: target, OpenPorts: openPorts}, nil
	}

	// fathom checks (verdict=vulnerable/safe/unknown) persist as high-severity
	// findings (vulnerable only) and as DetectionCheck audit rows before the
	// back-stage loop, mirroring the front stage's persistence position: these
	// durable facts survive even when a later optional stage fails.
	for _, finding := range fathomResult.Findings {
		if err := persistFinding(opts, finding); err != nil {
			return TargetScan{}, err
		}
		allFindings = append(allFindings, finding)
		// Live terminal hit line, mirroring the rdpscan/dameng VULNERABLE style
		// (info level + uppercase hit marker) so the frontend's per-line green
		// hit coloring catches fathom findings too. The parenthesized summary
		// keeps the original check id for traceability.
		progress.Emit("info", "fathom", "fathom %s:%d %s (%s)", finding.IP, finding.Port, strings.ToUpper(finding.ID), finding.Summary)
	}
	for _, check := range fathomResult.Checks {
		now := time.Now()
		fp := fingerprint.ServiceFingerprint{IP: check.IP, Port: check.Port, Protocol: check.Protocol}
		if err := recordDetectionCheck(opts, fp, check.Engine, check.Status, check.ReasonCode, check.Detail, now, now); err != nil {
			return TargetScan{}, err
		}
	}

	result := TargetScan{Target: target, OpenPorts: openPorts}
	for _, fp := range fingerprints {
		if opts.PersistFingerprint != nil {
			if err := opts.PersistFingerprint(fp); err != nil {
				return result, err
			}
		}
		httpResult := tools.HTTPResult{}
		if opts.Tools.Httpx != "" && (fp.IsWeb || tools.NeedsTLSWebEnhancement(fp.Normalized, fp.Port)) {
			probe := fp
			if probe.URL == "" {
				// fathom cannot complete a TLS handshake (spec decision 2): probe
				// the TLS web candidate port with an https URL so httpx can perform
				// the handshake and upgrade the fingerprint.
				probe.URL = (&url.URL{Scheme: "https", Host: net.JoinHostPort(fp.IP, strconv.Itoa(fp.Port))}).String()
			}
			progress.Emit("info", "httpx", "httpx %s", probe.URL)
			toolCtx, cancel = toolContext(ctx, opts.Timeouts.Httpx)
			var out []byte
			httpResult, out, err = tools.EnrichWebWithOutput(toolCtx, runner, opts.Tools.Httpx, probe, opts.ExtraArgs.Httpx)
			operatorCanceled := isOperatorCanceled(toolCtx)
			cancel()
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("httpx", fp.IP, strconv.Itoa(fp.Port))+".jsonl", out); writeErr != nil {
				result.HadErrors = true
				progress.Emit("error", "httpx", "httpx %s artifact failed: %v", probe.URL, writeErr)
			}
			if err != nil {
				if operatorCanceled {
					return result, context.Canceled
				}
				result.HadErrors = true
				progress.Emit("error", "httpx", "httpx %s failed: %v", probe.URL, err)
			}
			if httpResult.URL != "" && (opts.Scope.IsZero() || scopeAllowsURL(opts.Scope, httpResult.URL)) {
				fp.URL = httpResult.URL
				fp.IsWeb = true
			}
		}
		// Nuclei is a Dameng protocol authority alongside fathom. Nmap labels and
		// ports are candidates only and never authorize a credential attempt.
		damengMatched := false
		var damengIdentifyErr error
		switch {
		case fp.Normalized == "dameng":
			// fathom identified Dameng through its protocol-level handshake
			// (spec decision 4; authority equivalent to nuclei dameng-detect),
			// so the nuclei dameng-identify round trip is skipped and the
			// default-password check runs straight away.
			damengMatched = true
		case !fp.IsWeb && opts.Tools.Dameng != "" && opts.Tools.Nuclei != "" && isDamengCandidate(fp):
			// Only ports fathom could not identify (service "unknown") are
			// probed with nuclei dameng-detect: a non-standard Dameng port
			// (not 5236) escapes fathom's rule table, and the nuclei template
			// is the fallback authority. Ports fathom already fingerprinted by
			// protocol handshake (redis/mysql/ssh/...) cannot be Dameng and
			// skip the round trip.
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
						fp.Service = "dameng"
						fp.Product = "Dameng Database"
						fp.Normalized = "dameng"
						if len(finding.ExtractedResults) > 0 {
							fp.Version = strings.TrimSpace(finding.ExtractedResults[0])
						}
						break
					}
				}
			}
			if damengIdentifyErr != nil {
				progress.Emit("error", "dameng-identify", "dameng-identify %s:%d failed after %s: %v", fp.IP, fp.Port, time.Since(started).Round(time.Second), damengIdentifyErr)
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
						Summary:  fmt.Sprintf("Dameng Database Default Password (%s/%s)", damengResult.Username, damengResult.Password),
						Target:   fp.IP,
						Output:   strings.TrimSpace(damengResult.Output),
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

// parseScanPorts expands a resolved port spec ("22,80" / "1-65535") into the
// integer list fathom scans. PrepareScan already expands the "top1000" preset
// (fathom -p accepts explicit lists/ranges only), so scanTarget sees only
// CSV/range specs. The grammar mirrors internal/ports.expandPortSpec (same
// CSV + range parsing, dedup, ascending order) because that helper is
// unexported and this task's scope forbids touching internal/ports.
func parseScanPorts(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("ports is empty")
	}
	if spec == "top1000" {
		return nil, fmt.Errorf("port preset %q must be expanded before scanTarget runs", spec)
	}
	var out []int
	seen := map[int]struct{}{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		start, end, err := scanPortBounds(part)
		if err != nil {
			return nil, err
		}
		for port := start; port <= end; port++ {
			if _, ok := seen[port]; !ok {
				seen[port] = struct{}{}
				out = append(out, port)
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

func scanPortBounds(part string) (int, int, error) {
	if idx := strings.IndexByte(part, '-'); idx >= 0 {
		start, err := parseScanPort(part[:idx])
		if err != nil {
			return 0, 0, err
		}
		end, err := parseScanPort(part[idx+1:])
		if err != nil {
			return 0, 0, err
		}
		if end < start {
			start, end = end, start
		}
		return start, end, nil
	}
	port, err := parseScanPort(part)
	if err != nil {
		return 0, 0, err
	}
	return port, port, nil
}

// isDamengCandidate reports whether a fingerprint warrants a nuclei
// dameng-detect round trip: only ports fathom left unidentified ("unknown").
// A protocol-handshake-identified service (redis/mysql/ssh/...) cannot be
// Dameng, and a fathom "dameng" fingerprint already bypasses this branch.
func isDamengCandidate(fp fingerprint.ServiceFingerprint) bool {
	return strings.EqualFold(strings.TrimSpace(fp.Normalized), "unknown") ||
		strings.TrimSpace(fp.Normalized) == ""
}

func parseScanPort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", strings.TrimSpace(value))
	}
	return port, nil
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
