# Component Guidelines

Use Vue single-file components in `internal/web/frontend/`. Define typed props with `defineProps`, keep interaction state in `ref`, and derive display state with `computed` (see `RunDetail.vue`). Reuse `postJSON` and `getJSON` instead of copying request/error parsing.

Use native dialogs for dialogs, preserve focus and feedback where user-visible, and cover browser behavior with Playwright when that behavior is the contract. Do not introduce browser-only test modes or endpoints.

## Workbench Verification Dialogs

- Identify a verification by `(zone_id, vulnerability_key)`.
- New records create assets/sources, then upload evidence; existing records upload pending evidence then update metadata.
- Keep wire payload fields snake_case and use the shared helpers in `Workbench.vue`.
- Require at least one evidence image for a new verification and test the representative reopen/edit flow in browser smoke.
