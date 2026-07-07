# Task 4 Report: Persist Projects, Scan Runs, And Scan Events

## What I implemented
- Added store models for `Project`, `ScanRun`, and `ScanEvent` in `/Users/kun/DEV/new-Anchor/internal/store/models.go`.
- Extended the SQLite schema in `/Users/kun/DEV/new-Anchor/internal/store/sqlite.go` with the required `projects`, `scan_runs`, and `scan_events` tables exactly as specified in the brief.
- Added project persistence methods in `/Users/kun/DEV/new-Anchor/internal/store/projects.go`:
  - `SaveProject`
  - `GetProject`
  - `ListProjects`
  - `DeleteProject`
- Added scan run and scan event persistence methods in `/Users/kun/DEV/new-Anchor/internal/store/runs.go`:
  - `SaveScanRun`
  - `GetScanRun`
  - `ListScanRuns`
  - `ListProjectScanRuns`
  - `UpdateScanRunStatus`
  - `AppendScanEvent`
  - `ListScanEvents`
- Used `time.RFC3339Nano` for persisted timestamps, with empty strings for zero-value times so the required `TEXT NOT NULL` columns stay valid while preserving unset timestamps.

## What I tested and results
### Focused store tests
Command:
```bash
rtk go test ./internal/store -run 'TestStoreProjectCRUD|TestStoreScanRunsAndEvents' -count=1
```
Result:
- RED run failed first because the new types and store methods did not exist yet.
- GREEN run passed after implementation.

### Full store package
Command:
```bash
rtk go test ./internal/store -count=1
```
Result:
- PASS (`Go test: 4 passed in 1 packages`)

### Full repository suite
Command:
```bash
rtk go test ./... -count=1
```
Result:
- PASS (`Go test: 42 passed in 10 packages`)

## TDD evidence
### RED
Command:
```bash
rtk go test ./internal/store -run 'TestStoreProjectCRUD|TestStoreScanRunsAndEvents' -count=1
```
Output:
```text
Go test: 0 passed, 1 failed in 1 packages

store [build failed]
  internal/store/sqlite_test.go:75:13: undefined: Project
  internal/store/sqlite_test.go:85:18: store.SaveProject undefined (type *Store has no field or method SaveProject)
  internal/store/sqlite_test.go:89:25: store.ListProjects undefined (type *Store has no field or method ListProjects)
  internal/store/sqlite_test.go:99:18: store.SaveProject undefined (type *Store has no field or method SaveProject)
  internal/store/sqlite_test.go:103:20: store.GetProject undefined (type *Store has no field or method GetProject)
  internal/store/sqlite_test.go:111:18: store.DeleteProject undefined (type *Store has no field or method DeleteProject)
  internal/store/sqlite_test.go:115:24: store.ListProjects undefined (type *Store has no field or method ListProjects)
  internal/store/sqlite_test.go:130:9: undefined: ScanRun
  internal/store/sqlite_test.go:140:18: store.SaveScanRun undefined (type *Store has no field or method SaveScanRun)
  internal/store/sqlite_test.go:143:18: store.UpdateScanRunStatus undefined (type *Store has no field or method UpdateScanRunStatus)
```

### GREEN
Command:
```bash
rtk go test ./internal/store -run 'TestStoreProjectCRUD|TestStoreScanRunsAndEvents' -count=1
```
Output:
```text
Go test: 2 passed in 1 packages
```

## Files changed
- `/Users/kun/DEV/new-Anchor/internal/store/models.go`
- `/Users/kun/DEV/new-Anchor/internal/store/projects.go`
- `/Users/kun/DEV/new-Anchor/internal/store/runs.go`
- `/Users/kun/DEV/new-Anchor/internal/store/sqlite.go`
- `/Users/kun/DEV/new-Anchor/internal/store/sqlite_test.go`

## Self-review findings
- Scope stayed inside `internal/store` as requested.
- Schema matches the brief exactly: no extra tables, foreign keys, or indexes were added.
- Tests cover the required project CRUD flow plus run/event save, update, fetch, and list behavior.
- Timestamp handling is intentionally simple and local: format on write, parse on read.

## Concerns
- None at the store layer. The only notable choice is representing unset `finished_at` values as empty strings because the brief requires `TEXT NOT NULL`; this keeps the schema exact and the code simple.

## Reviewer follow-up fix: mixed-precision run ordering
- Verified the review finding against the current implementation: `started_at` was stored with `time.RFC3339Nano`, which omits trailing fractional zeros, so lexicographic `ORDER BY started_at DESC` could misorder runs within the same second.
- Fixed this by changing persisted timestamp formatting to a fixed-width UTC layout: `2006-01-02T15:04:05.000000000Z07:00`.
- Kept the brief's schema shape intact: `started_at` and `finished_at` remain `TEXT NOT NULL`, and the list queries still order by `started_at DESC`.
- Added a focused regression test covering one whole-second timestamp and one fractional-second timestamp, asserting correct order in both `ListScanRuns` and `ListProjectScanRuns`.

### Reviewer fix TDD evidence
#### RED
Command:
```bash
rtk go test ./internal/store -run TestStoreListsScanRunsByChronologicalStartedAtWithMixedPrecision -count=1
```
Output:
```text
Go test: 0 passed, 1 failed in 1 packages

store (0 passed, 1 failed)
  [FAIL] TestStoreListsScanRunsByChronologicalStartedAtWithMixedPrecision
     sqlite_test.go:232: unexpected global run order: []store.ScanRun{store.ScanRun{RunID:"run-whole-second", ...}}
```

#### GREEN
Command:
```bash
rtk go test ./internal/store -run TestStoreListsScanRunsByChronologicalStartedAtWithMixedPrecision -count=1
```
Output:
```text
Go test: 1 passed in 1 packages
```

### Reviewer fix verification
Command:
```bash
rtk go test ./internal/store -count=1
rtk go test ./... -count=1
```
Result:
- PASS (`Go test: 5 passed in 1 packages`)
- PASS (`Go test: 43 passed in 10 packages`)
