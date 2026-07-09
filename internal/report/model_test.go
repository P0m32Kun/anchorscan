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
			{IP: "192.168.1.10", Port: 6379, ID: "redis-default-logins", Source: "nuclei"},
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

func TestBuildKeepsTCPAndUDPSamePortSeparate(t *testing.T) {
	report := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "10.0.0.53", Port: 53, Protocol: "tcp", Service: "domain"},
			{IP: "10.0.0.53", Port: 53, Protocol: "udp", Service: "domain"},
		},
		[]Finding{
			{IP: "10.0.0.53", Port: 53, Protocol: "tcp", ID: "dns-version", Source: "nse"},
			{IP: "10.0.0.53", Port: 53, Protocol: "udp", ID: "dns-version", Source: "nse"},
		},
	)

	if len(report.Hosts) != 1 {
		t.Fatalf("expected one host, got %d", len(report.Hosts))
	}
	ports := report.Hosts[0].Ports
	if len(ports) != 2 {
		t.Fatalf("expected two port reports (tcp+udp), got %d", len(ports))
	}
	byProto := map[string]PortReport{}
	for _, p := range ports {
		byProto[p.Protocol] = p
	}
	if _, ok := byProto["tcp"]; !ok {
		t.Fatalf("missing tcp port report: %#v", ports)
	}
	if _, ok := byProto["udp"]; !ok {
		t.Fatalf("missing udp port report: %#v", ports)
	}
	if len(byProto["tcp"].Findings) != 1 || byProto["tcp"].Findings[0].Protocol != "tcp" {
		t.Fatalf("tcp finding not attached to tcp port: %#v", byProto["tcp"].Findings)
	}
	if len(byProto["udp"].Findings) != 1 || byProto["udp"].Findings[0].Protocol != "udp" {
		t.Fatalf("udp finding not attached to udp port: %#v", byProto["udp"].Findings)
	}
}
