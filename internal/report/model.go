package report

import (
	"fmt"
	"sort"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

type Finding struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Source   string `json:"source"`
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Target   string `json:"target"`
	Output   string `json:"output"`
}

type PortReport struct {
	Port        int       `json:"port"`
	Protocol    string    `json:"protocol"`
	Service     string    `json:"service"`
	Product     string    `json:"product"`
	CPE         string    `json:"cpe,omitempty"`
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
		key := portKey(finding.IP, finding.Port, finding.Protocol)
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
			Protocol:    fp.Protocol,
			Service:     fp.Service,
			Product:     fp.Product,
			CPE:         fp.CPE,
			Fingerprint: fp.Normalized,
			IsWeb:       fp.IsWeb,
			URL:         fp.URL,
			Findings:    append([]Finding(nil), findingsByPort[portKey(fp.IP, fp.Port, fp.Protocol)]...),
		}
		host.Ports = append(host.Ports, port)
	}

	hosts := make([]HostReport, 0, len(hostMap))
	for _, host := range hostMap {
		sort.Slice(host.Ports, func(i, j int) bool {
			if host.Ports[i].Port != host.Ports[j].Port {
				return host.Ports[i].Port < host.Ports[j].Port
			}
			return host.Ports[i].Protocol < host.Ports[j].Protocol
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

func portKey(ip string, port int, protocol string) string {
	return fmt.Sprintf("%s:%d:%s", ip, port, protocol)
}

func dedupeFindings(findings []Finding) []Finding {
	seen := make(map[string]struct{}, len(findings))
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		key := fmt.Sprintf("%s\x00%d\x00%s\x00%s", finding.IP, finding.Port, finding.Protocol, finding.ID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, finding)
	}
	return out
}
