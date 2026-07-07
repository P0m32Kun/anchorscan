# Task 6 Report: Add Host Worker Concurrency And Target Error Isolation

## What I implemented
- Added bounded host-level concurrency in `/Users/kun/DEV/new-Anchor/internal/app/scan.go` using a small worker pool driven by `ScanOptions.HostWorkers`.
- Preserved the existing per-target tool order inside a new `scanTarget(...)` helper:
  - rustscan -> nmap -> httpx/NSE/nuclei -> report aggregation
- Added per-target failure isolation:
  - target-local failures no longer abort the whole run immediately
  - successful targets still persist fingerprints/findings and contribute to the final report
  - failed targets emit a run event with `level=error` and `stage=target`
- Kept run-level semantics aligned with the brief:
  - return a run-level error only when every target fails
  - still return a run-level error if final report writing fails
  - otherwise complete the run and write the report from successful targets

## What I tested and results
### Focused RED/GREEN tests
- `go test ./internal/app -run 'TestRunScanRespectsHostWorkers|TestRunScanContinuesAfterTargetFailure' -count=1`
  - RED: failed as expected after fixing the worker-limit test shape
    - `TestRunScanRespectsHostWorkers`: `expected max active 2, got 1`
    - `TestRunScanContinuesAfterTargetFailure`: `RunScan returned error: boom`
  - GREEN: passed after the implementation change

### Package tests
- `go test ./internal/app -count=1`
  - PASS

### Full suite
- `go test ./...`
  - PASS

## TDD evidence (RED/GREEN)
### RED
1. Added `TestRunScanRespectsHostWorkers` to prove host concurrency is actually used.
   - I initially copied the brief's shape too literally and got an `EOF`, which proved the test double was wrong rather than the production behavior.
   - I corrected the test double so it returned valid rustscan/nmap data and made the assertion check real concurrency (`HostWorkers: 2`, expecting `maxActive == 2`).
   - Verified failure: `expected max active 2, got 1`.
2. Added `TestRunScanContinuesAfterTargetFailure`.
   - Verified failure: `RunScan returned error: boom`.

### GREEN
1. Extracted the existing single-target scan flow into `scanTarget(...)` without changing target-local ordering.
2. Added a minimal worker pool in `RunScan`.
3. Aggregated successful target results while collecting failed target errors.
4. Emitted target error events after workers completed so the event persistence stays reliable under concurrent target processing.
5. Re-ran the focused tests and got PASS.

## Files changed
- `/Users/kun/DEV/new-Anchor/internal/app/scan.go`
- `/Users/kun/DEV/new-Anchor/internal/app/scan_test.go`
- `/Users/kun/DEV/new-Anchor/.superpowers/sdd/task-6-report.md`

## Self-review findings
- The diff stays in the two owned app-layer files plus this report.
- The concurrency change is intentionally small: one extracted helper, one worker pool, one result type.
- Per-target tool order is preserved because all target-local work still runs serially inside `scanTarget(...)`.
- Run completion semantics remain unchanged for cancellation and report-write failure because `RunScan` still returns the underlying error in those cases.
- I did not add manager/web changes or new abstractions beyond what the task needed.

## Concerns
- The new tests cover the required worker-bound behavior and partial-failure continuation path.
- I did not add a separate explicit regression test for the "all targets fail" branch because the task-required RED/GREEN coverage already exercised the new worker/isolation behaviors and the implementation for the all-failed branch is a small conditional on the collected results.
