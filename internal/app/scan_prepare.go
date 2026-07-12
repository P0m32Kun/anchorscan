package app

import (
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
	RunID          string
	ProjectID      string
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

func PrepareScan(req PrepareScanRequest) (PreparedScan, error) {
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	targets, err := target.Parse(req.TargetSpec)
	if err != nil {
		return PreparedScan{}, err
	}
	targets, err = target.Exclude(targets, req.ExcludeTargets)
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
	resolvedPorts, err = ports.ExcludeForConfig(resolvedPorts, req.ExcludePorts, req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	effective, err := config.ResolveScan(cfg, req.Overrides)
	if err != nil {
		return PreparedScan{}, err
	}
	nseRules, err := config.LoadNSERulesForConfig(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}
	tagRules, err := config.LoadTagRulesForConfig(req.ConfigPath)
	if err != nil {
		return PreparedScan{}, err
	}

	toolPaths := cfg.Tools
	extraArgs := effective.ToolArgs
	preflightResult := preflight.Run(preflight.Options{
		ConfigDir:    filepath.Dir(req.ConfigPath),
		DBPath:       req.DBPath,
		JSONPath:     req.JSONReportPath,
		ReportDir:    filepath.Dir(req.JSONReportPath),
		Targets:      targets,
		PortSpec:     portSpec,
		Tools:        toolPaths,
		Profile:      effective.ProfileName,
		Workers:      effective.HostWorkers,
		ExtraArgs:    extraArgs,
		NSERuleCount: len(nseRules),
		TagRuleCount: len(tagRules),
	})
	prepared := PreparedScan{Preflight: preflightResult}
	if preflightResult.HasErrors() {
		return prepared, nil
	}

	prepared.Options = ScanOptions{
		RunID:          req.RunID,
		ProjectID:      req.ProjectID,
		Targets:        targets,
		Ports:          resolvedPorts,
		Tools:          toolPaths,
		ProfileName:    effective.ProfileName,
		HostWorkers:    effective.HostWorkers,
		ExtraArgs:      extraArgs,
		JSONReportPath: req.JSONReportPath,
		ArtifactRoot:   req.ArtifactRoot,
		NSERules:       nseRules,
		TagRules:       tagRules,
		Logf:           req.Logf,
	}
	return prepared, nil
}
