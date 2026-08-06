package fingerprint

import "testing"

// TestNormalizeAliases covers the alias table that maps both nmap-style and
// fathom-style service names onto the single normalized key that nse.yaml /
// service-tags.yaml look up. The fathom→nmap parity is asserted end-to-end
// (nmap XML vs fathom JSONL) in internal/tools/fathom_test.go; this file
// fixes the per-name contract.
func TestNormalizeAliases(t *testing.T) {
	cases := []struct {
		service string
		product string
		want    string
	}{
		// fathom short tokens -> nmap/nse.yaml keys (M4.1)
		{"mssql", "Microsoft SQL Server", "ms-sql"},
		{"postgres", "PostgreSQL", "postgresql"},
		{"rabbitmq", "RabbitMQ", "amqp"},
		// nmap-side names pass through unchanged to the same keys
		{"ms-sql", "", "ms-sql"},
		{"postgresql", "", "postgresql"},
		{"amqp", "", "amqp"},

		// existing aliases unchanged
		{"ssl/http", "nginx", "http"},
		{"http-proxy", "", "http"},
		{"microsoft-ds", "", "smb"},
		{"netbios-ssn", "", "smb"},
		{"ms-wbt-server", "", "rdp"},
		{"mariadb", "", "mysql"},
		{"dameng", "", "dameng"},

		// fathom/nmap shared tokens (no alias needed)
		{"redis", "Redis", "redis"},
		{"http", "nginx", "http"},
		{"ssh", "OpenSSH", "ssh"},
		{"mongodb", "MongoDB", "mongodb"},
		{"smb", "SMB", "smb"},
		{"rdp", "RDP", "rdp"},
		{"docker", "", "docker"},
	}
	for _, c := range cases {
		fp := Classify(ServiceFingerprint{IP: "192.0.2.1", Port: 1, Service: c.service, Product: c.product})
		if fp.Normalized != c.want {
			t.Errorf("normalize(service=%q, product=%q) = %q, want %q", c.service, c.product, fp.Normalized, c.want)
		}
	}
}

// TestNormalizeFallsBackToProductWhenServiceEmpty mirrors the nmap fallback
// path: a missing service name is covered by the product alias.
func TestNormalizeFallsBackToProductWhenServiceEmpty(t *testing.T) {
	fp := Classify(ServiceFingerprint{IP: "192.0.2.1", Port: 3306, Product: "mariadb"})
	if fp.Normalized != "mysql" {
		t.Fatalf("product-only mariadb normalized to %q, want mysql", fp.Normalized)
	}
}
