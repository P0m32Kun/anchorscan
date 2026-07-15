package knowledgebase

import (
	"sort"
	"strings"
)

type Status string

const (
	StatusDisabled    Status = "disabled"
	StatusUnavailable Status = "unavailable"
	StatusDegraded    Status = "degraded"
	StatusReady       Status = "ready"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type Tool string

const (
	ToolNuclei       Tool = "nuclei"
	ToolNSE          Tool = "nse"
	ToolManualReview Tool = "manual-review"
	ToolUnknown      Tool = "unknown"
)

type MatchStatus string

const (
	MatchMatched   MatchStatus = "matched"
	MatchUnmatched MatchStatus = "unmatched"
	MatchAmbiguous MatchStatus = "ambiguous"
)

type Commands struct {
	Nuclei     string
	NmapNSE    string
	Metasploit string
}

type MatchKeys struct {
	ToolIDs []string
	CVEs    []string
	Names   []string
}

type Entry struct {
	ID          string
	Name        string
	Severity    Severity
	Aliases     []string
	Match       MatchKeys
	Description string
	Commands    Commands
	Remediation string
}

type Observation struct {
	Tool   Tool
	ToolID string
	CVEs   []string
	Name   string
}

type Diagnostic struct {
	Status  Status
	EntryID string
	Tool    Tool
	Line    int
	Reason  string
}

type MatchResult struct {
	Status     MatchStatus
	Entry      Entry
	Candidates []Entry
}

type Catalog struct {
	status      Status
	diagnostics []Diagnostic
	entries     []Entry
	byID        map[string]Entry
}

func newCatalog(entries []Entry) *Catalog {
	entries = copyEntries(entries)
	sortEntries(entries)
	byID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	return &Catalog{status: StatusReady, entries: entries, byID: byID}
}

func (c *Catalog) Status() Status { return c.status }

func (c *Catalog) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), c.diagnostics...)
}

func (c *Catalog) Search(query string) []Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return copyEntries(c.entries)
	}
	entries := make([]Entry, 0, len(c.entries))
	for _, entry := range c.entries {
		if entryContains(entry, query) {
			entries = append(entries, entry)
		}
	}
	return copyEntries(entries)
}

func (c *Catalog) Entry(id string) (Entry, bool) {
	entry, ok := c.byID[id]
	return copyEntry(entry), ok
}

func (c *Catalog) Match(observation Observation) MatchResult {
	candidates := c.entries
	if observation.ToolID != "" {
		candidates = filterEntries(candidates, func(entry Entry) bool {
			return toolIDMatches(entry, observation.Tool, observation.ToolID)
		})
	}
	if len(candidates) > 1 && len(observation.CVEs) > 0 {
		candidates = filterEntries(candidates, func(entry Entry) bool {
			return cveMatches(entry, observation.CVEs)
		})
	}
	if len(candidates) > 1 && observation.Name != "" {
		candidates = filterEntries(candidates, func(entry Entry) bool {
			return nameMatches(entry, observation.Name)
		})
	}
	switch len(candidates) {
	case 0:
		return MatchResult{Status: MatchUnmatched}
	case 1:
		return MatchResult{Status: MatchMatched, Entry: copyEntry(candidates[0])}
	default:
		return MatchResult{Status: MatchAmbiguous, Candidates: copyEntries(candidates)}
	}
}

func entryContains(entry Entry, query string) bool {
	values := append([]string{entry.ID, entry.Name}, entry.Aliases...)
	values = append(values, entry.Match.CVEs...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func toolIDMatches(entry Entry, tool Tool, toolID string) bool {
	if tool == ToolUnknown || tool == ToolManualReview {
		return false
	}
	for _, candidate := range entry.Match.ToolIDs {
		if strings.EqualFold(candidate, toolID) {
			return true
		}
	}
	return false
}

func cveMatches(entry Entry, cves []string) bool {
	for _, cve := range cves {
		for _, candidate := range entry.Match.CVEs {
			if strings.EqualFold(candidate, cve) {
				return true
			}
		}
	}
	return false
}

func nameMatches(entry Entry, name string) bool {
	for _, candidate := range append([]string{entry.Name}, entry.Aliases...) {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

func filterEntries(entries []Entry, keep func(Entry) bool) []Entry {
	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if keep(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		left, right := severityRank(entries[i].Severity), severityRank(entries[j].Severity)
		if left != right {
			return left < right
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].ID < entries[j].ID
	})
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 3
	default:
		return 4
	}
}

func copyEntries(entries []Entry) []Entry {
	result := make([]Entry, len(entries))
	for i, entry := range entries {
		result[i] = copyEntry(entry)
	}
	return result
}

func copyEntry(entry Entry) Entry {
	entry.Aliases = append([]string(nil), entry.Aliases...)
	entry.Match.ToolIDs = append([]string(nil), entry.Match.ToolIDs...)
	entry.Match.CVEs = append([]string(nil), entry.Match.CVEs...)
	entry.Match.Names = append([]string(nil), entry.Match.Names...)
	return entry
}
