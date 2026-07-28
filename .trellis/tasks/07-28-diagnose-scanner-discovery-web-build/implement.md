# Implementation Plan

## Preconditions

- Preserve all pre-existing dirty worktree changes; inspect each touched file before editing.
- Do not cherry-pick the broad Ticket 08 commits. Use them only as a reference and implement the narrow discovery slice defined in `design.md`.
- Do not edit the external July 28 release directory or its SQLite database.

## 1. Capture the two red feedback loops

- [ ] Re-run the focused Go build/test command and retain the exact `DiscoveryMode` compile failures.
- [ ] Query the affected SQLite run and confirm SSH, Tomcat, and X11 still show persisted `skipped/no_matching_rule` checks.
- [ ] Run the current package recipe and prove the archive lacks the mandatory runtime manifest.
- [ ] Record the commands/results in the task journal; do not add permanent debug logging.

## 2. Add fail-closed rule-loading tests (red)

- [ ] Extend `internal/config/config_test.go` to require errors for missing and empty `nse.yaml` / `service-tags.yaml`, while preserving local-sidecar and root fallback success.
- [ ] Extend `internal/doctor/doctor_test.go` so absent, malformed, and empty rule files are failed checks, not `ok`.
- [ ] Run only these tests and confirm the new assertions fail for the expected reason.

## 3. Implement mandatory rule resources (green)

- [ ] Change `internal/config/rules.go` to return actionable errors when all candidates are absent and reject empty rule sets.
- [ ] Change `internal/doctor/doctor.go` to use the same `Load*ForConfig` contract as scan preparation.
- [ ] Keep candidate search order and `errors.Is(..., os.ErrNotExist)` behavior useful to callers.
- [ ] Run focused config and doctor tests.

## 4. Add package-manifest regression and fix packaging

- [ ] Define the explicit runtime config manifest in `Makefile`.
- [ ] Copy all six baseline files without wildcard-copying an operator's `default.yaml`.
- [ ] Add staging and tar-content assertions so an absent/empty required file or archive omission fails `make package`.
- [ ] Run `make package VERSION=test`, list the archive entries, and verify every required file is present.
- [ ] Negative-check the gate in an isolated temporary copy or controlled command without deleting/modifying the user's source files.

## 5. Add X11 routing tests and rule

- [ ] Extend default-rule coverage tests to expect `x11` with nuclei tag `x11`, `hostport` target, and no NSE mapping.
- [ ] Confirm the assertion is red before editing the YAML.
- [ ] Add the minimal safe rule to `config/service-tags.yaml`.
- [ ] Run config/vulnerability matcher tests and confirm no global brute-force/default-login tag was introduced.

## 6. Add configuration-loaded execution regression

- [ ] Add a package-shaped temporary config fixture that copies/loads the real rule sidecars through `PrepareScan`.
- [ ] Use a recording fake runner at the existing app integration seam; do not call live scanners or network targets.
- [ ] Assert SSH invokes configured NSE scripts and nuclei tag `ssh`.
- [ ] Assert Tomcat Web skips NSE by policy and invokes Tomcat nuclei routing.
- [ ] Assert X11 invokes nuclei tag `x11` against `IP:port` and has no NSE invocation.
- [ ] Assert completed zero-finding invocations persist `completed` DetectionChecks.
- [ ] Assert missing sidecars fail before the runner is called.
- [ ] Run the focused app test and refactor only duplicated test setup that materially improves clarity.

## 7. Add discovery-mode tests (red)

- [ ] Add `PrepareScan` tests for empty/default `auto`, explicit `assume-up`, and invalid-value rejection.
- [ ] Add `scanTargets` runner-order tests proving `auto` runs alive discovery and `assume-up` begins at the port scan while retaining the parsed/excluded target list.
- [ ] Add CLI test coverage for `--discovery` propagation and invalid values.
- [ ] Add Web tests for request parsing, validation, render default, snapshot persistence, and old-snapshot rerun default.
- [ ] Add report model/rebuild tests for visible discovery mode and historical `auto` fallback.
- [ ] Run focused tests and confirm failures correspond to missing implementation, not fixture errors.

## 8. Implement the narrow discovery-mode slice (green)

- [ ] Add centralized discovery constants/default/validation and historical snapshot parsing in `internal/app`.
- [ ] Add `DiscoveryMode` fields to `PrepareScanRequest`, `ScanOptions`, Web form snapshot, and report data/model.
- [ ] Pass CLI and Web values through preparation and execution; fix the current compile errors.
- [ ] In `scanTargets`, branch only around the alive sweep; `assume-up` uses the already authorized `opts.Targets` and sets `aliveIPs` consistently.
- [ ] Add Web form controls/defaulting and preserve the mode on rerun.
- [ ] Expose the effective mode in JSON/HTML and report rebuilding, defaulting old snapshots to `auto` without rewriting storage.
- [ ] Do not import unrelated Ticket 08 progress, cancellation, process, polling, scope, or heartbeat changes.
- [ ] Run focused app, CLI, Web, and report tests until green.

## 9. Full verification

- [ ] Run `gofmt` on changed Go files and the repository's existing frontend formatter/build only where required.
- [ ] Run LSP diagnostics on every changed supported source file.
- [ ] Run focused suites: `go test ./internal/config ./internal/doctor ./internal/vuln ./internal/app ./internal/report ./internal/web ./cmd/anchorscan`.
- [ ] Run frontend static tests/build relevant to modified Vue/template files.
- [ ] Run `go test ./...` and `node --test internal/web/static/*.test.mjs`.
- [ ] Run `make package VERSION=test` and inspect archive contents.
- [ ] Build/start `go run ./cmd/anchorscan web` with a controlled local config and bounded smoke timeout; confirm it passes the former compile/start failure.
- [ ] Re-query the external historical database read-only and confirm no rows changed.

## 10. Review and completion gates

- [ ] Dispatch an independent `trellis-check` / code review with the active-task prefix and task artifacts.
- [ ] Review specifically for: silent fallback paths, accidental wildcard packaging, unsafe nuclei tags, excluded-target reintroduction, snapshot compatibility, and DetectionCheck state changes.
- [ ] Apply review fixes and repeat affected verification.
- [ ] Update `.trellis/spec/` only with durable contracts learned here: mandatory release runtime resources and fail-closed rule loading.
- [ ] Present final diff/verification summary before the repository's required commit/archive steps.

## Rollback points

- After steps 3-4: strict rule loading and packaging can be reverted independently of discovery mode.
- After steps 5-6: X11/routing coverage is YAML/test-local.
- After step 8: discovery changes are additive fields plus one execution branch; no database rollback is required.
