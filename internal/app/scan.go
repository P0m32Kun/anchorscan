package app

import (
	"context"
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
	Targets        []string
	Ports          string
	Tools          ToolPaths
	ProfileName    string
	HostWorkers    int
	ExtraArgs      ToolExtraArgs
	JSONReportPath string
	NSERules       map[string][]string
	TagRules       []TagRule
	Logf           func(format string, args ...any)
}

func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) error {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding

	for _, target := range opts.Targets {
		logf(opts, "target %s", target)
		logf(opts, "rustscan %s ports=%s", target, opts.Ports)
		ports, err := tools.DiscoverPorts(ctx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
		if err != nil {
			return err
		}
		logf(opts, "rustscan %s open=%v", target, ports)

		logf(opts, "nmap %s ports=%v (service detection may be slow)", target, ports)
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
		logf(opts, "nmap %s services=%d elapsed=%s", target, len(fingerprints), time.Since(started).Round(time.Second))

		for _, fp := range fingerprints {
			httpResult := tools.HTTPResult{}
			if fp.IsWeb && opts.Tools.Httpx != "" {
				logf(opts, "httpx %s", fp.URL)
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
				logf(opts, "nse %s:%d scripts=%v", fp.IP, fp.Port, scripts)
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
				logf(opts, "nuclei %s tags=%v", match.Address, match.Tags)
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

	logf(opts, "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(allFingerprints, allFindings))
}

func logf(opts ScanOptions, format string, args ...any) {
	if opts.Logf != nil {
		opts.Logf(format, args...)
	}
}
