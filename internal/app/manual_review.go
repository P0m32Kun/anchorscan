package app

import (
	"fmt"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func ManualReviewFindings(fp fingerprint.ServiceFingerprint) []report.Finding {
	service := strings.ToLower(strings.Join([]string{fp.Service, fp.Normalized, fp.Product}, " "))
	if fp.Port != 3389 || !(strings.Contains(service, "rdp") || strings.Contains(service, "ms-wbt-server")) {
		return nil
	}
	return []report.Finding{{
		IP:       fp.IP,
		Port:     fp.Port,
		Source:   "manual-review",
		ID:       "manual-review:CVE-2019-0708",
		Severity: "critical",
		Summary:  "RDP service requires BlueKeep verification",
		Target:   fmt.Sprintf("%s:%d", fp.IP, fp.Port),
		Output:   fmt.Sprintf("RDP-like service detected on %s:%d (%s %s). BlueKeep requires external validation; AnchorScan does not bundle Metasploit or run exploit verification.", fp.IP, fp.Port, fp.Service, fp.Product),
	}}
}
