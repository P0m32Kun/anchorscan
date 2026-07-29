# Frontend State Management

Prefer component-local `ref` for dialog and filter state and `computed` for derived display state. Server state is fetched through explicit helpers and normalized before storage. Verification identity is the composite `(zone_id, vulnerability_key)`, never the vulnerability key alone; see `Workbench.vue`.
