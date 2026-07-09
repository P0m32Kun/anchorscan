// Package version holds the single source of truth for the AnchorScan version.
// Both the CLI (--version) and the Web Console footer consume this constant so
// a release bump only needs to touch it once.
package version

// Version is the current AnchorScan release. Keep it in sync with the latest
// git tag (e.g. "1.5.1" for tag v1.5.1) and the CHANGELOG.md entry.
const Version = "1.5.1"
