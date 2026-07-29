# Frontend Directory Structure

Vue entry components and request helpers live in `internal/web/frontend/`; static browser behavior tests live beside frontend/static files. Keep API normalization in `workbench-api.ts` and user flow state in the owning component, such as `Workbench.vue`.
