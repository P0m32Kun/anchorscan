# Backend Directory Structure

`cmd/anchorscan/` owns CLI wiring. `internal/app/` orchestrates domain workflows, `internal/store/` owns SQLite persistence, `internal/web/` owns HTTP/HTML boundaries, `internal/report/` builds delivery views, and `internal/tools/` wraps external processes. Keep orchestration out of handlers and persistence details out of `app`.

Example: `internal/app/scan_prepare.go` prepares shared ScanOptions; handlers call app boundaries rather than duplicating validation.
