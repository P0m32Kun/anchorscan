package knowledgebase

import "testing"

func TestCatalogSearchReturnsIndependentEntries(t *testing.T) {
	catalog := newCatalog([]Entry{{
		ID:       "smb-signing",
		Name:     "SMB signing disabled",
		Severity: SeverityMedium,
		Aliases:  []string{"SMB 签名未启用"},
		Match:    MatchKeys{CVEs: []string{"CVE-2024-0001"}},
	}})

	entries := catalog.Search("sign")
	if len(entries) != 1 {
		t.Fatalf("Search() returned %d entries, want 1", len(entries))
	}
	entries[0].Aliases[0] = "changed"

	entry, ok := catalog.Entry("smb-signing")
	if !ok || entry.Aliases[0] != "SMB 签名未启用" {
		t.Fatalf("Catalog leaked mutable entry: %#v", entry)
	}
}

func TestCatalogMatchReturnsAmbiguousForSharedToolID(t *testing.T) {
	catalog := newCatalog([]Entry{
		{ID: "first", Match: MatchKeys{NucleiIDs: []string{"shared"}}},
		{ID: "second", Match: MatchKeys{NucleiIDs: []string{"shared"}}},
	})

	result := catalog.Match(Observation{Tool: ToolNuclei, ToolID: "shared"})
	if result.Status != MatchAmbiguous || len(result.Candidates) != 2 {
		t.Fatalf("Match() = %#v, want two ambiguous candidates", result)
	}
}

func TestCatalogMatchDoesNotCrossToolNamespaces(t *testing.T) {
	catalog := newCatalog([]Entry{{ID: "nse-only", Match: MatchKeys{NSEIDs: []string{"shared"}}}})
	result := catalog.Match(Observation{Tool: ToolNuclei, ToolID: "shared"})
	if result.Status != MatchUnmatched {
		t.Fatalf("Match() = %#v, want unmatched", result)
	}
}

func TestCatalogMatchFallsBackToCVEAfterToolIDMiss(t *testing.T) {
	catalog := newCatalog([]Entry{
		{ID: "wanted", Match: MatchKeys{CVEs: []string{"CVE-2024-0001"}}},
		{ID: "other", Match: MatchKeys{NucleiIDs: []string{"other-id"}}},
	})

	result := catalog.Match(Observation{Tool: ToolNuclei, ToolID: "missing-id", CVEs: []string{"cve-2024-0001"}})
	if result.Status != MatchMatched || result.Entry.ID != "wanted" {
		t.Fatalf("Match() = %#v, want wanted entry", result)
	}
}

func TestCatalogMatchUsesManualReviewNames(t *testing.T) {
	catalog := newCatalog([]Entry{{
		ID:    "smb-signing",
		Match: MatchKeys{Names: []string{"smb-signing-disabled"}},
	}, {ID: "other"}})

	result := catalog.Match(Observation{Tool: ToolManualReview, ToolID: "SMB-SIGNING-DISABLED"})
	if result.Status != MatchMatched || result.Entry.ID != "smb-signing" {
		t.Fatalf("Match() = %#v, want smb-signing entry", result)
	}
}

func TestCatalogMatchUnknownToolIDDoesNotCrossNamespaces(t *testing.T) {
	catalog := newCatalog([]Entry{{ID: "nuclei-only", Match: MatchKeys{NucleiIDs: []string{"shared-id"}}}})

	result := catalog.Match(Observation{Tool: ToolUnknown, ToolID: "shared-id"})
	if result.Status != MatchUnmatched {
		t.Fatalf("Match() = %#v, want unmatched", result)
	}
}

func TestCatalogMatchUsesAliasAsNameEvidence(t *testing.T) {
	catalog := newCatalog([]Entry{
		{ID: "smb-signing", Name: "SMB signing disabled", Aliases: []string{"SMB 签名未启用"}},
		{ID: "other", Name: "other finding"},
	})

	result := catalog.Match(Observation{Tool: ToolNuclei, ToolID: "missing-id", Name: "smb 签名未启用"})
	if result.Status != MatchMatched || result.Entry.ID != "smb-signing" {
		t.Fatalf("Match() = %#v, want smb-signing entry", result)
	}
}

func TestCatalogMatchReturnsUnmatchedWhenAllEvidenceMisses(t *testing.T) {
	catalog := newCatalog([]Entry{
		{ID: "first", Match: MatchKeys{NucleiIDs: []string{"first-id"}, CVEs: []string{"CVE-2024-0001"}}, Name: "first finding"},
		{ID: "second", Match: MatchKeys{NucleiIDs: []string{"second-id"}, CVEs: []string{"CVE-2024-0002"}}, Name: "second finding"},
	})

	result := catalog.Match(Observation{
		Tool:   ToolNuclei,
		ToolID: "missing-id",
		CVEs:   []string{"CVE-2024-9999"},
		Name:   "missing finding",
	})
	if result.Status != MatchUnmatched {
		t.Fatalf("Match() = %#v, want unmatched", result)
	}
}

func TestEntryMatchesKeyword(t *testing.T) {
	entry := Entry{
		ID:      "test-id",
		Name:    "Test Vulnerability",
		Aliases: []string{"alias-one"},
		Match: MatchKeys{
			NucleiIDs: []string{"nuclei-test"},
			NSEIDs:    []string{"nse-test"},
			CVEs:      []string{"CVE-2024-1234"},
			Names:     []string{"manual-name"},
		},
	}
	cases := []struct {
		keyword string
		want    bool
	}{
		{"Test", true},
		{"cve-2024-1234", true},
		{"nse-test", true},
		{"alias-one", true},
		{"manual-name", true},
		{"missing", false},
		{"  TEST  ", true},
	}
	for _, c := range cases {
		if got := entry.MatchesKeyword(c.keyword); got != c.want {
			t.Errorf("MatchesKeyword(%q) = %v, want %v", c.keyword, got, c.want)
		}
	}
}
