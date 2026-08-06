package report

import (
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

// Ticket 05: host/port/service/product/vulnerability pivots must dedup repeated
// endpoints, sort deterministically, and label empty values consistently. These
// tests pin the semantics before the implementation exists.

func fp(ip string, port int, protocol, service, product string) fingerprint.ServiceFingerprint {
	return fingerprint.ServiceFingerprint{IP: ip, Port: port, Protocol: protocol, Service: service, Product: product}
}

func finding(ip string, port int, source, id, severity string) Finding {
	return Finding{IP: ip, Port: port, Source: source, ID: id, Severity: severity}
}

func TestBuildPivotFacetsDedupsAndSorts(t *testing.T) {
	// 192.0.2.10:80 is duplicated on purpose; redis and .13:8443 carry empty
	// product/service to exercise empty-value handling.
	fps := []fingerprint.ServiceFingerprint{
		fp("192.0.2.10", 443, "tcp", "https", "nginx"),
		fp("192.0.2.10", 80, "tcp", "http", "nginx"),
		fp("192.0.2.10", 80, "tcp", "http", "nginx"), // duplicate endpoint
		fp("192.0.2.11", 22, "tcp", "ssh", "OpenSSH"),
		fp("192.0.2.12", 6379, "tcp", "redis", ""),
		fp("192.0.2.13", 8443, "tcp", "", ""),
	}
	findings := []Finding{
		finding("192.0.2.12", 6379, "nuclei", "redis-logins", "high"),
		finding("192.0.2.10", 443, "nuclei", "tls-old", "medium"),
		finding("192.0.2.12", 6379, "nuclei", "redis-logins", "high"), // duplicate finding
	}

	facets := BuildPivotFacets(fps, findings)

	byDim := map[string][]PivotFacet{}
	for _, f := range facets {
		byDim[f.Dimension] = append(byDim[f.Dimension], f)
	}

	// Host: distinct endpoints per host, IP order.
	assertFacets(t, "host", byDim["host"], []PivotFacet{
		{Dimension: "host", RawValue: "192.0.2.10", Label: "192.0.2.10", Count: 2},
		{Dimension: "host", RawValue: "192.0.2.11", Label: "192.0.2.11", Count: 1},
		{Dimension: "host", RawValue: "192.0.2.12", Label: "192.0.2.12", Count: 1},
		{Dimension: "host", RawValue: "192.0.2.13", Label: "192.0.2.13", Count: 1},
	})

	// Port: distinct hosts per port, numeric order.
	assertFacets(t, "port", byDim["port"], []PivotFacet{
		{Dimension: "port", RawValue: "22", Label: "22", Count: 1},
		{Dimension: "port", RawValue: "80", Label: "80", Count: 1},
		{Dimension: "port", RawValue: "443", Label: "443", Count: 1},
		{Dimension: "port", RawValue: "6379", Label: "6379", Count: 1},
		{Dimension: "port", RawValue: "8443", Label: "8443", Count: 1},
	})

	// Service: distinct endpoints, alpha order, empty value labeled and last.
	assertFacets(t, "service", byDim["service"], []PivotFacet{
		{Dimension: "service", RawValue: "http", Label: "http", Count: 1},
		{Dimension: "service", RawValue: "https", Label: "https", Count: 1},
		{Dimension: "service", RawValue: "redis", Label: "redis", Count: 1},
		{Dimension: "service", RawValue: "ssh", Label: "ssh", Count: 1},
		{Dimension: "service", RawValue: "", Label: "未识别（空）", Count: 1},
	})

	// Product: distinct endpoints, case-insensitive alpha order, empty last.
	assertFacets(t, "product", byDim["product"], []PivotFacet{
		{Dimension: "product", RawValue: "nginx", Label: "nginx", Count: 2},
		{Dimension: "product", RawValue: "OpenSSH", Label: "OpenSSH", Count: 1},
		{Dimension: "product", RawValue: "", Label: "未识别（空）", Count: 2},
	})

	// Vulnerability: by severity, severity rank order, distinct findings.
	assertFacets(t, "vulnerability", byDim["vulnerability"], []PivotFacet{
		{Dimension: "vulnerability", RawValue: "high", Label: "高危", Count: 1},
		{Dimension: "vulnerability", RawValue: "medium", Label: "中危", Count: 1},
	})
}

func TestBuildPivotFacetsEmpty(t *testing.T) {
	facets := BuildPivotFacets(nil, nil)
	if len(facets) != 0 {
		t.Fatalf("expected no facets for empty input, got %d", len(facets))
	}
}

func TestBuildServiceMatrixDedupsAndOrders(t *testing.T) {
	fps := []fingerprint.ServiceFingerprint{
		fp("192.0.2.10", 443, "tcp", "https", "nginx"),
		fp("192.0.2.10", 80, "tcp", "http", "nginx"),
		fp("192.0.2.10", 80, "tcp", "http", "nginx"), // duplicate
		fp("192.0.2.11", 22, "tcp", "ssh", "OpenSSH"),
		fp("192.0.2.13", 8443, "tcp", "", ""),
	}
	m := BuildServiceMatrix(fps)
	if m.RowDimension != "host" || m.ColDimension != "service" {
		t.Fatalf("unexpected matrix dimensions: %s x %s", m.RowDimension, m.ColDimension)
	}
	if !reflect.DeepEqual(m.Rows, []string{"192.0.2.10", "192.0.2.11", "192.0.2.13"}) {
		t.Fatalf("unexpected matrix rows: %v", m.Rows)
	}
	if !reflect.DeepEqual(m.Cols, []string{"http", "https", "ssh", ""}) {
		t.Fatalf("unexpected matrix cols: %v", m.Cols)
	}
	// Cells[row][col] = distinct endpoint count.
	want := [][]int{
		{1, 1, 0, 0}, // 192.0.2.10: http, https
		{0, 0, 1, 0}, // 192.0.2.11: ssh
		{0, 0, 0, 1}, // 192.0.2.13: empty service
	}
	if !reflect.DeepEqual(m.Cells, want) {
		t.Fatalf("unexpected matrix cells:\n got %v\nwant %v", m.Cells, want)
	}
}

func TestBuildServiceMatrixEmpty(t *testing.T) {
	m := BuildServiceMatrix(nil)
	if len(m.Rows) != 0 || len(m.Cols) != 0 || len(m.Cells) != 0 {
		t.Fatalf("expected empty matrix, got %+v", m)
	}
}

func assertFacets(t *testing.T, dim string, got, want []PivotFacet) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s facets mismatch:\n got %+v\nwant %+v", dim, got, want)
	}
}
