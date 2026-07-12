# Release Single Publisher Design

## Goal

Eliminate the GitHub Release creation race caused by every platform build
publishing the same tag concurrently.

## Scope

The existing build matrix remains Linux amd64, Darwin arm64, and Windows
amd64. Each matrix job will upload its one built binary as a GitHub Actions
artifact. A new `release` job will require the full build matrix, download all
artifacts, detect prerelease tags, and invoke `softprops/action-gh-release`
exactly once.

## Behavior

- A build failure prevents the `release` job from running, so no incomplete
  release is published.
- A successful build uploads all three files in one release action invocation.
- Prerelease detection and release-note generation retain their current
  behavior.

## Verification

Add a small repository-local shell check that asserts the workflow has one
release action, a matrix artifact upload, and a dependent release job. Run the
check before and after the workflow edit, then parse the workflow and run the
project test suite.
