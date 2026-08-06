package scripts

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenArchivePatterns lists archive members the release archive must
// never contain: third-party scanner binaries (AnchorScan resolves them on
// PATH at runtime), SQLite/log/customer artifacts (no state ships with the
// release), and legacy fingerprint data (FingerprintHub/Web never enters the
// repo). The catalog copy is also forbidden by the single-source KB design.
//
// Patterns are matched as case-insensitive substrings against the tar member
// path. They are intentionally broad so a nested scanner binary or a renamed
// SQLite file is still caught.
var forbiddenArchivePatterns = []string{
	"catalog.json",
	"nmap", "nuclei", "rustscan", "httpx", "masscan", "naabu",
	".db", ".sqlite", ".sqlite3",
	".log",
	"reports/", "evidence/", "fingerprints/", "findings/",
	"fingerprinthub", "web-fingerprint",
}

// archiveMembers lists the tar member paths of a .tar.gz archive.
func archiveMembers(t *testing.T, archivePath string) []string {
	t.Helper()
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatalf("open gzip stream: %v", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var members []string
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return members
		}
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}
		members = append(members, header.Name)
	}
}

// findForbiddenMembers returns the archive members matching a forbidden
// pattern, paired with the pattern that matched. Empty when the archive is
// clean.
func findForbiddenMembers(members []string) []forbiddenMatch {
	var hits []forbiddenMatch
	for _, member := range members {
		name := strings.ToLower(filepath.ToSlash(member))
		for _, pattern := range forbiddenArchivePatterns {
			if strings.Contains(name, pattern) {
				hits = append(hits, forbiddenMatch{member: member, pattern: pattern})
			}
		}
	}
	return hits
}

type forbiddenMatch struct {
	member  string
	pattern string
}
