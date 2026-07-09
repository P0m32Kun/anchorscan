package web

import (
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
