package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
)

func runTool(args []string, stdout io.Writer, stderr io.Writer, deps cliDeps) error {
	if len(args) == 0 || isHelpRequest(args[0]) {
		printToolHelp(stdout)
		if len(args) == 0 {
			return errors.New("usage: anchorscan tool <rustscan|nmap|httpx|nuclei>")
		}
		return nil
	}

	toolName := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("tool "+toolName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	jsonPath := fs.String("json", "", "path to JSON report output")
	projectID := fs.String("project", "", "project id")
	targetValue := fs.String("target", "", "target host")
	urlValue := fs.String("url", "", "target URL")
	portsValue := fs.String("ports", "", "ports preset or csv")
	modeValue := fs.String("mode", "service", "nmap mode: service or alive")
	tagsValue := fs.String("tags", "", "nuclei tags csv")
	templateValue := fs.String("template", "", "nuclei template path")
	extraArgsValue := fs.String("args", "", "extra tool args")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printToolHelp(stdout)
			return nil
		}
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	timeouts, err := cfg.Timeouts.Durations()
	if err != nil {
		return err
	}
	resolvedPorts := strings.TrimSpace(*portsValue)
	if toolName == "rustscan" || (toolName == "nmap" && *modeValue != "alive") {
		if resolvedPorts == "" {
			resolvedPorts = cfg.Scan.Ports
		}
		resolvedPorts, err = ports.Resolve(resolvedPorts, filepath.Dir(*configPath))
		if err != nil {
			return err
		}
	}

	extraArgs, err := config.SplitArgs(*extraArgsValue)
	if err != nil {
		return err
	}
	if *jsonPath == "" {
		*jsonPath = filepath.Join("reports", "tool-"+toolName+"-"+deps.now().Format("20060102-150405")+".json")
	}
	if err := ensureParentDir(*dbPath); err != nil {
		return err
	}
	if err := ensureParentDir(*jsonPath); err != nil {
		return err
	}
	scanStore, err := deps.openStore(*dbPath)
	if err != nil {
		return err
	}

	runID := "tool-" + toolName + "-" + deps.now().Format("20060102-150405")
	opts := app.ToolRunOptions{
		RunID:     runID,
		ProjectID: *projectID,
		Tool:      toolName,
		Mode:      *modeValue,
		Target:    *targetValue,
		Ports:     resolvedPorts,
		URL:       *urlValue,
		Tags:      splitCSV(*tagsValue),
		Template:  *templateValue,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		Timeouts:       timeouts,
		JSONReportPath: *jsonPath,
		Logf: func(format string, args ...any) {
			logScan(stderr, format, args...)
		},
	}
	applyToolExtraArgs(&opts, toolName, extraArgs)
	if err := app.RunTool(context.Background(), deps.newRunner(), scanStore, opts); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "run_id=%s\njson=%s\n", runID, *jsonPath)
	return nil
}

func applyToolExtraArgs(opts *app.ToolRunOptions, toolName string, args []string) {
	switch toolName {
	case "rustscan":
		opts.ExtraArgs.Rustscan = args
	case "nmap":
		opts.ExtraArgs.Nmap = args
	case "httpx":
		opts.ExtraArgs.Httpx = args
	case "nuclei":
		opts.ExtraArgs.Nuclei = args
	}
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func printToolHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan tool <rustscan|nmap|httpx|nuclei> [flags]

Examples:
  anchorscan tool rustscan --target 192.168.1.10 --ports 80,443
  anchorscan tool nmap --target 192.168.1.10 --ports 80,443
  anchorscan tool nmap --target 192.168.1.10 --mode alive
  anchorscan tool httpx --url http://192.168.1.10:8080
  anchorscan tool nuclei --url http://192.168.1.10:8080 --tags tomcat

Flags:
  --config <path>
  --db <path>
  --json <path>
  --project <id>
  --target <host>
  --url <url>
  --ports <value>
  --mode service|alive
  --tags <csv>
  --template <path>
  --args "..."`)
}
