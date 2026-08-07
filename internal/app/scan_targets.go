package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/target"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type targetResult struct {
	target string
	scan   TargetScan
	err    error
}

func scanTargets(ctx context.Context, runner tools.Runner, opts ScanOptions, artifactDir string, progress Progress) ([]TargetScan, []string, bool, error) {
	var aliveIPs []string
	var ipv6Alive []string // nmap -sn confirmed IPv6 hosts (fathom is IPv4-only)
	scope := opts.Scope
	if scope.IsZero() {
		var err error
		scope, err = target.ParseScope(strings.Join(opts.Targets, ","), "")
		if err != nil {
			return nil, nil, false, err
		}
	}
	opts.Scope = scope
	// fathom is IPv4-only (spec decision 5), so the nmap -sn sweep survives for
	// IPv6 scope parts only. nmap stays a hard requirement whenever the scope
	// contains IPv6; IPv4 CIDR/exclude scopes no longer need it for discovery
	// (fathom scan expands and probes them internally).
	if opts.Tools.Nmap == "" {
		for _, discoveryScope := range scope.DiscoveryScopes() {
			if discoveryScope.IsIPv6() {
				return nil, nil, false, errors.New("nmap is required for IPv6 scan targets")
			}
		}
	}
	targets := scope.NmapTargets()

	if opts.DiscoveryMode == DiscoveryAssumeUp && len(targets) > 0 {
		// assume-up semantics stay anchorscan-side: every scope address enters
		// scanTarget unprocessed. fathom still probes alive internally (ICMP +
		// TCP fallback); the mode only skips anchorscan-side preprocessing and
		// does not change the fathom invocation (no --no-icmp, no weaken).
		targets = scope.Addresses()
		aliveIPs = append([]string(nil), targets...)
		progress.Emit("info", "target", "assume-up: skip alive discovery, treat %d host(s) as up", len(targets))
	} else {
		// auto: IPv4 addresses go straight to scanTarget. `fathom scan` probes
		// alive internally (alive::find → ICMP Datagram/Raw + TCP fallback on
		// 80/443/445/22) and only emits hosts with open ports, so alive
		// filtering is built into the port scan itself — no separate sweep.
		// IPv6 keeps the nmap -sn sweep (fathom is IPv4-only).
		discovered := make([]string, 0, int(scope.EstimatedAddresses()))
		for _, discoveryScope := range scope.DiscoveryScopes() {
			if !discoveryScope.IsIPv6() {
				discovered = append(discovered, discoveryScope.Addresses()...)
				continue
			}
			progress.Emit("info", "nmap", "nmap alive sweep targets=%v", discoveryScope.NmapTargets())
			toolCtx, cancel := toolContext(ctx, opts.Timeouts.Nmap)
			aliveTargets, out, err := tools.DiscoverAliveInScopeWithOutput(toolCtx, runner, opts.Tools.Nmap, discoveryScope, nil)
			if _, writeErr := writeArtifact(artifactDir, "nmap-alive-ipv6.xml", out); writeErr != nil {
				cancel()
				return nil, nil, false, writeErr
			}
			if err != nil {
				normalized := normalizeToolError(toolCtx, err)
				cancel()
				return nil, nil, false, normalized
			}
			cancel()
			ipv6Alive = append(ipv6Alive, aliveTargets...)
			discovered = append(discovered, aliveTargets...)
		}
		targets = discovered
		progress.Emit("info", "target", "scan targets=%d (fathom alive probing is internal; nmap -sn kept for IPv6)", len(targets))
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

	if opts.DiscoveryMode != DiscoveryAssumeUp {
		// aliveIPs used to record the nmap -sn sweep result. With fathom doing
		// the IPv4 alive probing, the alive set is derived from scan results:
		// hosts fathom emitted fingerprints for (= alive with open ports, the
		// only hosts `fathom scan` ever outputs). IPv6 hosts were confirmed by
		// the retained nmap -sn sweep and count as alive even without
		// fingerprints.
		aliveIPs = aliveHostsFromResults(scans, ipv6Alive)
		if len(aliveIPs) == 0 {
			progress.Emit("info", "target", "no live hosts discovered")
		} else {
			progress.Emit("info", "target", "alive hosts=%d", len(aliveIPs))
		}
	}

	return scans, aliveIPs, partialErrors, nil
}

// aliveHostsFromResults derives the alive set after the per-target fan-out:
// hosts with fathom fingerprints (= alive with at least one open port, since
// `fathom scan` only outputs hosts whose port scan found open ports) plus the
// IPv6 hosts the nmap -sn sweep confirmed alive.
func aliveHostsFromResults(scans []TargetScan, ipv6Alive []string) []string {
	alive := make([]string, 0, len(scans)+len(ipv6Alive))
	seen := make(map[string]bool, len(scans)+len(ipv6Alive))
	for _, scan := range scans {
		if len(scan.Fingerprints) == 0 || seen[scan.Target] {
			continue
		}
		seen[scan.Target] = true
		alive = append(alive, scan.Target)
	}
	for _, ip := range ipv6Alive {
		if !seen[ip] {
			seen[ip] = true
			alive = append(alive, ip)
		}
	}
	return alive
}
