package web

import (
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func exportAssetsTXT(items []fingerprint.ServiceFingerprint, kind string) string {
	var lines []string
	switch kind {
	case "ip":
		seen := map[string]struct{}{}
		for _, item := range items {
			if _, ok := seen[item.IP]; ok {
				continue
			}
			seen[item.IP] = struct{}{}
			lines = append(lines, item.IP)
		}
	case "url":
		for _, item := range items {
			if item.URL != "" {
				lines = append(lines, item.URL)
			}
		}
	default:
		for _, item := range items {
			lines = append(lines, item.IP+":"+strconv.Itoa(item.Port))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func exportAssetsCSV(items []fingerprint.ServiceFingerprint) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"ip", "port", "protocol", "service", "product", "version", "cpe", "url"}); err != nil {
		return "", err
	}
	for _, item := range items {
		if err := w.Write([]string{
			item.IP,
			strconv.Itoa(item.Port),
			item.Protocol,
			item.Service,
			item.Product,
			item.Version,
			item.CPE,
			item.URL,
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func exportFindingsCSV(items []report.Finding, fps []fingerprint.ServiceFingerprint) (string, error) {
	services := map[string]fingerprint.ServiceFingerprint{}
	for _, item := range fps {
		services[item.IP+":"+strconv.Itoa(item.Port)+":"+item.Protocol] = item
	}

	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"severity", "source", "id", "ip", "port", "protocol", "service", "product", "target", "summary", "evidence"}); err != nil {
		return "", err
	}
	for _, item := range items {
		fp := services[item.IP+":"+strconv.Itoa(item.Port)+":"+item.Protocol]
		if err := w.Write([]string{
			item.Severity,
			item.Source,
			item.ID,
			item.IP,
			strconv.Itoa(item.Port),
			fp.Service,
			fp.Product,
			item.Target,
			item.Summary,
			item.Output,
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}
