package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/doctor"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/target"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/version"
	"github.com/P0m32Kun/anchorscan/internal/web"
)

type cliDeps struct {
	newRunner func() tools.Runner
	openStore func(path string) (*store.Store, error)
	now       func() time.Time
}

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Execute() error {
	return run(os.Args[1:], os.Stdout, os.Stderr, cliDeps{})
}

func run(args []string, stdout io.Writer, stderr io.Writer, deps cliDeps) error {
	if len(args) == 0 || isHelpRequest(args[0]) {
		printRootHelp(stdout)
		return nil
	}
	if args[0] == "--version" || args[0] == "-v" {
		printVersion(stdout)
		return nil
	}

	if deps.newRunner == nil {
		deps.newRunner = tools.NewExecRunner
	}
	if deps.openStore == nil {
		deps.openStore = store.Open
	}
	if deps.now == nil {
		deps.now = time.Now
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:], stdout, stderr, deps)
	case "tool":
		return runTool(args[1:], stdout, stderr, deps)
	case "doctor":
		return runDoctor(args[1:], stdout)
	case "web":
		return runWeb(args[1:], stdout, stderr, deps)
	case "cancel":
		return runCancel(args[1:], stdout)
	case "report":
		return runReport(args[1:], stdout, deps)
	case "import-nmap":
		return runImportNmap(args[1:], stdout, deps)
	case "tools":
		return runTools(args[1:], stdout)
	case "version":
		printVersion(stdout)
		return nil
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return errors.New("unknown command")
	}
}

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

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	nseRules, err := config.LoadNSERulesForConfig(*configPath)
	if err != nil {
		return err
	}
	tagRules, err := config.LoadTagRulesForConfig(*configPath)
	if err != nil {
		return err
	}
	targets, err := target.Parse(*targetSpec)
	if err != nil {
		return err
	}

	portValue := cfg.Scan.Ports
	if *portsSpec != "" {
		portValue = *portsSpec
	}
	resolvedPorts, err := ports.ResolveForConfig(portValue, *configPath)
	if err != nil {
		return err
	}

	effective, err := config.ResolveScan(cfg, config.Overrides{
		ProfileName:  *profileFlag,
		HostWorkers:  *hostWorkersFlag,
		RustscanArgs: *rustscanArgsFlag,
		NmapArgs:     *nmapArgsFlag,
		HttpxArgs:    *httpxArgsFlag,
		NucleiArgs:   *nucleiArgsFlag,
	})
	if err != nil {
		return err
	}

	if *jsonPath == "" {
		*jsonPath = filepath.Join("reports", "scan-"+deps.now().Format("20060102-150405")+".json")
	}

	preflightResult := preflight.Run(preflight.Options{
		ConfigDir: filepath.Dir(*configPath),
		DBPath:    *dbPath,
		JSONPath:  *jsonPath,
		ReportDir: filepath.Dir(*jsonPath),
		Targets:   targets,
		PortSpec:  portValue,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		Profile: effective.ProfileName,
		Workers: effective.HostWorkers,
		ExtraArgs: app.ToolExtraArgs{
			Rustscan: effective.Rustscan,
			Nmap:     effective.Nmap,
			Httpx:    effective.Httpx,
			Nuclei:   effective.Nuclei,
		},
		NSERuleCount: len(nseRules),
		TagRuleCount: len(tagRules),
	})
	logPreflight(stderr, preflightResult)
	if preflightResult.HasErrors() {
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
	opts := app.ScanOptions{
		RunID:   runID,
		Targets: targets,
		Ports:   resolvedPorts,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		ProfileName: effective.ProfileName,
		HostWorkers: effective.HostWorkers,
		ExtraArgs: app.ToolExtraArgs{
			Rustscan: effective.Rustscan,
			Nmap:     effective.Nmap,
			Httpx:    effective.Httpx,
			Nuclei:   effective.Nuclei,
		},
		JSONReportPath: *jsonPath,
		ArtifactRoot:   strings.TrimSpace(*artifactRoot),
		NSERules:       nseRules,
		TagRules:       tagRules,
		Logf: func(format string, args ...any) {
			logScan(stderr, format, args...)
		},
	}
	if err := app.RunScan(context.Background(), deps.newRunner(), scanStore, opts); err != nil {
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

func runDoctor(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	reportsDir := fs.String("reports", "reports", "report output directory")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printDoctorHelp(stdout)
			return nil
		}
		return err
	}

	checks := doctor.Run(doctor.Options{
		ConfigPath: *configPath,
		DBPath:     *dbPath,
		ReportDir:  *reportsDir,
	})
	for _, check := range checks {
		status := "fail"
		if check.OK {
			status = "ok"
		}
		_, _ = fmt.Fprintf(stdout, "%s: %s %s\n", check.Name, status, check.Message)
	}
	if doctor.HasFailures(checks) {
		return errors.New("doctor found issues")
	}
	return nil
}

