package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

type ToolPaths = config.ToolPaths
type ToolExtraArgs = config.ToolArgs

type TagRule = vuln.TagRule

type ScanOptions struct {
	RunID          string
	ProjectID      string
	Targets        []string
	Ports          string
	Tools          ToolPaths
	ProfileName    string
	HostWorkers    int
	ExtraArgs      ToolExtraArgs
	ConfigSnapshot string
	JSONReportPath string
	ArtifactRoot   string
	NSERules       map[string][]string
	TagRules       []TagRule
	Logf           func(format string, args ...any)
}

func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) (runErr error) {
	artifactDir := ""

	if opts.ProfileName == "" {
		opts.ProfileName = "normal"
	}
	ctx, releaseLease, err := acquireRunLease(ctx, scanStore, opts.RunID)
	if err != nil {
		return err
	}
	defer releaseLease()
	if opts.RunID != "" && strings.TrimSpace(opts.ArtifactRoot) != "" {
		artifactDir = filepath.Join(opts.ArtifactRoot, opts.RunID)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return err
		}
	}
	if opts.RunID != "" && scanStore != nil {
		_ = scanStore.SaveScanRun(store.ScanRun{
			RunID:          opts.RunID,
			ProjectID:      opts.ProjectID,
			Target:         strings.Join(opts.Targets, ","),
			Ports:          opts.Ports,
			Profile:        opts.ProfileName,
			Status:         "running",
			StartedAt:      time.Now(),
			ConfigSnapshot: opts.ConfigSnapshot,
			ArtifactDir:    artifactDir,
		})
	}
	defer func() {
		if opts.RunID == "" || scanStore == nil {
			return
		}
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

	progress := storeProgress{runID: opts.RunID, log: opts.Logf, store: scanStore, now: time.Now}
	scans, aliveIPs, err := scanTargets(ctx, runner, scanStore, opts, artifactDir, progress)
	if err != nil {
		return err
	}

	allFingerprints, allFindings, openPorts := flattenScans(scans)
	progress.Emit("info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.BuildWithScanData(allFingerprints, allFindings, report.ScanData{
		AliveIPs:  aliveIPs,
		OpenPorts: openPorts,
	}))
}

func logf(opts ScanOptions, format string, args ...any) {
	if opts.Logf != nil {
		opts.Logf(format, args...)
	}
}

// flattenScans reduces a slice of per-target TargetScans into the flat shape the
// report builder expects: all fingerprints, all findings, and open ports keyed
// by host. It mirrors the accumulation that previously lived inside scanTargets.
func flattenScans(scans []TargetScan) ([]fingerprint.ServiceFingerprint, []report.Finding, map[string][]int) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding
	openPortsByHost := map[string][]int{}
	for _, scan := range scans {
		allFingerprints = append(allFingerprints, scan.Fingerprints...)
		allFindings = append(allFindings, scan.Findings...)
		if len(scan.OpenPorts) > 0 {
			openPortsByHost[scan.Target] = scan.OpenPorts
		}
	}
	return allFingerprints, allFindings, openPortsByHost
}

func normalizeToolError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return context.Canceled
	}
	return err
}
