package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

var nmapHeartbeatEvery = 30 * time.Second

type ToolPaths struct {
	Rustscan string
	Nmap     string
	Httpx    string
	Nuclei   string
}

type ToolExtraArgs struct {
	Rustscan []string
	Nmap     []string
	Httpx    []string
	Nuclei   []string
}

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
	NSERules       map[string][]string
	TagRules       []TagRule
	Logf           func(format string, args ...any)
}

func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) (runErr error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	if opts.ProfileName == "" {
		opts.ProfileName = "normal"
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

	for _, target := range opts.Targets {
		logf(opts, "target %s", target)
		emit(opts, scanStore, "info", "rustscan", "rustscan %s ports=%s", target, opts.Ports)
		ports, err := tools.DiscoverPorts(ctx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
		if err != nil {
			return err
		}
		emit(opts, scanStore, "info", "rustscan", "rustscan %s open=%v", target, ports)

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
		fingerprints, err := tools.Fingerprint(ctx, runner, opts.Tools.Nmap, target, ports, opts.ExtraArgs.Nmap)
		close(done)
		if err != nil {
			return err
		}
		emit(opts, scanStore, "info", "nmap", "nmap %s services=%d elapsed=%s", target, len(fingerprints), time.Since(started).Round(time.Second))

		for _, fp := range fingerprints {
			httpResult := tools.HTTPResult{}
			if fp.IsWeb && opts.Tools.Httpx != "" {
				emit(opts, scanStore, "info", "httpx", "httpx %s", fp.URL)
				httpResult, err = tools.EnrichWeb(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
				if err != nil {
					return err
				}
				if httpResult.URL != "" {
					fp.URL = httpResult.URL
				}
			}

			if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
				return err
			}
			allFingerprints = append(allFingerprints, fp)

			scripts := vuln.MatchNSE(fp, opts.NSERules)
			if len(scripts) > 0 {
				emit(opts, scanStore, "info", "nse", "nse %s:%d scripts=%v", fp.IP, fp.Port, scripts)
				nseResults, err := tools.RunNSE(ctx, runner, opts.Tools.Nmap, fp.IP, fp.Port, scripts, opts.ExtraArgs.Nmap)
				if err != nil {
					return err
				}
				for _, result := range nseResults {
					finding := report.Finding{
						IP:       fp.IP,
						Port:     fp.Port,
						Source:   "nse",
						ID:       result.ID,
						Severity: "info",
						Summary:  result.ID,
						Target:   fp.IP,
						Output:   result.Output,
					}
					if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
						return err
					}
					allFindings = append(allFindings, finding)
				}
			}

			match := vuln.MatchNucleiTags(fp, vuln.HTTPResult{URL: fp.URL, Tech: httpResult.Tech}, opts.TagRules)
			if len(match.Tags) > 0 && opts.Tools.Nuclei != "" {
				emit(opts, scanStore, "info", "nuclei", "nuclei %s tags=%v", match.Address, match.Tags)
				out, err := tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, match.Address, match.Tags, opts.ExtraArgs.Nuclei)
				if err != nil {
					return err
				}
				nucleiFindings, err := tools.ParseNucleiJSONL(out)
				if err != nil {
					return err
				}
				for _, result := range nucleiFindings {
					finding := report.Finding{
						IP:       fp.IP,
						Port:     fp.Port,
						Source:   "nuclei",
						ID:       result.TemplateID,
						Severity: result.Severity,
						Summary:  result.Name,
						Target:   result.MatchedAt,
						Output:   result.MatchedAt,
					}
					if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
						return err
					}
					allFindings = append(allFindings, finding)
				}
			}
		}
	}

	emit(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(allFingerprints, allFindings))
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
