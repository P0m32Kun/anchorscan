package report

import (
	"fmt"
	"sort"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

type Finding struct {
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Source      string `json:"source"`
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Target      string `json:"target"`
	Output      string `json:"output"`
	Description string `json:"description,omitempty"`
	Remediation string `json:"remediation,omitempty"`
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
	IP        string       `json:"ip"`
	OpenPorts []int        `json:"open_ports,omitempty"`
	Ports     []PortReport `json:"ports"`
}

type ScanMeta struct {
	Tool string `json:"tool"`
}

type ScanReport struct {
	ScanMeta          ScanMeta           `json:"scan_meta"`
	AliveIPs          []string           `json:"alive_ips,omitempty"`
	Hosts             []HostReport       `json:"hosts"`
	DetectionChecks   []DetectionCheck   `json:"detection_checks,omitempty"`
	DetectionCoverage *DetectionCoverage `json:"detection_coverage,omitempty"`
}

type DetectionCheck struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	Engine     string `json:"engine"`
	Status     string `json:"status"`
	ReasonCode string `json:"reason_code,omitempty"`
	Detail     string `json:"detail,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type DetectionCoverage struct {
	DualEngine   int `json:"dual_engine"`
	SingleEngine int `json:"single_engine"`
	Uncovered    int `json:"uncovered"`
	Failed       int `json:"failed"`
	Canceled     int `json:"canceled"`
	Interrupted  int `json:"interrupted"`
	Skipped      int `json:"skipped"`
}

func DetectionCheckTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

// ScanData carries the supplementary scan results that the report should expose
// alongside the fingerprint-driven port details: every alive IP discovered by
// the host sweep (even those with no open ports) and the raw open ports per host
// discovered by the port scan (even those nmap could not fingerprint).
type ScanData struct {
	AliveIPs  []string
	OpenPorts map[string][]int // IP → raw open ports from rustscan
}

func Build(fps []fingerprint.ServiceFingerprint, findings []Finding) ScanReport {
	return BuildWithScanData(fps, findings, ScanData{})
}

// BuildWithScanData assembles the report from fingerprints and findings, then
// enriches it with the full alive-IP list and per-host raw open ports. Hosts
// that are alive but yielded no fingerprints are still emitted (with empty
// Ports) so the report reflects every live host on the network.
func BuildWithScanData(fps []fingerprint.ServiceFingerprint, findings []Finding, data ScanData) ScanReport {
	return buildWithScanDataAndDetectionChecks(fps, findings, data, nil, false)
}

func BuildWithScanDataAndDetectionChecks(fps []fingerprint.ServiceFingerprint, findings []Finding, data ScanData, checks []DetectionCheck) ScanReport {
	return buildWithScanDataAndDetectionChecks(fps, findings, data, checks, true)
}

func buildWithScanDataAndDetectionChecks(fps []fingerprint.ServiceFingerprint, findings []Finding, data ScanData, checks []DetectionCheck, includeCoverage bool) ScanReport {
	hostMap := map[string]*HostReport{}
	findingsByPort := map[string][]Finding{}
	attachedFindingPorts := map[string]bool{}
	fingerprintProtocols := map[string]map[string]bool{}

	for _, finding := range dedupeFindings(findings) {
		key := portKey(finding.IP, finding.Port, finding.Protocol)
		findingsByPort[key] = append(findingsByPort[key], finding)
	}
	for _, fp := range fps {
		key := portKey(fp.IP, fp.Port, "")
		if fingerprintProtocols[key] == nil {
			fingerprintProtocols[key] = map[string]bool{}
		}
		fingerprintProtocols[key][fp.Protocol] = true
	}

	for _, fp := range fps {
		host := hostMap[fp.IP]
		if host == nil {
			host = &HostReport{IP: fp.IP}
			hostMap[fp.IP] = host
		}

		key := portKey(fp.IP, fp.Port, fp.Protocol)
		portFindings := append([]Finding(nil), findingsByPort[key]...)
		hostPortKey := portKey(fp.IP, fp.Port, "")
		if fp.Protocol != "" && len(fingerprintProtocols[hostPortKey]) == 1 {
			emptyProtocolKey := portKey(fp.IP, fp.Port, "")
			portFindings = append(portFindings, findingsByPort[emptyProtocolKey]...)
			attachedFindingPorts[emptyProtocolKey] = true
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
			Findings:    portFindings,
		}
		host.Ports = append(host.Ports, port)
		attachedFindingPorts[key] = true
	}

	// Keep findings visible even when their service fingerprint is unavailable.
	for key, portFindings := range findingsByPort {
		if attachedFindingPorts[key] || len(portFindings) == 0 {
			continue
		}
		finding := portFindings[0]
		host := hostMap[finding.IP]
		if host == nil {
			host = &HostReport{IP: finding.IP}
			hostMap[finding.IP] = host
		}
		host.Ports = append(host.Ports, PortReport{
			Port: finding.Port, Protocol: finding.Protocol, Findings: append([]Finding(nil), portFindings...),
		})
	}

	// Ensure every alive host appears in the report, even without fingerprints,
	// and attach the raw open ports discovered by the port scan.
	for _, ip := range data.AliveIPs {
		if hostMap[ip] == nil {
			hostMap[ip] = &HostReport{IP: ip}
		}
	}
	for ip, ports := range data.OpenPorts {
		host := hostMap[ip]
		if host == nil {
			host = &HostReport{IP: ip}
			hostMap[ip] = host
		}
		if len(ports) > 0 {
			open := append([]int(nil), ports...)
			sort.Ints(open)
			host.OpenPorts = dedupeInts(open)
		}
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

	aliveIPs := append([]string(nil), data.AliveIPs...)
	if len(aliveIPs) > 0 {
		sort.Strings(aliveIPs)
		aliveIPs = dedupeStrings(aliveIPs)
	}

	report := ScanReport{
		ScanMeta: ScanMeta{Tool: "anchorscan"},
		AliveIPs: aliveIPs,
		Hosts:    hosts,
	}
	if len(checks) > 0 {
		report.DetectionChecks = append([]DetectionCheck(nil), checks...)
		sort.Slice(report.DetectionChecks, func(i, j int) bool {
			left, right := report.DetectionChecks[i], report.DetectionChecks[j]
			if left.IP != right.IP {
				return left.IP < right.IP
			}
			if left.Port != right.Port {
				return left.Port < right.Port
			}
			if left.Protocol != right.Protocol {
				return left.Protocol < right.Protocol
			}
			return left.Engine < right.Engine
		})
	}
	if includeCoverage {
		coverage := summarizeDetectionCoverage(fps, report.DetectionChecks)
		report.DetectionCoverage = &coverage
	}
	return report
}

func summarizeDetectionCoverage(fps []fingerprint.ServiceFingerprint, checks []DetectionCheck) DetectionCoverage {
	completed := map[string]map[string]bool{}
	coverage := DetectionCoverage{}
	for _, check := range checks {
		switch check.Status {
		case "failed":
			coverage.Failed++
		case "canceled":
			coverage.Canceled++
		case "interrupted":
			coverage.Interrupted++
		case "skipped":
			coverage.Skipped++
		}
		if check.Status != "completed" || (check.Engine != "nse" && check.Engine != "nuclei") {
			continue
		}
		key := portKey(check.IP, check.Port, check.Protocol)
		if completed[key] == nil {
			completed[key] = map[string]bool{}
		}
		completed[key][check.Engine] = true
	}
	for _, fp := range fps {
		count := len(completed[portKey(fp.IP, fp.Port, fp.Protocol)])
		switch count {
		case 2:
			coverage.DualEngine++
		case 1:
			coverage.SingleEngine++
		default:
			coverage.Uncovered++
		}
	}
	return coverage
}

func dedupeInts(values []int) []int {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[int]struct{}, len(values))
	out := values[:0]
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func dedupeStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := values[:0]
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
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
