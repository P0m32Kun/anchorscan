# Backend Development Guidelines

AnchorScan backend code lives in `cmd/anchorscan/` and `internal/`. Preserve the domain language in [`CONTEXT.md`](../../../CONTEXT.md) and read applicable decisions in [`docs/adr/`](../../../docs/adr/).

## Pre-Development Checklist

- [ ] Read `CONTEXT.md`, relevant ADRs, and [`docs/testing-strategy.md`](../../../docs/testing-strategy.md).
- [ ] Choose the lowest sufficient seam: Go unit test, `httptest`, store integration, browser smoke, or Docker E2E.
- [ ] Trace the affected boundary through `internal/app`, `internal/store`, `internal/web`, and report code before changing a cross-layer contract.
- [ ] For behavioral work, record TDD Red → Green evidence before self-check and independent Standards/Spec review.

## Quality Check

- Run `make test` for normal backend changes; run `go vet ./...` when Go code changes.
- Run `make pr-check` before a PR; use `make e2e` only when real scanner collaboration is the risk.
- Use `code-review` from the fixed point against the authoritative ticket/spec after the write-capable self-check.

## Guides

| Guide | Use when |
| --- | --- |
| [Directory structure](./directory-structure.md) | placing packages or cross-layer code |
| [Error handling](./error-handling.md) | returning or mapping failures |
| [Database](./database-guidelines.md) | SQLite schema, transactions, or persistence |
| [Logging](./logging-guidelines.md) | recording operational failures |
| [Quality](./quality-guidelines.md) | selecting tests and review gates |
| [Runtime contracts](./scan-runtime-contracts.md) | scan/report compatibility |
