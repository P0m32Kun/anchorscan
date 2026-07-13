package web

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type reportFilters struct {
	IP         string
	Port       string
	Service    string
	Keyword    string
	Severity   string
	Severities []string
	Source     string
}

var supportedSeverities = []string{"critical", "high", "medium", "low", "info"}

func filterFingerprints(items []fingerprint.ServiceFingerprint, filters reportFilters) []fingerprint.ServiceFingerprint {
	var out []fingerprint.ServiceFingerprint
	for _, item := range items {
		if filters.IP != "" && item.IP != filters.IP {
			continue
		}
		if filters.Service != "" && item.Service != filters.Service {
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

func containsValue(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func findingMatchesService(item report.Finding, fps []fingerprint.ServiceFingerprint, service string) bool {
	for _, fp := range fps {
		if fp.IP == item.IP && fp.Port == item.Port && fp.Protocol == item.Protocol && fp.Service == service {
			return true
		}
	}
	return false
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
