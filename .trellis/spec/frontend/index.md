# Frontend Development Guidelines

The Vue 3 workbench is under `internal/web/frontend/`; its HTTP contract is implemented in `internal/web/`. Use the vocabulary in [`CONTEXT.md`](../../../CONTEXT.md).

## Pre-Development Checklist

- [ ] Read [`docs/testing-strategy.md`](../../../docs/testing-strategy.md) and the relevant backend handler/DTO before changing UI data flow.
- [ ] Keep server payload fields explicit and match the public snake_case contract (see `Workbench.vue` and `workbench-api.ts`).
- [ ] Select the lowest sufficient seam: Node unit test for pure normalization, `httptest` for HTTP mapping, Playwright only for browser behavior.
- [ ] For behavioral work, record TDD Red → Green, then self-check and independent Standards/Spec review.

## Quality Check

- Run `make test`; run `npm run build:web` for Vue/type changes.
- Run `make pr-check` before a PR; run `make e2e` only for real-tool behavior.
- Review browser changes with representative Playwright smoke and keep diagnostics actionable.

## Guides

| Guide | Use when |
| --- | --- |
| [Directory structure](./directory-structure.md) | adding modules or assets |
| [Components](./component-guidelines.md) | dialogs, props, accessibility |
| [Hooks](./hook-guidelines.md) | composables and request helpers |
| [State](./state-management.md) | local, derived, or server state |
| [Types](./type-safety.md) | DTO normalization and runtime boundaries |
| [Quality](./quality-guidelines.md) | tests, review, and delivery gates |
