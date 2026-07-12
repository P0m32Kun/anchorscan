package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func runScan(args []string, stdout io.Writer, stderr io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	targetSpec := fs.String("target", "", "target IP, CIDR, or comma-separated list")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	jsonPath := fs.String("json", "", "path to JSON report output")
	htmlPath := fs.String("html", "", "path to HTML report output")
	artifactRoot := fs.String("artifacts", filepath.Join("data", "artifacts"), "path to scan artifact directory root")
	portsSpec := fs.String("ports", "", "ports preset or csv")
	profileFlag := fs.String("profile", "", "scan profile: slow, normal, or fast")
	hostWorkersFlag := fs.Int("host-workers", 0, "host-level worker count override")
	rustscanArgsFlag := fs.String("rustscan-args", "", "extra rustscan args")
	nmapArgsFlag := fs.String("nmap-args", "", "extra nmap args")
	httpxArgsFlag := fs.String("httpx-args", "", "extra httpx args")
	nucleiArgsFlag := fs.String("nuclei-args", "", "extra nuclei args")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printScanHelp(stdout)
			return nil
		}
		return err
	}
	if *targetSpec == "" {
		return errors.New("scan requires --target")
	}

	if *jsonPath == "" {
		*jsonPath = filepath.Join("reports", "scan-"+deps.now().Format("20060102-150405")+".json")
	}

	prepared, err := app.PrepareScan(app.PrepareScanRequest{
		ConfigPath:     *configPath,
		TargetSpec:     *targetSpec,
		PortSpec:       *portsSpec,
		DBPath:         *dbPath,
		JSONReportPath: *jsonPath,
		ArtifactRoot:   strings.TrimSpace(*artifactRoot),
		Overrides: config.Overrides{
			ProfileName:  *profileFlag,
			HostWorkers:  *hostWorkersFlag,
			RustscanArgs: *rustscanArgsFlag,
			NmapArgs:     *nmapArgsFlag,
			HttpxArgs:    *httpxArgsFlag,
			NucleiArgs:   *nucleiArgsFlag,
		},
	})
	if err != nil {
		return err
	}
	logPreflight(stderr, prepared.Preflight)
	if prepared.Preflight.HasErrors() {
		return errors.New("preflight failed")
	}

	if err := ensureParentDir(*dbPath); err != nil {
		return err
	}
	if err := ensureParentDir(*jsonPath); err != nil {
		return err
	}
	if *htmlPath != "" {
		if err := ensureParentDir(*htmlPath); err != nil {
			return err
		}
	}
	if strings.TrimSpace(*artifactRoot) != "" {
		if err := os.MkdirAll(*artifactRoot, 0o755); err != nil {
			return err
		}
	}

	scanStore, err := deps.openStore(*dbPath)
	if err != nil {
		return err
	}

	runID := deps.now().Format("20060102-150405")
	logScan(stderr, "run %s", runID)
	prepared.Options.RunID = runID
	prepared.Options.Logf = func(format string, args ...any) {
		logScan(stderr, format, args...)
	}
	if err := app.RunScan(context.Background(), deps.newRunner(), scanStore, prepared.Options); err != nil {
		return err
	}

	if *htmlPath != "" {
		logScan(stderr, "report html %s", *htmlPath)
		fps, err := scanStore.ListFingerprints(runID)
		if err != nil {
			return err
		}
		findings, err := scanStore.ListFindings(runID)
		if err != nil {
			return err
		}
		if err := report.WriteHTML(*htmlPath, report.Build(fps, findings)); err != nil {
			return err
		}
	}
	logScan(stderr, "done %s", runID)

	_, _ = fmt.Fprintf(stdout, "run_id=%s\njson=%s\n", runID, *jsonPath)
	if *htmlPath != "" {
		_, _ = fmt.Fprintf(stdout, "html=%s\n", *htmlPath)
	}
	return nil
}

func logScan(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "[scan] "+format+"\n", args...)
}

func logPreflight(w io.Writer, result preflight.Result) {
	logScan(w, "preflight targets=%d ports=%s profile=%s workers=%d", result.Summary.TargetCount, result.Summary.PortSpec, result.Summary.Profile, result.Summary.Workers)
	for _, warning := range result.Warnings {
		logScan(w, "preflight warning %s: %s", warning.Field, warning.Message)
	}
	for _, issue := range result.Errors {
		logScan(w, "preflight error %s: %s", issue.Field, issue.Message)
	}
}

func printScanHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan scan --target <target> [flags]

Flags:
  --config <path>   Config file path
  --target <value>  Target IP, CIDR, IP range, or comma-separated list
  --ports <value>   top1000, a range like 100-1000, or CSV like 80,443
  --profile slow|normal|fast
  --host-workers N
  --rustscan-args "..."
  --nmap-args "..."
  --httpx-args "..."
  --nuclei-args "..."
  --db <path>       SQLite database path
  --json <path>     JSON report output path
  --html <path>     HTML report output path
  --artifacts <path> Scan artifact directory root`)
}
