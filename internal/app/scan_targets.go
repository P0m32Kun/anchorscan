package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type targetResult struct {
	target string
	scan   TargetScan
	err    error
}

func scanTargets(ctx context.Context, runner tools.Runner, opts ScanOptions, artifactDir string, progress Progress) ([]TargetScan, []string, bool, error) {
	var aliveIPs []string
	targets := opts.Targets

	if opts.Tools.Nmap != "" && len(targets) > 0 {
		progress.Emit("info", "nmap", "nmap alive sweep targets=%v", targets)
		toolCtx, cancel := toolContext(ctx, opts.Timeouts.Nmap)
		aliveTargets, out, err := tools.DiscoverAliveWithOutput(toolCtx, runner, opts.Tools.Nmap, targets, nil)
		if _, writeErr := writeArtifact(artifactDir, "nmap-alive-targets.xml", out); writeErr != nil {
			cancel()
			return nil, nil, false, writeErr
		}
		if err != nil {
			normalized := normalizeToolError(toolCtx, err)
			cancel()
			return nil, nil, false, normalized
		}
		cancel()
		targets = aliveTargets
		aliveIPs = append([]string(nil), targets...)
		progress.Emit("info", "nmap", "nmap alive hosts=%v", targets)
		if len(targets) == 0 {
			progress.Emit("info", "target", "no live hosts discovered; skip port scan")
		}
	}

	totalTargets := len(targets)
	if totalTargets > 0 {
		progress.Emit("info", "progress", "progress 0/%d done=0 failed=0", totalTargets)
	}

	workers := opts.HostWorkers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(targets) {
		workers = len(targets)
	}

	var scans []TargetScan
	partialErrors := false
	if workers > 0 {
		targetCh := make(chan string)
		results := make(chan targetResult, len(targets))
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
			for _, target := range targets {
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
				partialErrors = true
				progress.Emit("info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
				continue
			}
			done++
			if result.scan.HadErrors {
				partialErrors = true
			}
			scans = append(scans, result.scan)
			progress.Emit("info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
		}
		for _, result := range failedTargets {
			progress.Emit("error", "target", "target %s failed: %v", result.target, result.err)
		}
		if canceledErr != nil {
			return nil, nil, partialErrors, canceledErr
		}
		if failed == len(targets) {
			return nil, nil, partialErrors, fmt.Errorf("all targets failed: %w", firstErr)
		}
	}

	return scans, aliveIPs, partialErrors, nil
}
