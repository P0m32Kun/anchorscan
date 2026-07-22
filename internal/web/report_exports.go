package web

import (
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
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
