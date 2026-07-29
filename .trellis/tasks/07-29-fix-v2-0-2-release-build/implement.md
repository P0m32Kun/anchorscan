# Implementation Plan

1. Add a focused regression check for the workflow archive-path contract if an existing lightweight test seam exists; otherwise validate the workflow expression directly and retain the package-integration reproduction as evidence.
2. Change `.github/workflows/release.yml` so `ANCHORSCAN_PACKAGE_ARCHIVE` is rooted at `${{ github.workspace }}`.
3. Change `scripts/package_smoke_test.go` so native Windows test binaries use `.exe`.
4. Format Go code if changed.
5. Run focused validation:
   - package integration test with an absolute existing archive path;
   - `TestBuildVersionCanBeInjected` on the host;
   - cross-compile/check the Windows output naming logic where locally possible;
   - validate workflow YAML/expression structure with the repository's available checker.
6. Run the relevant broader release/package checks required by project specs.
7. Perform independent code review and address findings.
8. Apply the selected release recovery strategy only after the fix commit exists; do not rewrite `v2.0.2` without explicit approval.

## Risky Files / Rollback Points

- `.github/workflows/release.yml`: a malformed expression blocks all release smoke jobs. Keep the edit to the environment value only.
- `scripts/package_smoke_test.go`: suffix handling must affect both injected-version and development-version temporary binaries.

## Completion Gate

- Local deterministic archive-path reproduction passes with the fixed absolute-path contract.
- Host package/version tests pass.
- The reviewed workflow is ready to run from the selected new or recreated tag.
