// Package version holds the single source of truth for the AnchorScan version.
// Both the CLI (--version) and the Web Console footer consume this constant so
// a release bump only needs to touch it once.
package version

// Version is the development fallback. Release builds override it with the
// release tag through Go's linker (-X).
var Version = "dev"
