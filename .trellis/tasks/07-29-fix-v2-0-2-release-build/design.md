# Technical Design

## Boundary

This repair is limited to release smoke-test wiring and its cross-platform package test. It does not change application runtime behavior, archive contents, matrix targets, or release permissions.

## Changes

### Archive location contract

`actions/download-artifact` extracts the selected archive under the workflow workspace, while `go test ./scripts` executes tests with the package directory as the process working directory. The workflow must therefore pass `ANCHORSCAN_PACKAGE_ARCHIVE` as an absolute path:

```text
${{ github.workspace }}/dist/anchorscan-${{ github.ref_name }}-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz
```

The test remains able to accept any caller-provided absolute path; its fallback package-building behavior remains unchanged.

### Native executable naming contract

Temporary binaries built and executed by `TestBuildVersionCanBeInjected` must use the native executable suffix. On Windows both temporary output paths end in `.exe`; other platforms remain unchanged. Reuse a minimal local naming expression/helper only if it removes duplication without expanding the public API.

## Compatibility and Risk

- GitHub expression interpolation into `env` supports Windows workspace paths; Go's `filepath`/`os.Open` accept the resulting absolute path.
- The archive remains `.tar.gz` on every target; no packaging behavior changes.
- The Windows fix affects test paths only.
- Rerunning the old tag workflow cannot test a later fix commit. Release recovery must use the user-selected tag strategy.

## Rollback

Both edits are isolated. Revert the workflow path and test suffix changes if validation reveals a platform-specific incompatibility.
