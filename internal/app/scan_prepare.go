package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/target"
)

type PrepareScanRequest struct {
	ConfigPath     string
	TargetSpec     string
	ExcludeTargets string
	PortSpec       string
	ExcludePorts   string
	DiscoveryMode  string
	RunID          string
	ProjectID      string
	ZoneID         string
	DBPath         string
	JSONReportPath string
	ArtifactRoot   string
	Overrides      config.Overrides
	Logf           func(format string, args ...any)
}

type PreparedScan struct {
	Options   ScanOptions
	Preflight preflight.Result
}

type scanConfigSnapshot struct {
	target.Snapshot
	DiscoveryMode string `json:"discovery_mode"`
}

func PrepareScan(req PrepareScanRequest) (PreparedScan, error) {
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	scope, err := target.ParseScope(req.TargetSpec, req.ExcludeTargets)
	if err != nil {
		return PreparedScan{}, err
	}

	discoveryMode, err := normalizeDiscoveryMode(req.DiscoveryMode)
	if err != nil {
		return PreparedScan{}, err
	}
	targets := scope.NmapTargets()
	scopeSnapshot, err := json.Marshal(scanConfigSnapshot{
		Snapshot:      scope.Snapshot(),
		DiscoveryMode: discoveryMode,
	})
	if err != nil {
		return PreparedScan{}, err
	}

	portSpec := req.PortSpec
	if portSpec == "" {
		portSpec = cfg.Scan.Ports
	}
	resolvedPorts, err := ports.ResolveForConfig(portSpec, req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}
	// fathom -p accepts explicit port lists/ranges only (no preset names), so
	// the "top1000" preset is expanded to its CSV form here; exclusions still
	// apply afterwards. Preflight keeps the user's original expression
	// ("top1000") in Summary.PortSpec for display.
	if resolvedPorts == "top1000" {
		resolvedPorts, err = ports.LoadPresetForConfig("top1000", req.ConfigPath)
		if err != nil {
			return PreparedScan{}, fmt.Errorf("load top1000 port preset: %w", err)
		}
	}
	resolvedPorts, err = ports.ExcludeForConfig(resolvedPorts, req.ExcludePorts, req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	effective, err := config.ResolveScan(cfg, req.Overrides)
	if err != nil {
		return PreparedScan{}, err
	}
	timeouts, err := cfg.Timeouts.Durations()
	if err != nil {
		return PreparedScan{}, err
	}
	nseRules, nseRulesErr := config.LoadNSERulesForConfig(req.ConfigPath)
	tagRules, tagRulesErr := config.LoadTagRulesForConfig(req.ConfigPath)
	configDir := filepath.Dir(req.ConfigPath)
	rulePaths := []string{
		filepath.Join(configDir, "nse.yaml"),
		filepath.Join(configDir, "service-tags.yaml"),
	}

	toolPaths := cfg.Tools
	extraArgs := effective.ToolArgs
	if err := config.ValidateScopeSafeToolArgs(extraArgs); err != nil {
		return PreparedScan{}, err
	}
	preflightResult := preflight.Run(preflight.Options{
		ConfigDir:     filepath.Dir(req.ConfigPath),
		DBPath:        req.DBPath,
		JSONPath:      req.JSONReportPath,
		ReportDir:     filepath.Dir(req.JSONReportPath),
		Targets:       targets,
		TargetCount:   int(scope.EstimatedAddresses()),
		PortSpec:      portSpec,
		Tools:         toolPaths,
		Profile:       effective.ProfileName,
		Workers:       effective.HostWorkers,
		ExtraArgs:     extraArgs,
		Timeouts:      cfg.Timeouts,
		NSERuleCount:  len(nseRules),
		TagRuleCount:  len(tagRules),
		NSERulesError: nseRulesErr,
		TagRulesError: tagRulesErr,
	})
	prepared := PreparedScan{Preflight: preflightResult}
	if preflightResult.HasErrors() {
		return prepared, nil
	}

	prepared.Options = ScanOptions{
		RunID:          req.RunID,
		ProjectID:      req.ProjectID,
		ZoneID:         req.ZoneID,
		Targets:        targets,
		Scope:          scope,
		Ports:          resolvedPorts,
		Tools:          toolPaths,
		ProfileName:    effective.ProfileName,
		HostWorkers:    effective.HostWorkers,
		ExtraArgs:      extraArgs,
		Timeouts:       timeouts,
		DiscoveryMode:  discoveryMode,
		ConfigSnapshot: string(scopeSnapshot),
		RulePaths:      rulePaths,
		JSONReportPath: req.JSONReportPath,
		ArtifactRoot:   req.ArtifactRoot,
		NSERules:       nseRules,
		TagRules:       tagRules,
		Logf:           req.Logf,
	}
	return prepared, nil
}
