package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
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

	allFingerprints, allFindings, err := scanTargets(ctx, runner, scanStore, opts, artifactDir)
	if err != nil {
		return err
	}

	emit(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(allFingerprints, allFindings))
}

func logf(opts ScanOptions, format string, args ...any) {
	if opts.Logf != nil {
		opts.Logf(format, args...)
	}
}

func emit(opts ScanOptions, scanStore *store.Store, level string, stage string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logf(opts, "%s", message)
	if opts.RunID == "" || scanStore == nil {
		return
	}
	_ = scanStore.AppendScanEvent(store.ScanEvent{
		RunID:   opts.RunID,
		Time:    time.Now(),
		Level:   level,
		Stage:   stage,
		Message: message,
	})
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