func runWeb(args []string, stdout io.Writer, _ io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	listen := fs.String("listen", "127.0.0.1:8088", "listen address")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printWebHelp(stdout)
			return nil
		}
		return err
	}
	handler, err := web.NewServer(web.ServerOptions{
		ConfigPath: *configPath,
		DBPath:     *dbPath,
		Listen:     *listen,
		Runner:     deps.newRunner(),
		Now:        deps.now,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "listening on http://%s\n", *listen)
	return http.ListenAndServe(*listen, handler)
}

func runCancel(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("cancel", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run-id", "", "scan run id")
	serverURL := fs.String("server", "http://127.0.0.1:8088", "local web console URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("cancel requires --run-id")
	}
	resp, err := http.Post(strings.TrimRight(*serverURL, "/")+"/runs/"+*runID+"/cancel", "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("cancel failed: %s", resp.Status)
	}
	_, _ = fmt.Fprintf(stdout, "canceled %s\n", *runID)
	return nil
}

func runTools(args []string, stdout io.Writer) error {
	if len(args) == 0 || isHelpRequest(args[0]) {
		printToolsHelp(stdout)
		return nil
	}
	if args[0] != "check" {
		return errors.New("usage: anchorscan tools check --config <path>")
	}

	fs := flag.NewFlagSet("tools check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printToolsCheckHelp(stdout)
			return nil
		}
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	for name, path := range map[string]string{
		"rustscan": cfg.Tools.Rustscan,
		"nmap":     cfg.Tools.Nmap,
		"httpx":    cfg.Tools.Httpx,
		"nuclei":   cfg.Tools.Nuclei,
	} {
		if err := checkToolPath(path); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		_, _ = fmt.Fprintf(stdout, "%s: ok (%s)\n", name, path)
	}
	return nil
}

func checkToolPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("path is a directory")
	}
	return nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func isHelpRequest(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "anchorscan version %s\n", version.Version)
}

func printRootHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan <command> [flags]

Commands:
  scan        Run discovery, fingerprinting, and reporting
  tool        Run one scanner and store its results
  doctor      Validate config, tools, and paths
  web         Start the local Web Console
  cancel      Cancel a Web-managed scan
  report      Rebuild reports from stored results
  import-nmap Import an existing Nmap XML into an AnchorScan run
  tools check Verify configured external tools
  version     Print the AnchorScan version

Examples:
  anchorscan doctor --config config/default.yaml
  anchorscan tool nmap --target 127.0.0.1 --mode alive
  anchorscan web --config config/default.yaml --db data/scans.sqlite
  anchorscan cancel --run-id 20260707-120000

Global flags:
  -h, --help  Show help`)
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

func printDoctorHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan doctor [flags]

Flags:
  --config <path>   Config file path
  --db <path>       SQLite database path
  --reports <path>  Report output directory`)
}

func printWebHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan web [flags]

Flags:
  --config <path>   Config file path
  --db <path>       SQLite database path
  --listen <addr>   Listen address`)
}

func printToolsHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan tools check [flags]

Subcommands:
  check     Verify configured tool paths`)
}

func printToolsCheckHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan tools check --config <path>

Flags:
  --config <path>   Config file path`)
}
