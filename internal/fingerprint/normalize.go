package fingerprint

import "strings"

var aliases = map[string]string{
	"ssl/http":    "http",
	"http-proxy":  "http",
	"microsoft-ds": "smb",
	"netbios-ssn": "smb",
	"ms-wbt-server": "rdp",
	"mariadb":     "mysql",
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
