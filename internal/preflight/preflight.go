package preflight

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/ports"
)

type Options struct {
	ConfigDir    string
	DBPath       string
	JSONPath     string
	ReportDir    string
	Targets      []string
	PortSpec     string
	Tools        app.ToolPaths
	Profile      string
	Workers      int
	ExtraArgs    app.ToolExtraArgs
	NSERuleCount int
	TagRuleCount int
}

type Result struct {
	Summary  Summary
	Warnings []Message
	Errors   []Message
}

type Summary struct {
	TargetCount   int
	Targets       []string
	PortSpec      string
	ResolvedPorts string
	Profile       string
	Workers       int
	Tools         app.ToolPaths
	ExtraArgs     app.ToolExtraArgs
	NSERuleCount  int
	TagRuleCount  int
}

type Message struct {
	Field   string
	Message string
}

func (r Result) HasErrors() bool {
	return len(r.Errors) > 0
}

func Run(opts Options) Result {
	result := Result{
		Summary: Summary{
			TargetCount:  len(opts.Targets),
			Targets:      append([]string(nil), opts.Targets...),
			PortSpec:     opts.PortSpec,
			Profile:      opts.Profile,
			Workers:      opts.Workers,
			Tools:        opts.Tools,
			ExtraArgs:    opts.ExtraArgs,
			NSERuleCount: opts.NSERuleCount,
			TagRuleCount: opts.TagRuleCount,
		},
	}

	if len(opts.Targets) == 0 {
		result.Errors = append(result.Errors, Message{Field: "target", Message: "no targets"})
	}

	if strings.TrimSpace(opts.PortSpec) == "" {
		result.Errors = append(result.Errors, Message{Field: "ports", Message: "ports is empty"})
	} else if resolved, err := ports.Resolve(opts.PortSpec, opts.ConfigDir); err != nil {
		result.Errors = append(result.Errors, Message{Field: "ports", Message: err.Error()})
	} else if err := validatePorts(resolved); err != nil {
		result.Errors = append(result.Errors, Message{Field: "ports", Message: err.Error()})
	} else {
		result.Summary.ResolvedPorts = resolved
	}

	if result.Summary.ResolvedPorts == "1-65535" {
		result.Warnings = append(result.Warnings, Message{Field: "ports", Message: "full range scan may be slow"})
	}
	if opts.Profile == "fast" && len(opts.Targets) > 16 {
		result.Warnings = append(result.Warnings, Message{Field: "profile", Message: "fast profile with many targets may increase load"})
	}

	checkRequiredTool(&result, "rustscan", opts.Tools.Rustscan)
	checkRequiredTool(&result, "nmap", opts.Tools.Nmap)
	checkOptionalTool(&result, "httpx", opts.Tools.Httpx)
	checkOptionalTool(&result, "nuclei", opts.Tools.Nuclei)

	checkWritableParent(&result, "database", opts.DBPath)
	checkWritableParent(&result, "json", opts.JSONPath)
	if strings.TrimSpace(opts.ReportDir) != "" {
		checkWritableDir(&result, "reports", opts.ReportDir)
	}

	return result
}

func checkRequiredTool(result *Result, name string, path string) {
	if err := executablePath(path); err != nil {
		result.Errors = append(result.Errors, Message{Field: name, Message: err.Error()})
	}
}

func checkOptionalTool(result *Result, name string, path string) {
	if strings.TrimSpace(path) == "" {
		result.Warnings = append(result.Warnings, Message{Field: name, Message: "path is empty"})
		return
	}
	if err := executablePath(path); err != nil {
		result.Warnings = append(result.Warnings, Message{Field: name, Message: err.Error()})
	}
}

func executablePath(path string) error {
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
	if info.Mode()&0o111 == 0 {
		return errors.New("not executable")
	}
	return nil
}

func checkWritableParent(result *Result, name string, path string) {
	if strings.TrimSpace(path) == "" {
		result.Errors = append(result.Errors, Message{Field: name, Message: "path is empty"})
		return
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		result.Errors = append(result.Errors, Message{Field: name, Message: err.Error()})
	}
}

func checkWritableDir(result *Result, name string, path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		result.Errors = append(result.Errors, Message{Field: name, Message: err.Error()})
	}
}

func validatePorts(value string) error {
	if value == "top1000" {
		return nil
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return fmt.Errorf("invalid port expression: %q", value)
		}
		if strings.Contains(part, "-") {
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 || !validPort(bounds[0]) || !validPort(bounds[1]) {
				return fmt.Errorf("invalid port expression: %q", value)
			}
			continue
		}
		if !validPort(part) {
			return fmt.Errorf("invalid port expression: %q", value)
		}
	}
	return nil
}

func validPort(value string) bool {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && port >= 1 && port <= 65535
}
