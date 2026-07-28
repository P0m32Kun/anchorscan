// Package version holds the AnchorScan build version shared by the CLI and Web
// Console. Release builds override Version through the Go linker.
package version

// Version identifies a local source build when no release value is injected.
var Version = "dev"
