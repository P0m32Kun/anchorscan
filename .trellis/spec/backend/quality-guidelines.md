# Backend Quality Guidelines

Use the smallest test seam that proves the risk; the full matrix is in [`docs/testing-strategy.md`](../../../docs/testing-strategy.md). Pure rules belong in Go tests, HTTP mapping in `httptest`, SQLite behavior in store/integration tests, and scanner collaboration in `make e2e` only when fixtures cannot prove it.

## Required workflow

Behavioral work follows TDD Red → smallest Green → write-capable self-check → read-only Standards review → read-only Spec/AC review → full verification → PR. `trellis-check` is not independent review.

Run `make test` and `go vet ./...` for Go changes, then `make pr-check` before delivery. Use `make e2e` for Docker/real scanner behavior and record why it is required or not applicable.

## Do not

- Do not bypass `app.PrepareScan` when constructing scan options; it is the shared runtime boundary.
- Do not silently turn missing configuration/rules into empty results; see [scan-runtime-contracts](./scan-runtime-contracts.md).
- Do not rewrite persisted DetectionCheck history after rules change.

Representative references: `internal/app/scan_prepare.go`, `internal/store/sqlite.go`, and `internal/web/verifications_test.go`.
