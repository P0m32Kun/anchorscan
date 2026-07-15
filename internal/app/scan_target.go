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

func scanTarget(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, target string, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, []int, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	logf(opts, "target %s", target)
	emit(opts, scanStore, "info", "rustscan", "rustscan %s ports=%s", target, opts.Ports)
	ports, out, err := tools.DiscoverPortsWithOutput(ctx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("rustscan", target, "ports")+".txt", out); writeErr != nil {
		return nil, nil, nil, writeErr
	}
	if err != nil {
		return nil, nil, nil, normalizeToolError(ctx, err)
	}
	emit(opts, scanStore, "info", "rustscan", "rustscan %s open=%v", target, ports)
	openPorts := append([]int(nil), ports...)
	if len(ports) == 0 {
		emit(opts, scanStore, "info", "target", "target %s has no open ports; skip fingerprint and vulnerability checks", target)
		return nil, nil, openPorts, nil
	}

	emit(opts, scanStore, "info", "nmap", "nmap %s ports=%v (service detection may be slow)", target, ports)
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
		return nil, nil, openPorts, writeErr
	}
	if err != nil {
		return nil, nil, openPorts, normalizeToolError(ctx, err)
	}
	emit(opts, scanStore, "info", "nmap", "nmap %s services=%d elapsed=%s", target, len(fingerprints), time.Since(started).Round(time.Second))

	for _, fp := range fingerprints {
		httpResult := tools.HTTPResult{}
		if fp.IsWeb && opts.Tools.Httpx != "" {
			emit(opts, scanStore, "info", "httpx", "httpx %s", fp.URL)
			httpResult, out, err = tools.EnrichWebWithOutput(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("httpx", fp.IP, strconv.Itoa(fp.Port))+".jsonl", out); writeErr != nil {
				return nil, nil, openPorts, writeErr
			}
			if err != nil {
				return nil, nil, openPorts, normalizeToolError(ctx, err)
			}
			if httpResult.URL != "" {
				fp.URL = httpResult.URL
			}
		}

		if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
			return nil, nil, openPorts, err
		}
		allFingerprints = append(allFingerprints, fp)

		for _, finding := range ManualReviewFindings(fp) {
			if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
				return nil, nil, openPorts, err
			}
			allFindings = append(allFindings, finding)
		}

		scripts := vuln.MatchNSE(fp, opts.NSERules)
		if len(scripts) > 0 && !fp.IsWeb {
			emit(opts, scanStore, "info", "nse", "nse %s:%d scripts=%v", fp.IP, fp.Port, scripts)
			nseResults, out, err := tools.RunNSEWithOutput(ctx, runner, opts.Tools.Nmap, fp.IP, fp.Port, scripts, opts.ExtraArgs.Nmap)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nse", fp.IP, strconv.Itoa(fp.Port), strings.Join(scripts, ","))+".xml", out); writeErr != nil {
				return nil, nil, openPorts, writeErr
			}
			if err != nil {
				return nil, nil, openPorts, normalizeToolError(ctx, err)
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
				if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
					return nil, nil, openPorts, err
				}
				allFindings = append(allFindings, finding)
			}
		}

		match := vuln.MatchNucleiTags(fp, vuln.HTTPResult{URL: fp.URL, Tech: httpResult.Tech}, opts.TagRules)
		if len(match.Tags) > 0 && opts.Tools.Nuclei != "" {
			emit(opts, scanStore, "info", "nuclei", "nuclei %s tags=%v", match.Address, match.Tags)
			out, err := tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, match.Address, match.Tags, match.ExcludeTags, opts.ExtraArgs.Nuclei)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nuclei", fp.IP, strconv.Itoa(fp.Port), strings.Join(match.Tags, ","))+".jsonl", out); writeErr != nil {
				return nil, nil, openPorts, writeErr
			}
			if err != nil {
				return nil, nil, openPorts, normalizeToolError(ctx, err)
			}
			nucleiFindings, err := tools.ParseNucleiJSONL(out)
			if err != nil {
				return nil, nil, openPorts, err
			}
			for _, result := range nucleiFindings {
				finding := findingFromNuclei(result, fp, allFingerprints)
				if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
					return nil, nil, openPorts, err
				}
				allFindings = append(allFindings, finding)
			}
		}
	}

	return allFingerprints, allFindings, openPorts, nil
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
