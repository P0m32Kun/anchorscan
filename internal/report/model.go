package report

import (
	"fmt"
	"sort"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

type Finding struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Source   string `json:"source"`
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Target   string `json:"target"`
	Output   string `json:"output"`
}

type PortReport struct {
	Port        int       `json:"port"`
	Service     string    `json:"service"`
	Product     string    `json:"product"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	IsWeb       bool      `json:"is_web"`
	URL         string    `json:"url,omitempty"`
	Findings    []Finding `json:"findings,omitempty"`
}

type HostReport struct {
	IP    string       `json:"ip"`
	Ports []PortReport `json:"ports"`
}

type ScanMeta struct {
	Tool string `json:"tool"`
}

type ScanReport struct {
	ScanMeta ScanMeta     `json:"scan_meta"`
	Hosts    []HostReport `json:"hosts"`
}

func Build(fps []fingerprint.ServiceFingerprint, findings []Finding) ScanReport {
	hostMap := map[string]*HostReport{}
	findingsByPort := map[string][]Finding{}

	for _, finding := range dedupeFindings(findings) {
		key := portKey(finding.IP, finding.Port)
		findingsByPort[key] = append(findingsByPort[key], finding)
	}

	for _, fp := range fps {
		host := hostMap[fp.IP]
		if host == nil {
			host = &HostReport{IP: fp.IP}
			hostMap[fp.IP] = host
		}

		port := PortReport{
			Port:        fp.Port,
			Service:     fp.Service,
			Product:     fp.Product,
			Fingerprint: fp.Normalized,
			IsWeb:       fp.IsWeb,
			URL:         fp.URL,
			Findings:    append([]Finding(nil), findingsByPort[portKey(fp.IP, fp.Port)]...),
		}
		host.Ports = append(host.Ports, port)
	}

	hosts := make([]HostReport, 0, len(hostMap))
	for _, host := range hostMap {
		sort.Slice(host.Ports, func(i, j int) bool {
			return host.Ports[i].Port < host.Ports[j].Port
		})
		hosts = append(hosts, *host)
	}
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].IP < hosts[j].IP
	})

	return ScanReport{
		ScanMeta: ScanMeta{Tool: "anchorscan"},
		Hosts:    hosts,
	}
}

func portKey(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func dedupeFindings(findings []Finding) []Finding {
	seen := make(map[string]struct{}, len(findings))
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		key := finding.IP + "\x00" + finding.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, finding)
	}
	return out
}
