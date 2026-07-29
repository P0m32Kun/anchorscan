# Database Guidelines

SQLite persistence belongs in `internal/store/` behind explicit store methods. Preserve Run Lease and DetectionCheck historical facts; migrations and transaction cleanup require store/integration tests. See `internal/store/sqlite.go` and ADR-0002/0003.
