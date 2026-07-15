package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type ToolRunOptions struct {
	RunID          string
	ProjectID      string
	Tool           string
	Mode           string
	Target         string
	Ports          string
	UseNativeArgs  bool
	NativeArgs     []string
	URL            string
	Tags           []string
	Template       string
	Tools          ToolPaths
	ExtraArgs      ToolExtraArgs
	JSONReportPath string
	Logf           func(format string, args ...any)
}

func RunTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) (runErr error) {
	if opts.RunID == "" {
		return errors.New("tool run requires run id")
	}
	if scanStore == nil {
		return errors.New("tool run requires store")
	}
	if opts.JSONReportPath == "" {
		return errors.New("tool run requires json report path")
	}
	if opts.Mode == "" {
		opts.Mode = "service"
	}

	saveToolRun(scanStore, opts)
	defer func() {
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

	if opts.UseNativeArgs {
		findings, runErr := runNativeTool(ctx, runner, scanStore, opts)
		if runErr != nil {
			return runErr
		}
		emitTool(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
		return report.WriteJSON(opts.JSONReportPath, report.Build(nil, findings))
	}

	var fingerprints []fingerprint.ServiceFingerprint
	var findings []report.Finding
	switch opts.Tool {
	case "rustscan":
		fingerprints, runErr = runRustscanTool(ctx, runner, scanStore, opts)
	case "nmap":
		fingerprints, findings, runErr = runNmapTool(ctx, runner, scanStore, opts)
	case "httpx":
		fingerprints, findings, runErr = runHTTPXTool(ctx, runner, scanStore, opts)
	case "nuclei":
		findings, runErr = runNucleiTool(ctx, runner, scanStore, opts)
	default:
		runErr = fmt.Errorf("unknown tool: %s", opts.Tool)
	}
	if runErr != nil {
		return runErr
	}

	emitTool(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(fingerprints, findings))
}

func saveToolRun(scanStore *store.Store, opts ToolRunOptions) {
	snapshot, _ := json.Marshal(map[string]any{
		"tool":        opts.Tool,
		"mode":        opts.Mode,
		"target":      opts.Target,
		"url":         opts.URL,
		"ports":       opts.Ports,
		"tags":        opts.Tags,
		"template":    opts.Template,
		"use_native":  opts.UseNativeArgs,
		"native_args": opts.NativeArgs,
	})
	_ = scanStore.SaveScanRun(store.ScanRun{
		RunID:          opts.RunID,
		ProjectID:      opts.ProjectID,
		Target:         firstNonEmpty(opts.Target, opts.URL, strings.Join(opts.NativeArgs, " ")),
		Ports:          opts.Ports,
		Profile:        "tool:" + opts.Tool,
		Status:         "running",
		StartedAt:      time.Now(),
		ConfigSnapshot: string(snapshot),
	})
}

func runNativeTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]report.Finding, error) {
	binary, err := nativeToolBinary(opts)
	if err != nil {
		return nil, err
	}
	emitTool(opts, scanStore, "info", opts.Tool, "%s", displayCommand(binary, opts.NativeArgs))
	out, err := runner.Run(ctx, binary, opts.NativeArgs)
	output := string(out)
	if strings.TrimSpace(output) != "" {
		emitTool(opts, scanStore, "info", opts.Tool, "%s", strings.TrimRight(output, "\n"))
	}
	if err != nil {
		return nil, normalizeToolError(ctx, err)
	}
	finding := report.Finding{
		Source:   opts.Tool,
		ID:       "native-output",
		Severity: "info",
		Summary:  opts.Tool + " native output",
		Target:   strings.Join(opts.NativeArgs, " "),
		Output:   output,
	}
	if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
		return nil, err
	}
	return []report.Finding{finding}, nil
}

func displayCommand(binary string, args []string) string {
	parts := []string{binary}
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'\\") {
			arg = strconv.Quote(arg)
		}
		parts = append(parts, arg)
	}
	return strings.Join(parts, " ")
}

func nativeToolBinary(opts ToolRunOptions) (string, error) {
	switch opts.Tool {
	case "rustscan":
		return opts.Tools.Rustscan, nil
	case "nmap":
		return opts.Tools.Nmap, nil
	case "httpx":
		return opts.Tools.Httpx, nil
	case "nuclei":
		return opts.Tools.Nuclei, nil
	default:
		return "", fmt.Errorf("unknown tool: %s", opts.Tool)
	}
}

func runRustscanTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]fingerprint.ServiceFingerprint, error) {
	if strings.TrimSpace(opts.Target) == "" || strings.TrimSpace(opts.Ports) == "" {
		return nil, errors.New("rustscan requires target and ports")
	}
	emitTool(opts, scanStore, "info", "rustscan", "rustscan %s ports=%s", opts.Target, opts.Ports)
	ports, err := tools.DiscoverPorts(ctx, runner, opts.Tools.Rustscan, opts.Target, opts.Ports, opts.ExtraArgs.Rustscan)
	if err != nil {
		return nil, normalizeToolError(ctx, err)
	}

	out := make([]fingerprint.ServiceFingerprint, 0, len(ports))
	for _, port := range ports {
		fp := fingerprint.ServiceFingerprint{IP: opts.Target, Port: port, Protocol: "tcp"}
		if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
			return nil, err
		}
		out = append(out, fp)
	}
	return out, nil
}

func runNmapTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
	if strings.TrimSpace(opts.Target) == "" {
		return nil, nil, errors.New("nmap requires target")
	}
	if opts.Mode == "alive" {
		alive, err := tools.CheckAlive(ctx, runner, opts.Tools.Nmap, opts.Target, opts.ExtraArgs.Nmap)
		if err != nil {
			return nil, nil, normalizeToolError(ctx, err)
		}
		summary := "Host did not respond"
		if alive {
			summary = "Host is alive"
		}
		finding := report.Finding{IP: opts.Target, Source: "nmap", ID: "host-alive", Severity: "info", Summary: summary, Target: opts.Target, Output: summary}
		if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
			return nil, nil, err
		}
		return nil, []report.Finding{finding}, nil
	}

	ports, err := parsePortCSV(opts.Ports)
	if err != nil {
		return nil, nil, err
	}
	fps, err := tools.Fingerprint(ctx, runner, opts.Tools.Nmap, opts.Target, ports, opts.ExtraArgs.Nmap)
	if err != nil {
		return nil, nil, normalizeToolError(ctx, err)
	}
	var findings []report.Finding
	for _, fp := range fps {
		if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
			return nil, nil, err
		}
		for _, finding := range ManualReviewFindings(fp) {
			if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
				return nil, nil, err
			}
			findings = append(findings, finding)
		}
	}
	return fps, findings, nil
}

func runHTTPXTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
	fp, err := fingerprintFromURL(opts.URL)
	if err != nil {
		return nil, nil, err
	}
	result, err := tools.EnrichWeb(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
	if err != nil {
		return nil, nil, normalizeToolError(ctx, err)
	}
	if result.URL != "" {
		fp.URL = result.URL
	}
	fp.IsWeb = true
	fp.Product = strings.Join(result.Tech, ", ")
	fp.Normalized = fp.Product
	if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
		return nil, nil, err
	}

	finding := report.Finding{
		IP: fp.IP, Port: fp.Port, Source: "httpx", ID: "web-fingerprint", Severity: "info",
		Summary: result.Title, Target: fp.URL,
		Output: fmt.Sprintf("status=%d tech=%s title=%s", result.StatusCode, strings.Join(result.Tech, ","), result.Title),
	}
	if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
		return nil, nil, err
	}
	return []fingerprint.ServiceFingerprint{fp}, []report.Finding{finding}, nil
}

func runNucleiTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]report.Finding, error) {
	fp, err := fingerprintFromURL(opts.URL)
	if err != nil {
		return nil, err
	}

	var out []byte
	if strings.TrimSpace(opts.Template) != "" {
		out, err = tools.RunNucleiTemplate(ctx, runner, opts.Tools.Nuclei, opts.URL, opts.Template, opts.ExtraArgs.Nuclei)
	} else {
		if len(opts.Tags) == 0 {
			return nil, errors.New("nuclei requires tags or template")
		}
		out, err = tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, opts.URL, opts.Tags, nil, opts.ExtraArgs.Nuclei)
	}
	if err != nil {
		return nil, normalizeToolError(ctx, err)
	}

	parsed, err := tools.ParseNucleiJSONL(out)
	if err != nil {
		return nil, err
	}
	findings := make([]report.Finding, 0, len(parsed))
	for _, result := range parsed {
		finding := findingFromNuclei(result, fp, nil)
		if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	return findings, nil
}

func fingerprintFromURL(raw string) (fingerprint.ServiceFingerprint, error) {
	if strings.TrimSpace(raw) == "" {
		return fingerprint.ServiceFingerprint{}, errors.New("url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fingerprint.ServiceFingerprint{}, err
	}
	host := parsed.Hostname()
	if host == "" {
		return fingerprint.ServiceFingerprint{}, errors.New("url requires host")
	}
	port := parsed.Port()
	if port == "" && parsed.Scheme == "https" {
		port = "443"
	}
	if port == "" {
		port = "80"
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fingerprint.ServiceFingerprint{}, err
	}
	return fingerprint.Classify(fingerprint.ServiceFingerprint{IP: host, Port: portNum, Protocol: "tcp", Service: "http", IsWeb: true, URL: raw}), nil
}

func parsePortCSV(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("nmap service mode requires ports")
	}
	parts := strings.Split(value, ",")
	ports := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid port: %s", part)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func emitTool(opts ToolRunOptions, scanStore *store.Store, level string, stage string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if opts.Logf != nil {
		opts.Logf("%s", message)
	}
	_ = scanStore.AppendScanEvent(store.ScanEvent{
		RunID:   opts.RunID,
		Time:    time.Now(),
		Level:   level,
		Stage:   stage,
		Message: message,
	})
}
