package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

var nmapHeartbeatEvery = 30 * time.Second

type ToolPaths = config.ToolPaths
type ToolExtraArgs = config.ToolArgs

type TagRule = vuln.TagRule

type ScanOptions struct {
	RunID          string
	ProjectID      string
	Targets        []string
	Ports          string
	Tools          ToolPaths
	ProfileName    string
	HostWorkers    int
	ExtraArgs      ToolExtraArgs
	ConfigSnapshot string
	JSONReportPath string
	ArtifactRoot   string
	NSERules       map[string][]string
	TagRules       []TagRule
	Logf           func(format string, args ...any)
}

func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) (runErr error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding
	scanTargets := opts.Targets
	artifactDir := ""

	if opts.ProfileName == "" {
		opts.ProfileName = "normal"
	}
	if opts.RunID != "" && strings.TrimSpace(opts.ArtifactRoot) != "" {
		artifactDir = filepath.Join(opts.ArtifactRoot, opts.RunID)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return err
		}
	}
	if opts.RunID != "" && scanStore != nil {
		_ = scanStore.SaveScanRun(store.ScanRun{
			RunID:          opts.RunID,
			ProjectID:      opts.ProjectID,
			Target:         strings.Join(opts.Targets, ","),
			Ports:          opts.Ports,
			Profile:        opts.ProfileName,
			Status:         "running",
			StartedAt:      time.Now(),
			ConfigSnapshot: opts.ConfigSnapshot,
			ArtifactDir:    artifactDir,
		})
	}
	defer func() {
		if opts.RunID == "" || scanStore == nil {
			return
		}
		status := "completed"
		message := ""
		if runErr != nil {
			status = "failed"
			message = runErr.Error()
			if errors.Is(runErr, context.Canceled) {
				status = "canceled"
			}
		}
		_ = scanStore.UpdateScanRunStatus(opts.RunID, status, message, time.Now())
	}()

	if opts.Tools.Nmap != "" && len(scanTargets) > 0 {
		emit(opts, scanStore, "info", "nmap", "nmap alive sweep targets=%v", scanTargets)
		aliveTargets, out, err := tools.DiscoverAliveWithOutput(ctx, runner, opts.Tools.Nmap, scanTargets, opts.ExtraArgs.Nmap)
		if _, writeErr := writeArtifact(artifactDir, "nmap-alive-targets.xml", out); writeErr != nil {
			return writeErr
		}
		if err != nil {
			return normalizeToolError(ctx, err)
		}
		scanTargets = aliveTargets
		emit(opts, scanStore, "info", "nmap", "nmap alive hosts=%v", scanTargets)
		if len(scanTargets) == 0 {
			emit(opts, scanStore, "info", "target", "no live hosts discovered; skip port scan")
		}
	}

	workers := opts.HostWorkers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(scanTargets) {
		workers = len(scanTargets)
	}
	if workers > 0 {
		targetCh := make(chan string)
		results := make(chan targetResult, len(scanTargets))
		var wg sync.WaitGroup

		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for target := range targetCh {
					if ctx.Err() != nil {
						return
					}
					fingerprints, findings, err := scanTarget(ctx, runner, scanStore, opts, target, artifactDir)
					results <- targetResult{
						target:       target,
						fingerprints: fingerprints,
						findings:     findings,
						err:          err,
					}
				}
			}()
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		go func() {
			defer close(targetCh)
			for _, target := range scanTargets {
				select {
				case <-ctx.Done():
					return
				case targetCh <- target:
				}
			}
		}()

		var canceledErr error
		var failed int
		var failedTargets []targetResult
		var firstErr error
		for result := range results {
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) {
					if canceledErr == nil {
						canceledErr = result.err
					}
					continue
				}
				failed++
				if firstErr == nil {
					firstErr = result.err
				}
				failedTargets = append(failedTargets, result)
				continue
			}
			allFingerprints = append(allFingerprints, result.fingerprints...)
			allFindings = append(allFindings, result.findings...)
		}
		for _, result := range failedTargets {
			emit(opts, scanStore, "error", "target", "target %s failed: %v", result.target, result.err)
		}
		if canceledErr != nil {
			return canceledErr
		}
		if failed == len(scanTargets) {
			return fmt.Errorf("all targets failed: %w", firstErr)
		}
	}

	emit(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(allFingerprints, allFindings))
}

type targetResult struct {
	target       string
	fingerprints []fingerprint.ServiceFingerprint
	findings     []report.Finding
	err          error
}

