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
	openPorts    []int
	err          error
}

func scanTargets(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, []string, map[string][]int, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding
	var aliveIPs []string
	openPortsByHost := map[string][]int{}
	scanTargets := opts.Targets

	if opts.Tools.Nmap != "" && len(scanTargets) > 0 {
		emit(opts, scanStore, "info", "nmap", "nmap alive sweep targets=%v", scanTargets)
		aliveTargets, out, err := tools.DiscoverAliveWithOutput(ctx, runner, opts.Tools.Nmap, scanTargets, nil)
		if _, writeErr := writeArtifact(artifactDir, "nmap-alive-targets.xml", out); writeErr != nil {
			return nil, nil, nil, nil, writeErr
		}
		if err != nil {
			return nil, nil, nil, nil, normalizeToolError(ctx, err)
		}
		scanTargets = aliveTargets
		aliveIPs = append([]string(nil), scanTargets...)
		emit(opts, scanStore, "info", "nmap", "nmap alive hosts=%v", scanTargets)
		if len(scanTargets) == 0 {
			emit(opts, scanStore, "info", "target", "no live hosts discovered; skip port scan")
		}
	}

	totalTargets := len(scanTargets)
	if totalTargets > 0 {
		emit(opts, scanStore, "info", "progress", "progress 0/%d done=0 failed=0", totalTargets)
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
					ts, err := scanTarget(ctx, runner, scanStore, opts, target, artifactDir)
					results <- targetResult{
						target:       target,
						fingerprints: ts.Fingerprints,
						findings:     ts.Findings,
						openPorts:    ts.OpenPorts,
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
				emit(opts, scanStore, "info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
				continue
			}
			done++
			allFingerprints = append(allFingerprints, result.fingerprints...)
			allFindings = append(allFindings, result.findings...)
			if len(result.openPorts) > 0 {
				openPortsByHost[result.target] = result.openPorts
			}
			emit(opts, scanStore, "info", "progress", "progress %d/%d done=%d failed=%d current=%s", done, totalTargets, done, failed, result.target)
		}
		for _, result := range failedTargets {
			emit(opts, scanStore, "error", "target", "target %s failed: %v", result.target, result.err)
		}
		if canceledErr != nil {
			return nil, nil, nil, nil, canceledErr
		}
		if failed == len(scanTargets) {
			return nil, nil, nil, nil, fmt.Errorf("all targets failed: %w", firstErr)
		}
	}

	return allFingerprints, allFindings, aliveIPs, openPortsByHost, nil
}
