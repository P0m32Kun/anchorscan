package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/doctor"
	"github.com/P0m32Kun/anchorscan/internal/web"
)

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
		ConfigPath:        *configPath,
		DBPath:            *dbPath,
		ReportDir:         *reportsDir,
		DocxRenderProject: filepath.Join("tools", "docx-render"),
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
		ConfigPath:        *configPath,
		DBPath:            *dbPath,
		Listen:            *listen,
		Runner:            deps.newRunner(),
		Now:               deps.now,
		DocxTemplatePath:  filepath.Join("tools", "docx-render", "templates", "project-report.docx"),
		DocxRenderProject: filepath.Join("tools", "docx-render"),
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
