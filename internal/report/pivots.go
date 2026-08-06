package report

import (
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

// Pivot dimension labels. These are the five analysis axes derived from a run's
// fingerprints and findings: host, port, service, product, and vulnerability
// (by severity). Dimension values are stable identifiers used both as JSON keys
// for the frontend and as the URL query parameter namespace.
const (
	PivotHost          = "host"
	PivotPort          = "port"
	PivotService       = "service"
	PivotProduct       = "product"
	PivotVulnerability = "vulnerability"
)

// emptyFacetLabel is the human label shown for a dimension value that is empty
// (e.g. a fingerprint with no identified service or product). The RawValue
// stays "" so deep-link filtering targets the genuine empty value.
const emptyFacetLabel = "未识别（空）"

// PivotFacet is one value of one pivot dimension with the number of distinct
// assets behind it. Counts never double-count a repeated endpoint or finding.
type PivotFacet struct {
	Dimension string `json:"dimension"`
	RawValue  string `json:"raw_value"`
	Label     string `json:"label"`
	Count     int    `json:"count"`
}

// PivotMatrix is a host x service cross-tab. Rows are distinct hosts, columns
// are distinct services (empty service last), and each cell holds the number of
// distinct endpoints for that host/service pair.
type PivotMatrix struct {
	RowDimension string   `json:"row_dimension"`
	ColDimension string   `json:"col_dimension"`
	Rows         []string `json:"rows"`
	Cols         []string `json:"cols"`
	Cells        [][]int  `json:"cells"`
}

type endpoint struct {
	IP       string
	Port     int
	Protocol string
	Service  string
	Product  string
}

type endpointKey struct {
	ip       string
	port     int
	protocol string
}

// dedupEndpoints collapses repeated fingerprints for the same (IP, Port,
// Protocol) into a single endpoint. A run normally stores one fingerprint per
// endpoint, but the report view must stay correct when duplicates exist.
func dedupEndpoints(fps []fingerprint.ServiceFingerprint) []endpoint {
	seen := map[endpointKey]endpoint{}
	order := make([]endpointKey, 0, len(fps))
	for _, fp := range fps {
		key := endpointKey{ip: fp.IP, port: fp.Port, protocol: fp.Protocol}
		if _, ok := seen[key]; !ok {
			seen[key] = endpoint{IP: fp.IP, Port: fp.Port, Protocol: fp.Protocol, Service: fp.Service, Product: fp.Product}
			order = append(order, key)
		}
	}
	out := make([]endpoint, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	return out
}

// BuildPivotFacets derives the five pivot dimensions from fingerprints and
// findings. Fingerprints are deduped by (IP, Port, Protocol); findings are
// deduped by (IP, Port, Source, ID), so repeated rows never inflate counts.
func BuildPivotFacets(fps []fingerprint.ServiceFingerprint, findings []Finding) []PivotFacet {
	endpoints := dedupEndpoints(fps)
	facets := make([]PivotFacet, 0)
	facets = append(facets, hostFacets(endpoints)...)
	facets = append(facets, portFacets(endpoints)...)
	facets = append(facets, stringFacets(endpoints, PivotService, func(e endpoint) string { return e.Service })...)
	facets = append(facets, stringFacets(endpoints, PivotProduct, func(e endpoint) string { return e.Product })...)
	facets = append(facets, vulnerabilityFacets(findings)...)
	return facets
}

// BuildServiceMatrix builds the host x service cross-tab from deduped endpoints.
func BuildServiceMatrix(fps []fingerprint.ServiceFingerprint) PivotMatrix {
	endpoints := dedupEndpoints(fps)
	if len(endpoints) == 0 {
		// Non-nil empty slices so JSON emits [] rather than null.
		return PivotMatrix{RowDimension: PivotHost, ColDimension: PivotService, Rows: []string{}, Cols: []string{}, Cells: [][]int{}}
	}

	rowSet := map[string]struct{}{}
	colSet := map[string]struct{}{}
	cell := map[string]map[string]int{}
	for _, e := range endpoints {
		rowSet[e.IP] = struct{}{}
		colSet[e.Service] = struct{}{}
		if cell[e.IP] == nil {
			cell[e.IP] = map[string]int{}
		}
		cell[e.IP][e.Service]++
	}

	rows := sortedHosts(rowSet)
	cols := sortedStringsWithEmptyLast(keysOf(colSet))
	colIndex := map[string]int{}
	for j, c := range cols {
		colIndex[c] = j
	}
	cells := make([][]int, len(rows))
	for i, row := range rows {
		cells[i] = make([]int, len(cols))
		for rawCol, count := range cell[row] {
			cells[i][colIndex[rawCol]] = count
		}
	}
	return PivotMatrix{RowDimension: PivotHost, ColDimension: PivotService, Rows: rows, Cols: cols, Cells: cells}
}

func hostFacets(endpoints []endpoint) []PivotFacet {
	counts := map[string]int{}
	for _, e := range endpoints {
		counts[e.IP]++
	}
	hosts := make([]string, 0, len(counts))
	for host := range counts {
		hosts = append(hosts, host)
	}
	sort.Slice(hosts, func(i, j int) bool { return compareHost(hosts[i], hosts[j]) < 0 })
	facets := make([]PivotFacet, 0, len(hosts))
	for _, host := range hosts {
		facets = append(facets, PivotFacet{Dimension: PivotHost, RawValue: host, Label: host, Count: counts[host]})
	}
	return facets
}

func portFacets(endpoints []endpoint) []PivotFacet {
	// Distinct hosts per port, so a host with the same port over two protocols
	// counts once.
	hostsByPort := map[int]map[string]struct{}{}
	for _, e := range endpoints {
		if hostsByPort[e.Port] == nil {
			hostsByPort[e.Port] = map[string]struct{}{}
		}
		hostsByPort[e.Port][e.IP] = struct{}{}
	}
	ports := make([]int, 0, len(hostsByPort))
	for port := range hostsByPort {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	facets := make([]PivotFacet, 0, len(ports))
	for _, port := range ports {
		value := strconv.Itoa(port)
		facets = append(facets, PivotFacet{Dimension: PivotPort, RawValue: value, Label: value, Count: len(hostsByPort[port])})
	}
	return facets
}

func stringFacets(endpoints []endpoint, dimension string, pick func(endpoint) string) []PivotFacet {
	counts := map[string]int{}
	for _, e := range endpoints {
		counts[pick(e)]++
	}
	values := sortedStringsWithEmptyLast(keysOf(counts))
	facets := make([]PivotFacet, 0, len(values))
	for _, value := range values {
		label := value
		if value == "" {
			label = emptyFacetLabel
		}
		facets = append(facets, PivotFacet{Dimension: dimension, RawValue: value, Label: label, Count: counts[value]})
	}
	return facets
}

func vulnerabilityFacets(findings []Finding) []PivotFacet {
	seen := map[vulnKey]struct{}{}
	counts := map[string]int{}
	for _, f := range findings {
		key := vulnKey{ip: f.IP, port: f.Port, source: f.Source, id: f.ID}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		counts[strings.ToLower(f.Severity)]++
	}
	severities := make([]string, 0, len(counts))
	for sev := range counts {
		severities = append(severities, sev)
	}
	sort.Slice(severities, func(i, j int) bool { return severityRank(severities[i]) < severityRank(severities[j]) })
	facets := make([]PivotFacet, 0, len(severities))
	for _, sev := range severities {
		facets = append(facets, PivotFacet{Dimension: PivotVulnerability, RawValue: sev, Label: severityLabel(sev), Count: counts[sev]})
	}
	return facets
}

type vulnKey struct {
	ip     string
	port   int
	source string
	id     string
}

func compareHost(a, b string) int {
	addrA, errA := netip.ParseAddr(a)
	addrB, errB := netip.ParseAddr(b)
	if errA == nil && errB == nil {
		return addrA.Compare(addrB)
	}
	return strings.Compare(a, b)
}

func sortedHosts(set map[string]struct{}) []string {
	hosts := keysOf(set)
	sort.Slice(hosts, func(i, j int) bool { return compareHost(hosts[i], hosts[j]) < 0 })
	return hosts
}

// sortedStringsWithEmptyLast sorts the slice in place: non-empty values
// case-insensitively, then the empty value last, so "未识别" always trails.
func sortedStringsWithEmptyLast(values []string) []string {
	sort.SliceStable(values, func(i, j int) bool {
		if values[i] == "" {
			return false
		}
		if values[j] == "" {
			return true
		}
		return strings.ToLower(values[i]) < strings.ToLower(values[j])
	})
	return values
}

func keysOf[T any](set map[string]T) []string {
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}
