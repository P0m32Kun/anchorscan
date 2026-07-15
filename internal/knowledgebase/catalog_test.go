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
		{ID: "first", Match: MatchKeys{ToolIDs: []string{"shared"}}},
		{ID: "second", Match: MatchKeys{ToolIDs: []string{"shared"}}},
	})

	result := catalog.Match(Observation{Tool: ToolNuclei, ToolID: "shared"})
	if result.Status != MatchAmbiguous || len(result.Candidates) != 2 {
		t.Fatalf("Match() = %#v, want two ambiguous candidates", result)
	}
}
