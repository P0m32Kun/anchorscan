# Diagnose scanner rule discovery and web build regression

## Goal

Restore a buildable Web Console and ensure packaged AnchorScan releases actually load their detection rules, so recognized services execute the intended NSE/nuclei checks instead of silently completing with every check skipped.

## Background and Confirmed Facts

- The exact affected run is `run-20260728-104816.622007000` in `/Users/kun/NARI/2026/07/宁夏/anchorscan-v2.0.1-darwin-arm64/data/scans.sqlite`.
- Its fingerprints are correctly normalized: SSH as `ssh`, X11 as `x11`, and Apache Tomcat as Web service `http`. The run persisted `skipped/no_matching_rule` for both NSE and nuclei on SSH, X11 and Tomcat (Tomcat NSE additionally explains that Web services do not run NSE).
- The release directory contains `config/default.yaml`, `default.yaml.example`, and a manually supplied `ports-highrisk.txt`, but it is missing `config/nse.yaml` and `config/service-tags.yaml`.
- `Makefile`'s `package` recipe copies only `config/default.yaml.example`; it omits all other tracked runtime resources: `nse.yaml`, `service-tags.yaml`, `ports-highrisk.txt`, `ports-top100.txt`, and `ports-top1000.txt`. GitHub release archives are built through this same recipe.
- `internal/config/loadRuleFileForConfig` suppresses `os.ErrNotExist` for every candidate and returns an empty ruleset. `PrepareScan` then accepts zero rules, while `doctor` explicitly reports missing rule files as `ok`. This explains the exact observed all-skipped result and makes the packaging failure invisible before scanning.
- The bundled source rules already cover SSH and Tomcat. The local nuclei template set also confirms `javascript/misconfiguration/x11/x11-unauth-access.yaml` has tag `x11`; the bundled `service-tags.yaml` lacks the corresponding `x11` routing rule.
- The Web build failure is independent but also reproducible: `internal/web/scans.go` references `scanForm.DiscoveryMode` and `app.PrepareScanRequest.DiscoveryMode`, while those types do not declare it. `cmd/anchorscan/scan_command.go` also declares an unused `discoveryFlag`.
- Cross-session history shows the intended discovery contract was already implemented and reviewed on local branch `agent/harden-release-and-scan-trust` (`fd47bae`, `760d864`): modes are `auto` and `assume-up`, default is `auto`, and the value flows through CLI, Web, rerun snapshots, execution, and reports. The current main working tree contains only fragments of that previously approved implementation.
- DetectionCheck history remains an audit fact governed by ADR-0003; historical runs must not be rewritten after fixing packaging or rules.

## Requirements

1. Package every tracked runtime config resource needed by a standalone release, and make the package gate fail if any required resource is absent.
2. Treat `nse.yaml` and `service-tags.yaml` as mandatory runtime resources. Scan preparation must fail immediately with an actionable error when either file is absent or unusable, and `doctor` must report the same condition as a failure rather than healthy.
3. Add a safe X11 nuclei routing rule using normalized service `x11`, nuclei tag `x11`, and a `hostport` target. Do not add an X11 NSE script in this task.
4. Establish a deterministic regression test that loads rules from a packaged/config-directory shape (not hand-constructed `ScanOptions`) and proves SSH invokes both configured engines, Tomcat routes to nuclei, and X11 routes to nuclei.
5. Complete only the already-approved discovery-mode slice needed to repair the current partial changes: `auto` / `assume-up` validation, CLI and Web propagation, execution behavior, rerun snapshot compatibility, and report visibility. Do not pull unrelated heartbeat, event polling, process-group cancellation, or other Ticket 08 changes into this task.
6. Preserve intentional behavior: Web services do not run NSE; X11 gains nuclei-only coverage; zero findings after a completed tool execution remain a valid result.
7. Do not mutate the affected historical run or its persisted DetectionChecks.

## Acceptance Criteria

- [ ] `go run ./cmd/anchorscan web` builds and starts past the former `DiscoveryMode` compile errors.
- [ ] `make package VERSION=test` produces an archive containing `config/default.yaml.example`, `nse.yaml`, `service-tags.yaml`, `ports-highrisk.txt`, `ports-top100.txt`, and `ports-top1000.txt`; removal/omission of a required resource makes the package gate fail.
- [ ] Packaged/config-directory rule loading returns non-empty NSE and nuclei rule sets and routes an SSH fingerprint to `ssh2-enum-algos,ssh-hostkey` plus nuclei `-tags ssh`.
- [ ] An X11 fingerprint normalized as `x11` routes to nuclei `-tags x11` with target `IP:port` and does not gain an NSE probe.
- [ ] A Tomcat Web fingerprint still skips NSE by design and routes to Tomcat nuclei tags.
- [ ] Missing or unusable `nse.yaml` / `service-tags.yaml` makes scan preparation fail immediately with an actionable error, and `doctor` no longer prints misleading `nse rules: ok` / `tag rules: ok` for absent files.
- [ ] `--discovery auto|assume-up` and the Web/rerun equivalent propagate consistently; invalid values are rejected; `assume-up` skips only the alive sweep and retains authorized targets.
- [ ] Focused tests, relevant package suites, build/package verification, and final review pass; temporary debug instrumentation is absent.

## Out of Scope

- Re-running or rewriting the July 28 historical scan.
- Treating a completed zero-finding detection execution as an error.
- Adding broad/brute-force templates or an X11 NSE probe.
- Pulling unrelated changes from the hardening worktree.
- Redesigning the detection-rule system or embedding all rules into the binary.

## Approved Strictness Decision

Missing `nse.yaml` or `service-tags.yaml` is a fatal configuration error. AnchorScan must not create a successful-looking scan that cannot route vulnerability checks. A future explicitly designed fingerprint-only mode may relax this contract; deleting mandatory rule files is not a supported way to enable such a mode.
