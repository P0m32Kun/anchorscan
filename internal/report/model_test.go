package report

import (
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestBuildAttachesProtocollessFindingToUniqueFingerprint(t *testing.T) {
	report := Build(
		[]fingerprint.ServiceFingerprint{{IP: "192.0.2.1", Port: 80, Protocol: "tcp", Service: "http"}},
		[]Finding{{IP: "192.0.2.1", Port: 80, Source: "httpx", ID: "missing-header", Severity: "low"}},
	)
	if len(report.Hosts) != 1 || len(report.Hosts[0].Ports) != 1 {
		t.Fatalf("report ports = %#v, want one fingerprint port", report.Hosts)
	}
	port := report.Hosts[0].Ports[0]
	if port.Protocol != "tcp" || len(port.Findings) != 1 || port.Findings[0].ID != "missing-header" {
		t.Fatalf("port = %#v, want protocol-less finding attached to tcp", port)
	}
}

func TestBuildKeepsProtocollessFindingSeparateWhenFingerprintIsAmbiguous(t *testing.T) {
	report := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "192.0.2.1", Port: 53, Protocol: "tcp", Service: "domain"},
			{IP: "192.0.2.1", Port: 53, Protocol: "udp", Service: "domain"},
		},
		[]Finding{{IP: "192.0.2.1", Port: 53, Source: "httpx", ID: "ambiguous", Severity: "low"}},
	)
	if len(report.Hosts) != 1 || len(report.Hosts[0].Ports) != 3 {
		t.Fatalf("report ports = %#v, want tcp, udp, and protocol-less entries", report.Hosts)
	}
	if port := report.Hosts[0].Ports[0]; port.Protocol != "" || len(port.Findings) != 1 {
		t.Fatalf("first port = %#v, want separate protocol-less finding", port)
	}
}
