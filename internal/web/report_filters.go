package web

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type reportFilters struct {
	IP                  string
	Port                string
	Service             string
	Product             string
	ExcludeUnidentified bool
	Keyword             string
	Severity            string
	Severities          []string
	Source              string
}

var supportedSeverities = []string{"critical", "high", "medium", "low", "info"}

type serviceFacet struct {
	RawValue string `json:"raw_value"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
}

func (filters reportFilters) withoutServiceFilters() reportFilters {
	filters.Service = ""
	filters.ExcludeUnidentified = false
	return filters
}

func buildServiceFacets(items []fingerprint.ServiceFingerprint) []serviceFacet {
	counts := make(map[string]int)
	for _, item := range items {
		counts[item.Service]++
	}
	facets := make([]serviceFacet, 0, len(counts))
	for rawValue, count := range counts {
		label := rawValue
		if rawValue == "" {
			label = "未识别（空）"
		}
		facets = append(facets, serviceFacet{RawValue: rawValue, Label: label, Count: count})
	}
	sort.Slice(facets, func(i, j int) bool {
		return facets[i].RawValue < facets[j].RawValue
	})
	return facets
}

func filterFingerprints(items []fingerprint.ServiceFingerprint, filters reportFilters) []fingerprint.ServiceFingerprint {
	var out []fingerprint.ServiceFingerprint
	for _, item := range items {
		if filters.IP != "" && item.IP != filters.IP {
			continue
		}
		if filters.Service != "" && item.Service != filters.Service {
			continue
		}
		if filters.Product != "" && item.Product != filters.Product {
			continue
		}
		if filters.ExcludeUnidentified && isUnidentifiedService(item.Service) {
			continue
		}
		if filters.Port != "" {
			port, err := strconv.Atoi(filters.Port)
			if err != nil || item.Port != port {
				continue
			}
		}
		if filters.Keyword != "" && !fingerprintMatchesKeyword(item, filters.Keyword) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterFindings(items []report.Finding, fps []fingerprint.ServiceFingerprint, filters reportFilters) []report.Finding {
	var out []report.Finding
	for _, item := range items {
		if filters.IP != "" && item.IP != filters.IP {
			continue
		}
		if filters.Service != "" && !findingMatchesService(item, fps, filters.Service) {
			continue
		}
		if filters.Product != "" && !findingMatchesProduct(item, fps, filters.Product) {
			continue
		}
		if filters.ExcludeUnidentified && findingMatchesUnidentifiedService(item, fps) {
			continue
		}
		if len(filters.Severities) > 0 && !containsValue(filters.Severities, item.Severity) {
			continue
		}
		if len(filters.Severities) == 0 && filters.Severity != "" && item.Severity != filters.Severity {
			continue
		}
		if filters.Source != "" && item.Source != filters.Source {
			continue
		}
		if filters.Port != "" {
			port, err := strconv.Atoi(filters.Port)
			if err != nil || item.Port != port {
				continue
			}
		}
		if filters.Keyword != "" && !findingMatchesKeyword(item, fps, filters.Keyword) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterFindingsForView(items []report.Finding, fps []fingerprint.ServiceFingerprint, filters reportFilters, catalog *knowledgebase.Catalog, vulnerabilityView bool) []report.Finding {
	keyword := filters.Keyword
	if !vulnerabilityView || strings.TrimSpace(keyword) == "" {
		return filterFindings(items, fps, filters)
	}
	filters.Keyword = ""
	candidates := filterFindings(items, fps, filters)
	result := make([]report.Finding, 0, len(candidates))
	for _, finding := range candidates {
		if findingMatchesKeyword(finding, fps, keyword) || catalogEntryMatchesKeyword(catalog, finding, keyword) {
			result = append(result, finding)
		}
	}
	return result
}

func catalogEntryMatchesKeyword(catalog *knowledgebase.Catalog, finding report.Finding, keyword string) bool {
	if catalog == nil {
		return false
	}
	match := catalog.Match(report.ObservationFromFinding(finding))
	if match.Status != knowledgebase.MatchMatched {
		return false
	}
	return match.Entry.MatchesKeyword(keyword)
}

func filterDetectionChecks(items []report.DetectionCheck, fps []fingerprint.ServiceFingerprint) []report.DetectionCheck {
	var out []report.DetectionCheck
	for _, item := range items {
		for _, fp := range fps {
			if item.IP == fp.IP && item.Port == fp.Port && item.Protocol == fp.Protocol {
				out = append(out, item)
				break
			}
		}
	}
	return out
}

func containsValue(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func findingMatchesUnidentifiedService(item report.Finding, fps []fingerprint.ServiceFingerprint) bool {
	for _, fp := range fps {
		if findingMatchesFingerprint(item, fp, fps) && isUnidentifiedService(fp.Service) {
			return true
		}
	}
	return false
}

func isUnidentifiedService(service string) bool {
	return service == "" || service == "tcpwrapped" || service == "unknown"
}

func findingMatchesService(item report.Finding, fps []fingerprint.ServiceFingerprint, service string) bool {
	for _, fp := range fps {
		if findingMatchesFingerprint(item, fp, fps) && fp.Service == service {
			return true
		}
	}
	return false
}

func findingMatchesProduct(item report.Finding, fps []fingerprint.ServiceFingerprint, product string) bool {
	for _, fp := range fps {
		if findingMatchesFingerprint(item, fp, fps) && fp.Product == product {
			return true
		}
	}
	return false
}

// findingMatchesFingerprint follows report.Build's fallback association for a
// protocol-less Finding: it attaches only when the port has one protocol.
func findingMatchesFingerprint(item report.Finding, candidate fingerprint.ServiceFingerprint, fps []fingerprint.ServiceFingerprint) bool {
	if item.IP != candidate.IP || item.Port != candidate.Port {
		return false
	}
	if item.Protocol == candidate.Protocol {
		return true
	}
	if item.Protocol != "" || candidate.Protocol == "" {
		return false
	}

	protocols := map[string]struct{}{}
	for _, fp := range fps {
		if fp.IP == item.IP && fp.Port == item.Port {
			protocols[fp.Protocol] = struct{}{}
		}
	}
	return len(protocols) == 1
}

func fingerprintMatchesKeyword(item fingerprint.ServiceFingerprint, keyword string) bool {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return true
	}
	haystack := []string{
		item.IP,
		strconv.Itoa(item.Port),
		item.Service,
		item.Product,
		item.Version,
		item.ExtraInfo,
		item.Normalized,
		item.URL,
	}
	for _, field := range haystack {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func findingMatchesKeyword(item report.Finding, fps []fingerprint.ServiceFingerprint, keyword string) bool {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return true
	}
	haystack := []string{
		item.IP,
		strconv.Itoa(item.Port),
		item.Source,
		item.ID,
		item.Severity,
		item.Summary,
		item.Target,
		item.Output,
	}
	for _, field := range haystack {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	for _, fp := range fps {
		if fp.IP == item.IP && fp.Port == item.Port && fingerprintMatchesKeyword(fp, keyword) {
			return true
		}
	}
	return false
}

func parseSeverityFilters(values url.Values) []string {
	if len(values["severity"]) == 0 {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range values["severity"] {
		for _, item := range strings.Split(raw, ",") {
			value := strings.ToLower(strings.TrimSpace(item))
			if !containsValue(supportedSeverities, value) {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}
