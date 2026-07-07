package web

import (
	"strconv"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type reportFilters struct {
	IP       string
	Port     string
	Service  string
	Severity string
	Source   string
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
		if filters.Port != "" {
			port, err := strconv.Atoi(filters.Port)
			if err != nil || item.Port != port {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func filterFindings(items []report.Finding, filters reportFilters) []report.Finding {
	var out []report.Finding
	for _, item := range items {
		if filters.IP != "" && item.IP != filters.IP {
			continue
		}
		if filters.Severity != "" && item.Severity != filters.Severity {
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
		out = append(out, item)
	}
	return out
}
