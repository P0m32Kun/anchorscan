# Frontend Hook Guidelines

This Vue codebase primarily uses Composition API primitives (`ref`, `computed`, lifecycle hooks) in components rather than a separate custom-hook layer. Keep request/normalization helpers pure and importable, as in `workbench-api.ts`; do not hide server payload casing in ad-hoc component casts.
