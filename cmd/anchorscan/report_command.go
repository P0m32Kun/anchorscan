package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func runReport(args []string, stdout io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	runID := fs.String("run-id", "", "scan run id")
	jsonPath := fs.String("json", "", "path to JSON report output")
	htmlPath := fs.String("html", "", "path to HTML report output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printReportHelp(stdout)
			return nil
		}
		return err
	}
	if *runID == "" {
		return errors.New("report requires --run-id")
	}
	if *jsonPath == "" && *htmlPath == "" {
		return errors.New("report requires --json or --html")
	}
	if _, err := config.Load(*configPath); err != nil {
		return err
	}

	scanStore, err := deps.openStore(*dbPath)
	if err != nil {
		return err
	}
	fps, err := scanStore.ListFingerprints(*runID)
	if err != nil {
		return err
	}
	findings, err := scanStore.ListFindings(*runID)
	if err != nil {
		return err
	}
	builtReport := report.Build(fps, findings)
	if *jsonPath != "" {
		if err := ensureParentDir(*jsonPath); err != nil {
			return err
		}
		if err := report.WriteJSON(*jsonPath, builtReport); err != nil {
			return err
		}
	}
	if *htmlPath != "" {
		if err := ensureParentDir(*htmlPath); err != nil {
			return err
		}
		if err := report.WriteHTML(*htmlPath, builtReport); err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintf(stdout, "run_id=%s\n", *runID)
	return nil
}

func runImportNmap(args []string, stdout io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("import-nmap", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	xmlPath := fs.String("xml", "", "path to Nmap XML file to import")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	runID := fs.String("run-id", "", "import run id (default: import-<timestamp>)")
	projectID := fs.String("project", "", "project id to attach the run to")
	jsonPath := fs.String("json", "", "path to JSON report output")
	htmlPath := fs.String("html", "", "path to HTML report output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printImportNmapHelp(stdout)
			return nil
		}
		return err
	}
	if *xmlPath == "" {
		return errors.New("import-nmap requires --xml")
	}

	scanStore, err := deps.openStore(*dbPath)
	if err != nil {
		return err
	}

	resolvedRunID, err := app.ImportNmap(context.Background(), scanStore, app.ImportNmapOptions{
		XMLPath:   *xmlPath,
		RunID:     *runID,
		ProjectID: *projectID,
		JSONPath:  *jsonPath,
		HTMLPath:  *htmlPath,
	})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "run_id=%s\n", resolvedRunID)
	return nil
}

func printReportHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan report --run-id <id> [flags]

Flags:
  --config <path>   Config file path
  --db <path>       SQLite database path
  --run-id <id>     Scan run id
  --json <path>     JSON report output path
  --html <path>     HTML report output path`)
}

func printImportNmapHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan import-nmap --xml <path> [flags]

Import an existing Nmap XML file as a completed AnchorScan run, preserving
service protocol, CPE and NSE script output. Reuses the existing SQLite,
JSON/HTML report and Web Console pipeline.

Flags:
  --xml <path>      Nmap XML file to import (required)
  --db <path>       SQLite database path
  --run-id <id>     Import run id (default: import-<timestamp>)
  --project <id>    Project id to attach the run to
  --json <path>     JSON report output path (optional)
  --html <path>     HTML report output path (optional)`)
}
