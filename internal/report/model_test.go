package report

import (
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestBuildDeduplicatesFindingsByIPAndID(t *testing.T) {
	report := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "192.168.1.10", Port: 6379, Service: "redis", Product: "redis"},
			{IP: "192.168.1.10", Port: 6380, Service: "redis", Product: "redis"},
		},
		[]Finding{
			{IP: "192.168.1.10", Port: 6379, ID: "redis-default-logins", Source: "nuclei"},
			{IP: "192.168.1.10", Port: 6380, ID: "redis-default-logins", Source: "nuclei"},
		},
	)

	total := 0
	for _, host := range report.Hosts {
		for _, port := range host.Ports {
			total += len(port.Findings)
		}
	}
	if total != 1 {
		t.Fatalf("expected one deduplicated finding, got %#v", report.Hosts)
	}
}

func TestBuildKeepsSameFindingIDAcrossDifferentIPs(t *testing.T) {
	report := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "192.168.1.10", Port: 6379, Service: "redis", Product: "redis"},
			{IP: "192.168.1.20", Port: 6379, Service: "redis", Product: "redis"},
		},
		[]Finding{
			{IP: "192.168.1.10", Port: 6379, ID: "redis-default-logins", Source: "nuclei"},
			{IP: "192.168.1.20", Port: 6379, ID: "redis-default-logins", Source: "nuclei"},
		},
	)

	total := 0
	for _, host := range report.Hosts {
		for _, port := range host.Ports {
			total += len(port.Findings)
		}
	}
	if total != 2 {
		t.Fatalf("expected two findings across different IPs, got %#v", report.Hosts)
	}
}
