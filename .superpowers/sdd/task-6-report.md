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

---

## Fix pass for reviewer findings (after commit 027b2ab)

### What I fixed
1. Restored the worker-limit regression test to the brief's intended shape.
   - `TestRunScanRespectsHostWorkers` now uses `HostWorkers: 1` with two targets.
   - It fails if `maxActive > 1`.
2. Fixed multi-target cancellation semantics in `/Users/kun/DEV/new-Anchor/internal/app/scan.go`.
   - The target feeder now stops on `ctx.Done()` instead of enqueueing the rest of the run.
   - `context.Canceled` from any worker is now treated as a run-level cancellation, not an ordinary target-local failure.
3. Added direct regression coverage for the all-targets-failed rule.
   - `TestRunScanReturnsErrorWhenAllTargetsFail` now asserts a run-level error is returned.

### Root cause
- The worker feeder goroutine always pushed every target into the queue, even after the run context was canceled.
- The result collector counted `context.Canceled` the same way as a normal target error, so a partial success could incorrectly let a canceled multi-target run finish as success.
- The original Task 6 worker test had drifted from the review requirement and no longer proved the concurrency bound itself.

### TDD evidence for the fix pass
- RED:
  - Ran `go test ./internal/app -run 'TestRunScanRespectsHostWorkers|TestRunScanContinuesAfterTargetFailure|TestRunScanMarksCanceledWhenContextCanceled|TestRunScanReturnsErrorWhenAllTargetsFail' -count=1`
  - First run failed to compile because the new cancellation assertion needed the `errors` import.
  - After fixing the test compile issue, the focused run failed for the intended product reason:
    - `TestRunScanMarksCanceledWhenContextCanceled`: `expected only one target start before cancellation, got 2 calls`
- GREEN:
  - Updated the feeder to `select` on `ctx.Done()` while sending targets.
  - Short-circuited `context.Canceled` as a run-level return path.
  - Re-ran the focused test command and it passed.

### Fix-pass verification
- `go test ./internal/app -run 'TestRunScanRespectsHostWorkers|TestRunScanContinuesAfterTargetFailure|TestRunScanMarksCanceledWhenContextCanceled|TestRunScanReturnsErrorWhenAllTargetsFail' -count=1`
  - PASS
- `go test ./internal/app -count=1`
  - PASS
- `go test ./...`
  - PASS

### Files changed in the fix pass
- `/Users/kun/DEV/new-Anchor/internal/app/scan.go`
- `/Users/kun/DEV/new-Anchor/internal/app/scan_test.go`

### Self-review
- Stayed inside Task 6 scope and only touched the two owned app files plus this append-only report.
- Preserved the existing per-target execution order; the fix only changes queue feeding and result classification for cancellation.
- The cancellation regression test now covers both required behaviors in one path: no post-cancel target dispatch and run-level canceled return.

### Remaining concerns
- None specific to Task 6 scope.
