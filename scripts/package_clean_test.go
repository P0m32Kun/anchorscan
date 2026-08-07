package scripts

import (
	"testing"
)

// Ticket 08 hardening: the release archive purity gate must catch every
// forbidden artifact — third-party scanner binaries, SQLite/log state,
// customer artifacts, and legacy fingerprint data — not just the catalog copy.

func TestFindForbiddenMembersCatchesAllCategories(t *testing.T) {
	members := []string{
		"anchorscan-1.0.0-darwin-arm64/anchorscan",
		"anchorscan-1.0.0-darwin-arm64/config/default.yaml.example",
		"anchorscan-1.0.0-darwin-arm64/config/nse.yaml",
		"anchorscan-1.0.0-darwin-arm64/docs/deploy.md",
		"anchorscan-1.0.0-darwin-arm64/tools/docx-render/render_docx.py",
		// forbidden:
		"anchorscan-1.0.0-darwin-arm64/bin/nmap",
		"anchorscan-1.0.0-darwin-arm64/vendor/nuclei-templates/foo.yaml",
		"anchorscan-1.0.0-darwin-arm64/data/scan.db",
		"anchorscan-1.0.0-darwin-arm64/var/server.log",
		"anchorscan-1.0.0-darwin-arm64/reports/run-x.html",
		"anchorscan-1.0.0-darwin-arm64/evidence/v1/shot.png",
		"anchorscan-1.0.0-darwin-arm64/legacy/fingerprinthub/services.txt",
		"anchorscan-1.0.0-darwin-arm64/config/catalog.json",
	}
	hits := findForbiddenMembers(members)
	if len(hits) == 0 {
		t.Fatal("expected forbidden members to be flagged, got none")
	}
	got := map[string]bool{}
	for _, hit := range hits {
		got[hit.member] = true
	}
	for _, want := range []string{
		"anchorscan-1.0.0-darwin-arm64/bin/nmap",
		"anchorscan-1.0.0-darwin-arm64/vendor/nuclei-templates/foo.yaml",
		"anchorscan-1.0.0-darwin-arm64/data/scan.db",
		"anchorscan-1.0.0-darwin-arm64/var/server.log",
		"anchorscan-1.0.0-darwin-arm64/reports/run-x.html",
		"anchorscan-1.0.0-darwin-arm64/evidence/v1/shot.png",
		"anchorscan-1.0.0-darwin-arm64/legacy/fingerprinthub/services.txt",
		"anchorscan-1.0.0-darwin-arm64/config/catalog.json",
	} {
		if !got[want] {
			t.Errorf("forbidden member not flagged: %q", want)
		}
	}
}

func TestFindForbiddenMembersAllowsCleanArchive(t *testing.T) {
	members := []string{
		"anchorscan-1.0.0-darwin-arm64/anchorscan",
		"anchorscan-1.0.0-darwin-arm64/config/default.yaml.example",
		"anchorscan-1.0.0-darwin-arm64/config/nse.yaml",
		"anchorscan-1.0.0-darwin-arm64/config/service-tags.yaml",
		"anchorscan-1.0.0-darwin-arm64/config/ports-highrisk.txt",
		"anchorscan-1.0.0-darwin-arm64/config/ports-top1000.txt",
		"anchorscan-1.0.0-darwin-arm64/docs/README.md",
		"anchorscan-1.0.0-darwin-arm64/docs/deploy.md",
		"anchorscan-1.0.0-darwin-arm64/tools/docx-render/render_docx.py",
		"anchorscan-1.0.0-darwin-arm64/tools/docx-render/pyproject.toml",
		"anchorscan-1.0.0-darwin-arm64/tools/docx-render/templates/project-report.docx",
	}
	if hits := findForbiddenMembers(members); len(hits) != 0 {
		t.Fatalf("clean archive must produce no forbidden hits, got %+v", hits)
	}
}

func TestForbiddenPatternsIncludeStateAndLegacyData(t *testing.T) {
	// The catalog pattern is covered elsewhere; this guards the broader set
	// that the acceptance checklist calls out explicitly.
	want := map[string]bool{
		"nmap": true, "nuclei": true, "httpx": true,
		".db": true, ".sqlite": true, ".log": true,
		"reports/": true, "evidence/": true, "fingerprints/": true,
		"fingerprinthub": true,
	}
	for _, pattern := range forbiddenArchivePatterns {
		delete(want, pattern)
	}
	for missing := range want {
		t.Errorf("forbidden archive patterns missing required entry: %q", missing)
	}
}
