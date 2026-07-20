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
type ToolTimeouts = config.ToolDurations

type TagRule = vuln.TagRule

type ScanOptions struct {
	RunID                string
	LeaseOwnerToken      string
	ProjectID            string
	Targets              []string
	Ports                string
	Tools                ToolPaths
	ProfileName          string
	HostWorkers          int
	ExtraArgs            ToolExtraArgs
	Timeouts             ToolTimeouts
	ConfigSnapshot       string
	JSONReportPath       string
	ArtifactRoot         string
	NSERules             map[string][]string
	TagRules             []TagRule
	PersistFingerprint   func(fingerprint.ServiceFingerprint) error
	PersistFinding       func(report.Finding) error
	RecordDetectionCheck func(store.DetectionCheck) error
	Logf                 func(format string, args ...any)
}

func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) (runErr error) {
	artifactDir := ""
	partialErrors := false

	if opts.ProfileName == "" {
		opts.ProfileName = "normal"
	}
	ctx, finishLease, abortLease, err := acquireRunLease(ctx, scanStore, opts.RunID, opts.LeaseOwnerToken)
	if err != nil {
		return err
	}
	runSaved := false
	defer func() {
		if !runSaved {
			abortLease()
			return
		}
		status := "completed"
		message := ""
		if partialErrors {
			status = "completed_with_errors"
			message = "one or more optional stages failed"
		}
		if runErr != nil {
			status = "failed"
			message = runErr.Error()
			if errors.Is(runErr, context.Canceled) {
				status = "canceled"
			}
		}
		finishLease(status, message, time.Now())
	}()
	if opts.RunID != "" && strings.TrimSpace(opts.ArtifactRoot) != "" {
		artifactDir = filepath.Join(opts.ArtifactRoot, opts.RunID)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return err
		}
	}
	if opts.RunID != "" && scanStore != nil {
		if err := scanStore.SaveScanRun(store.ScanRun{
			RunID:          opts.RunID,
			ProjectID:      opts.ProjectID,
			Target:         strings.Join(opts.Targets, ","),
			Ports:          opts.Ports,
			Profile:        opts.ProfileName,
			Status:         "running",
			StartedAt:      time.Now(),
			ConfigSnapshot: opts.ConfigSnapshot,
			ArtifactDir:    artifactDir,
		}); err != nil {
			return err
		}
		runSaved = true
	}

	if opts.RecordDetectionCheck == nil && scanStore != nil {
		opts.RecordDetectionCheck = scanStore.UpsertDetectionCheck
	}
	if opts.PersistFingerprint == nil && scanStore != nil {
		opts.PersistFingerprint = func(fp fingerprint.ServiceFingerprint) error { return scanStore.UpsertFingerprint(opts.RunID, fp) }
	}
	if opts.PersistFinding == nil && scanStore != nil {
		opts.PersistFinding = func(finding report.Finding) error { return scanStore.SaveFinding(opts.RunID, finding) }
	}
	progress := storeProgress{runID: opts.RunID, log: opts.Logf, store: scanStore, now: time.Now}
	scans, aliveIPs, partial, err := scanTargets(ctx, runner, opts, artifactDir, progress)
	if err != nil {
		return err
	}
	partialErrors = partial

	allFingerprints, allFindings, openPorts := flattenScans(scans)
	var detectionChecks []report.DetectionCheck
	if scanStore != nil {
		checks, err := scanStore.ListDetectionChecks(opts.RunID)
		if err != nil {
			return err
		}
		for _, check := range checks {
			detectionChecks = append(detectionChecks, report.DetectionCheck{
				IP: check.IP, Port: check.Port, Protocol: check.Protocol, Engine: check.Engine,
				Status: check.Status, ReasonCode: check.ReasonCode, Detail: check.Detail,
				StartedAt: report.DetectionCheckTime(check.StartedAt), FinishedAt: report.DetectionCheckTime(check.FinishedAt),
			})
		}
	}
	progress.Emit("info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.BuildWithScanDataAndDetectionChecks(allFingerprints, allFindings, report.ScanData{
		AliveIPs:  aliveIPs,
		OpenPorts: openPorts,
	}, detectionChecks))
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
	if isOperatorCanceled(ctx) {
		return context.Canceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return err
}

func toolContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func isOperatorCanceled(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}
