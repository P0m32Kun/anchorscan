# v2.0.2 Release Build Failure Root Cause

## Evidence

- Latest failing run: `30374031929` (`https://github.com/P0m32Kun/anchorscan/actions/runs/30374031929`).
- The lab and all three build matrix jobs succeeded.
- All three native smoke jobs failed in `Smoke test native release archive`; the final release job was skipped.
- Linux and macOS failed at `scripts/package_smoke_test.go:46` because the test tried to open `dist/anchorscan-v2.0.2-<os>-<arch>.tar.gz` from the Go package working directory (`scripts/`), not from the repository root where `actions/download-artifact` placed it.
- Windows had the same archive-path failure and also failed `TestBuildVersionCanBeInjected`: the test built an executable named `anchorscan` without the `.exe` suffix, which Windows `exec` could not launch.
- Downloading artifact `anchorscan-linux-amd64` from the failed run confirms that it contains the correctly named file `anchorscan-v2.0.2-linux-amd64.tar.gz`; the build artifact itself is valid.

## Local Red-Capable Loop

The archive-path defect reproduces deterministically in about one second:

```bash
ANCHORSCAN_PACKAGE_ARCHIVE=dist/anchorscan-v9.8.7-darwin-arm64.tar.gz \
ANCHORSCAN_PACKAGE_NAME=anchorscan-v9.8.7-darwin-arm64 \
ANCHORSCAN_PACKAGE_VERSION=v9.8.7 \
go test -tags packageintegration ./scripts \
  -run '^TestPackageArchiveIncludesRuntimeResources$' -count=1 -v
```

Observed failure:

```text
open archive: open dist/anchorscan-v9.8.7-darwin-arm64.tar.gz: no such file or directory
```

## Minimal Repair

1. In the smoke workflow, pass an absolute archive path rooted at `${{ github.workspace }}`.
2. In `TestBuildVersionCanBeInjected`, add `.exe` to temporary binary paths when `runtime.GOOS == "windows"`.
3. Keep the archive format and build matrix unchanged.

## Release Recovery Constraint

The existing `v2.0.2` tag points at the failing workflow commit. A fix committed after that tag will not be included by merely rerunning the old workflow. Recovery therefore requires either:

- preferred: publish a new immutable patch tag (for example `v2.0.3`); or
- exceptional: delete/recreate or force-move `v2.0.2`, which rewrites a public release reference.
