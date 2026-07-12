package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding
	scanTargets := opts.Targets
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

	if opts.Tools.Nmap != "" && len(scanTargets) > 0 {
		emit(opts, scanStore, "info", "nmap", "nmap alive sweep targets=%v", scanTargets)
		aliveTargets, out, err := tools.DiscoverAliveWithOutput(ctx, runner, opts.Tools.Nmap, scanTargets, opts.ExtraArgs.Nmap)
		if _, writeErr := writeArtifact(artifactDir, "nmap-alive-targets.xml", out); writeErr != nil {
			return writeErr
		}
		if err != nil {
			return normalizeToolError(ctx, err)
		}
		scanTargets = aliveTargets
		emit(opts, scanStore, "info", "nmap", "nmap alive hosts=%v", scanTargets)
		if len(scanTargets) == 0 {
			emit(opts, scanStore, "info", "target", "no live hosts discovered; skip port scan")
		}
	}

	workers := opts.HostWorkers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(scanTargets) {
		workers = len(scanTargets)
	}
	if workers > 0 {
		targetCh := make(chan string)
		results := make(chan targetResult, len(scanTargets))
		var wg sync.WaitGroup

		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for target := range targetCh {
					if ctx.Err() != nil {
						return
					}
					fingerprints, findings, err := scanTarget(ctx, runner, scanStore, opts, target, artifactDir)
					results <- targetResult{
						target:       target,
						fingerprints: fingerprints,
						findings:     findings,
						err:          err,
					}
				}
			}()
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		go func() {
			defer close(targetCh)
			for _, target := range scanTargets {
				select {
				case <-ctx.Done():
					return
				case targetCh <- target:
				}
			}
		}()

		var canceledErr error
		var failed int
		var failedTargets []targetResult
		var firstErr error
		for result := range results {
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) {
					if canceledErr == nil {
						canceledErr = result.err
					}
					continue
				}
				failed++
				if firstErr == nil {
					firstErr = result.err
				}
				failedTargets = append(failedTargets, result)
				continue
			}
			allFingerprints = append(allFingerprints, result.fingerprints...)
			allFindings = append(allFindings, result.findings...)
		}
		for _, result := range failedTargets {
			emit(opts, scanStore, "error", "target", "target %s failed: %v", result.target, result.err)
		}
		if canceledErr != nil {
			return canceledErr
		}
		if failed == len(scanTargets) {
			return fmt.Errorf("all targets failed: %w", firstErr)
		}
	}

	emit(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(allFingerprints, allFindings))
}

type targetResult struct {
	target       string
	fingerprints []fingerprint.ServiceFingerprint
	findings     []report.Finding
	err          error
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
