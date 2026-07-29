# Frontend Quality Guidelines

Use Node tests for pure DTO/normalization behavior, handler tests for HTTP mapping, and Playwright smoke only where the browser is part of the contract. Follow [`docs/testing-strategy.md`](../../../docs/testing-strategy.md).

## Required workflow

Behavioral work follows TDD Red → smallest Green → write-capable self-check → read-only Standards review → read-only Spec/AC review → full verification → PR. Run `make test`, `npm run build:web`, and `make pr-check`; use `make e2e` only for real-tool behavior.

## Do not

- Do not invent alternate DTO casing: public association fields are snake_case.
- Do not key verification state by vulnerability key alone; use `(zone_id, vulnerability_key)`.
- Do not add browser-only test modes or production test endpoints.

Representative references: `internal/web/frontend/Workbench.vue`, `internal/web/frontend/workbench-api.ts`, and `internal/web/frontend/workbench-api.test.mjs`.
