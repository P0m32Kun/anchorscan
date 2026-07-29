# Frontend Type Safety

Use TypeScript interfaces at public DTO boundaries and normalize malformed optional fields in `workbench-api.ts`. Preserve backend snake_case fields in wire objects; only convert deliberately at a named boundary. `workbench-api.test.mjs` covers normalization behavior.