func scanTarget(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, target string, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	logf(opts, "target %s", target)
	emit(opts, scanStore, "info", "rustscan", "rustscan %s ports=%s", target, opts.Ports)
	ports, out, err := tools.DiscoverPortsWithOutput(ctx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("rustscan", target, "ports")+".txt", out); writeErr != nil {
		return nil, nil, writeErr
	}
	if err != nil {
		return nil, nil, normalizeToolError(ctx, err)
	}
	emit(opts, scanStore, "info", "rustscan", "rustscan %s open=%v", target, ports)
	if len(ports) == 0 {
		emit(opts, scanStore, "info", "target", "target %s has no open ports; skip fingerprint and vulnerability checks", target)
		return nil, nil, nil
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
	if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nmap-service", target, joinIntParts(ports))+".xml", out); writeErr != nil {
		return nil, nil, writeErr
	}
	if err != nil {
		return nil, nil, normalizeToolError(ctx, err)
	}
	emit(opts, scanStore, "info", "nmap", "nmap %s services=%d elapsed=%s", target, len(fingerprints), time.Since(started).Round(time.Second))

	for _, fp := range fingerprints {
		httpResult := tools.HTTPResult{}
		if fp.IsWeb && opts.Tools.Httpx != "" {
			emit(opts, scanStore, "info", "httpx", "httpx %s", fp.URL)
			httpResult, out, err = tools.EnrichWebWithOutput(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("httpx", fp.IP, strconv.Itoa(fp.Port))+".jsonl", out); writeErr != nil {
				return nil, nil, writeErr
			}
			if err != nil {
				return nil, nil, normalizeToolError(ctx, err)
			}
			if httpResult.URL != "" {
				fp.URL = httpResult.URL
			}
		}

		if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
			return nil, nil, err
		}
		allFingerprints = append(allFingerprints, fp)

		for _, finding := range ManualReviewFindings(fp) {
			if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
				return nil, nil, err
			}
			allFindings = append(allFindings, finding)
		}

		scripts := vuln.MatchNSE(fp, opts.NSERules)
		if len(scripts) > 0 && !fp.IsWeb {
			emit(opts, scanStore, "info", "nse", "nse %s:%d scripts=%v", fp.IP, fp.Port, scripts)
			nseResults, out, err := tools.RunNSEWithOutput(ctx, runner, opts.Tools.Nmap, fp.IP, fp.Port, scripts, opts.ExtraArgs.Nmap)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nse", fp.IP, strconv.Itoa(fp.Port), strings.Join(scripts, ","))+".xml", out); writeErr != nil {
				return nil, nil, writeErr
			}
			if err != nil {
				return nil, nil, normalizeToolError(ctx, err)
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
					return nil, nil, err
				}
				allFindings = append(allFindings, finding)
			}
		}

		match := vuln.MatchNucleiTags(fp, vuln.HTTPResult{URL: fp.URL, Tech: httpResult.Tech}, opts.TagRules)
		if len(match.Tags) > 0 && opts.Tools.Nuclei != "" {
			emit(opts, scanStore, "info", "nuclei", "nuclei %s tags=%v", match.Address, match.Tags)
			out, err := tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, match.Address, match.Tags, opts.ExtraArgs.Nuclei)
			if _, writeErr := writeArtifact(artifactDir, safeArtifactName("nuclei", fp.IP, strconv.Itoa(fp.Port), strings.Join(match.Tags, ","))+".jsonl", out); writeErr != nil {
				return nil, nil, writeErr
			}
			if err != nil {
				return nil, nil, normalizeToolError(ctx, err)
			}
			nucleiFindings, err := tools.ParseNucleiJSONL(out)
			if err != nil {
				return nil, nil, err
			}
			for _, result := range nucleiFindings {
				finding := report.Finding{
					IP:       fp.IP,
					Port:     fp.Port,
					Protocol: fp.Protocol,
					Source:   "nuclei",
					ID:       result.TemplateID,
					Severity: result.Severity,
					Summary:  result.Name,
					Target:   result.MatchedAt,
					Output:   formatNucleiEvidence(result),
				}
				if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
					return nil, nil, err
				}
				allFindings = append(allFindings, finding)
			}
		}
	}

	return allFingerprints, allFindings, nil
}

func logf(opts ScanOptions, format string, args ...any) {
	if opts.Logf != nil {
		opts.Logf(format, args...)
	}
}

func emit(opts ScanOptions, scanStore *store.Store, level string, stage string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logf(opts, "%s", message)
	if opts.RunID == "" || scanStore == nil {
		return
	}
	_ = scanStore.AppendScanEvent(store.ScanEvent{
		RunID:   opts.RunID,
		Time:    time.Now(),
		Level:   level,
		Stage:   stage,
		Message: message,
	})
}

func normalizeToolError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	return err
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
