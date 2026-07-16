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

func TestBuildWithScanDataIncludesAliveIPsAndOpenPorts(t *testing.T) {
	rpt := BuildWithScanData(
		[]fingerprint.ServiceFingerprint{
			{IP: "10.0.0.1", Port: 80, Service: "http"},
		},
		nil,
		ScanData{
			// 10.0.0.2 is alive but has no fingerprints — must still appear.
			// 10.0.0.3 has open ports nmap could not fingerprint.
			AliveIPs:  []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			OpenPorts: map[string][]int{"10.0.0.1": {80, 443}, "10.0.0.3": {22, 8080}},
		},
	)

	// Alive IP list must be complete, sorted, and deduplicated.
	if len(rpt.AliveIPs) != 3 || rpt.AliveIPs[0] != "10.0.0.1" || rpt.AliveIPs[2] != "10.0.0.3" {
		t.Fatalf("alive IPs = %#v", rpt.AliveIPs)
	}

	hostByIP := map[string]HostReport{}
	for _, h := range rpt.Hosts {
		hostByIP[h.IP] = h
	}

	// 10.0.0.2 alive but no ports at all.
	if _, ok := hostByIP["10.0.0.2"]; !ok {
		t.Fatalf("alive host 10.0.0.2 with no ports missing from report")
	}
	// 10.0.0.3 has open ports but no fingerprint details.
	host3, ok := hostByIP["10.0.0.3"]
	if !ok {
		t.Fatalf("host 10.0.0.3 missing from report")
	}
	if len(host3.OpenPorts) != 2 || host3.OpenPorts[0] != 22 || host3.OpenPorts[1] != 8080 {
		t.Fatalf("open ports for 10.0.0.3 = %#v", host3.OpenPorts)
	}
	if len(host3.Ports) != 0 {
		t.Fatalf("host 10.0.0.3 should have no fingerprinted ports, got %#v", host3.Ports)
	}
	// 10.0.0.1 has both fingerprint details and raw open ports.
	host1 := hostByIP["10.0.0.1"]
	if len(host1.OpenPorts) != 2 || len(host1.Ports) != 1 {
		t.Fatalf("host 10.0.0.1 openPorts=%#v ports=%#v", host1.OpenPorts, host1.Ports)
	}
}

func TestBuildWithoutScanDataOmitsNewFields(t *testing.T) {
	// The legacy Build entry point must not populate the new fields.
	rpt := Build(
		[]fingerprint.ServiceFingerprint{{IP: "10.0.0.1", Port: 80, Service: "http"}},
		nil,
	)
	if len(rpt.AliveIPs) != 0 {
		t.Fatalf("legacy Build should not set AliveIPs, got %#v", rpt.AliveIPs)
	}
	for _, h := range rpt.Hosts {
		if len(h.OpenPorts) != 0 {
			t.Fatalf("legacy Build should not set OpenPorts, host=%#v", h)
		}
	}
}

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
