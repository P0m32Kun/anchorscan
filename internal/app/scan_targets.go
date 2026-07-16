package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type targetResult struct {
	target string
	scan   TargetScan
	err    error
}

func scanTargets(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, artifactDir string, progress Progress) ([]TargetScan, []string, error) {
	var aliveIPs []string
	scanTargets := opts.Targets

	if opts.Tools.Nmap != "" && len(scanTargets) > 0 {
		progress.Emit("info", "nmap", "nmap alive sweep targets=%v", scanTargets)
		aliveTargets, out, err := tools.DiscoverAliveWithOutput(ctx, runner, opts.Tools.Nmap, scanTargets, nil)
		if _, writeErr := writeArtifact(artifactDir, "nmap-alive-targets.xml", out); writeErr != nil {
			return nil, nil, writeErr
		}
		if err != nil {
			return nil, nil, normalizeToolError(ctx, err)
		}
		scanTargets = aliveTargets
		aliveIPs = append([]string(nil), scanTargets...)
		progress.Emit("info", "nmap", "nmap alive hosts=%v", scanTargets)
		if len(scanTargets) == 0 {
			progress.Emit("info", "target", "no live hosts discovered; skip port scan")
		}
	}

	totalTargets := len(scanTargets)
	if totalTargets > 0 {
		progress.Emit("info", "progress", "progress 0/%d done=0 failed=0", totalTargets)
	}

	workers := opts.HostWorkers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(scanTargets) {
		workers = len(scanTargets)
	}

	var scans []TargetScan
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
					ts, err := scanTarget(ctx, runner, opts, target, artifactDir, progress)
					results <- targetResult{target: target, scan: ts, err: err}
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
		var done int
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
				done++
				if firstErr == nil {
					firstErr = result.err
				}
				failedTargets = append(failedTargets, result)
				progress.Emit("info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
				continue
			}
			// Per-target persistence: each target's fingerprints/findings land as
			// soon as that target completes, preserving the incremental behavior
			// the live report view relies on (results are read by ip/port order,
			// not insertion order, so grouping by target is observation-equivalent).
			if err := persistTargetScan(scanStore, opts.RunID, result.scan); err != nil {
				failed++
				done++
				if firstErr == nil {
					firstErr = err
				}
				failedTargets = append(failedTargets, targetResult{target: result.target, err: err})
				progress.Emit("info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
				continue
			}
			done++
			scans = append(scans, result.scan)
			progress.Emit("info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
		}
		for _, result := range failedTargets {
			progress.Emit("error", "target", "target %s failed: %v", result.target, result.err)
		}
		if canceledErr != nil {
			return nil, nil, canceledErr
		}
		if failed == len(scanTargets) {
			return nil, nil, fmt.Errorf("all targets failed: %w", firstErr)
		}
	}

	return scans, aliveIPs, nil
}

// persistTargetScan writes one target's fingerprints and findings to the store.
// It is the persistence seam that previously lived inline inside scanTarget;
// pulling it out keeps scanTarget a pure pipeline with no *store.Store dependency.
func persistTargetScan(scanStore *store.Store, runID string, scan TargetScan) error {
	if scanStore == nil {
		return nil
	}
	for _, fp := range scan.Fingerprints {
		if err := scanStore.SaveFingerprint(runID, fp); err != nil {
			return err
		}
	}
	for _, finding := range scan.Findings {
		if err := scanStore.SaveFinding(runID, finding); err != nil {
			return err
		}
	}
	return nil
}
