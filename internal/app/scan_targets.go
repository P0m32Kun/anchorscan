package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type targetResult struct {
	target       string
	fingerprints []fingerprint.ServiceFingerprint
	findings     []report.Finding
	err          error
}

func scanTargets(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding
	scanTargets := opts.Targets

	if opts.Tools.Nmap != "" && len(scanTargets) > 0 {
		emit(opts, scanStore, "info", "nmap", "nmap alive sweep targets=%v", scanTargets)
		aliveTargets, out, err := tools.DiscoverAliveWithOutput(ctx, runner, opts.Tools.Nmap, scanTargets, opts.ExtraArgs.Nmap)
		if _, writeErr := writeArtifact(artifactDir, "nmap-alive-targets.xml", out); writeErr != nil {
			return nil, nil, writeErr
		}
		if err != nil {
			return nil, nil, normalizeToolError(ctx, err)
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
			return nil, nil, canceledErr
		}
		if failed == len(scanTargets) {
			return nil, nil, fmt.Errorf("all targets failed: %w", firstErr)
		}
	}

	return allFingerprints, allFindings, nil
}
