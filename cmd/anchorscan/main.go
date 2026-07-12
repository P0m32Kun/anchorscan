package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/version"
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
