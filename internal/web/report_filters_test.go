package web

import (
	"net/url"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func TestFilterFindingsBySeverityAndSource(t *testing.T) {
	findings := []report.Finding{
		{IP: "127.0.0.1", Port: 6379, Source: "nuclei", Severity: "high", ID: "redis-default-logins"},
		{IP: "127.0.0.1", Port: 8080, Source: "nse", Severity: "info", ID: "http-title"},
	}
	got := filterFindings(findings, nil, reportFilters{Severity: "high", Source: "nuclei"})
	if len(got) != 1 || got[0].ID != "redis-default-logins" {
		t.Fatalf("unexpected findings: %#v", got)
	}
}

func TestFilterFindingsByMultipleSeverities(t *testing.T) {
	findings := []report.Finding{
		{IP: "127.0.0.1", Port: 6379, Source: "nuclei", Severity: "critical", ID: "redis-rce"},
		{IP: "127.0.0.1", Port: 8080, Source: "nse", Severity: "high", ID: "tomcat-default-login"},
		{IP: "127.0.0.1", Port: 8443, Source: "nuclei", Severity: "low", ID: "banner-detect"},
	}

	got := filterFindings(findings, nil, reportFilters{Severities: []string{"critical", "high"}})
	if len(got) != 2 {
		t.Fatalf("unexpected findings count: %#v", got)
	}
	if got[0].ID != "redis-rce" || got[1].ID != "tomcat-default-login" {
		t.Fatalf("unexpected findings: %#v", got)
	}
}

func TestFilterFingerprintsMatchesKeywordAcrossFingerprintFields(t *testing.T) {
	items := []fingerprint.ServiceFingerprint{
		{IP: "127.0.0.1", Port: 6379, Service: "unknown", Product: "Redis", Version: "7.2.0", URL: "", Normalized: "redis"},
		{IP: "127.0.0.1", Port: 8080, Service: "http", Product: "Apache Tomcat", Version: "10.1.0", URL: "http://127.0.0.1:8080", Normalized: "tomcat"},
	}

	got := filterFingerprints(items, reportFilters{Keyword: "redis"})
	if len(got) != 1 || got[0].Port != 6379 {
		t.Fatalf("unexpected fingerprints: %#v", got)
	}
}

func TestParseSeverityFiltersNormalizesAndDeduplicates(t *testing.T) {
	got := parseSeverityFilters(url.Values{"severity": {"HIGH,unknown", "high", "critical"}})
	want := []string{"high", "critical"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("severities = %#v", got)
	}
}

func TestFilterFingerprintsExcludesUnidentifiedServices(t *testing.T) {
	items := []fingerprint.ServiceFingerprint{
		{IP: "192.0.2.1", Port: 80, Service: "tcpwrapped"},
		{IP: "192.0.2.2", Port: 81, Service: "unknown"},
		{IP: "192.0.2.3", Port: 82, Service: ""},
		{IP: "192.0.2.4", Port: 6379, Service: "redis"},
	}

	got := filterFingerprints(items, reportFilters{ExcludeUnidentified: true})
	if len(got) != 1 || got[0].Service != "redis" {
		t.Fatalf("filtered fingerprints = %#v, want only redis", got)
	}
}

func TestServiceFacetsIgnoreCurrentServiceFilters(t *testing.T) {
	items := []fingerprint.ServiceFingerprint{
		{IP: "192.0.2.1", Port: 80, Service: "redis"},
		{IP: "192.0.2.1", Port: 81, Service: "redis"},
		{IP: "192.0.2.1", Port: 82, Service: "unknown"},
		{IP: "192.0.2.1", Port: 83, Service: ""},
		{IP: "192.0.2.2", Port: 84, Service: "http"},
	}
	filters := reportFilters{IP: "192.0.2.1", Service: "redis", ExcludeUnidentified: true}

	facets := buildServiceFacets(filterFingerprints(items, filters.withoutServiceFilters()))
	if len(facets) != 3 {
		t.Fatalf("facets = %#v, want redis, unknown, and empty", facets)
	}
	if facets[0].RawValue != "" || facets[0].Label != "未识别（空）" || facets[0].Count != 1 {
		t.Fatalf("empty facet = %#v", facets[0])
	}
	if facets[1].RawValue != "redis" || facets[1].Count != 2 {
		t.Fatalf("redis facet = %#v", facets[1])
	}
	if facets[2].RawValue != "unknown" || facets[2].Count != 1 {
		t.Fatalf("unknown facet = %#v", facets[2])
	}
}

func TestReportFiltersFromValuesParsesUnidentifiedExclusion(t *testing.T) {
	filters := reportFiltersFromValues(url.Values{"exclude_unidentified": {"1"}})
	if !filters.ExcludeUnidentified {
		t.Fatal("ExcludeUnidentified = false, want true")
	}
}

func TestFilterFindingsExcludesUnidentifiedSingleProtocolFallback(t *testing.T) {
	for _, service := range []string{"tcpwrapped", "unknown", ""} {
		t.Run(service, func(t *testing.T) {
			fps := []fingerprint.ServiceFingerprint{{IP: "192.0.2.1", Port: 80, Protocol: "tcp", Service: service}}
			findings := []report.Finding{
				{IP: "192.0.2.1", Port: 80, Protocol: "", ID: "attached"},
				{IP: "192.0.2.2", Port: 81, Protocol: "", ID: "isolated"},
			}

			got := filterFindings(findings, fps, reportFilters{ExcludeUnidentified: true})
			if len(got) != 1 || got[0].ID != "isolated" {
				t.Fatalf("filtered findings = %#v, want only isolated finding", got)
			}
		})
	}
}
