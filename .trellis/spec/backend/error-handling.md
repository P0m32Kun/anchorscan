# Backend Error Handling

Return actionable errors from configuration and app boundaries, then map stable client errors in `internal/web/`. Do not hide missing sidecars or invalid discovery modes as empty scan data. Handler tests use `httptest` (for example `internal/web/verifications_test.go`) to lock status/body behavior.
