package fingerprint

import "strings"

var aliases = map[string]string{
	"ssl/http":      "http",
	"http-proxy":    "http",
	"microsoft-ds":  "smb",
	"netbios-ssn":   "smb",
	"ms-wbt-server": "rdp",
	"mariadb":       "mysql",
	"dameng":        "dameng",

	// fathom service-name → nmap-style normalized name. fathom emits its own
	// short service tokens (see ~/DEV/fathom src/rules.rs RULES and the
	// banner/http probes in src/fingerprint.rs). These three diverge from the
	// nse.yaml/service-tags.yaml keys that normalize.go output must satisfy so
	// the NSE script lookup and nuclei tag routing keep working:
	//   - fathom "mssql"    ↔ nmap/nse.yaml key "ms-sql"
	//   - fathom "postgres" ↔ nmap/nse.yaml key "postgresql"
	//   - fathom "rabbitmq" ↔ nmap/nse.yaml key "amqp" (NSE amqp-* scripts)
	// The parity test in normalize_test.go asserts that nmap XML output and
	// fathom JSONL output normalize identically for the same logical service.
	"mssql":    "ms-sql",
	"postgres": "postgresql",
	"rabbitmq": "amqp",
}

func normalizeService(service string, product string) string {
	service = strings.ToLower(strings.TrimSpace(service))
	product = strings.ToLower(strings.TrimSpace(product))

	if alias, ok := aliases[service]; ok {
		return alias
	}
	if alias, ok := aliases[product]; ok {
		return alias
	}
	if service != "" {
		return service
	}
	return product
}
