package fingerprint

import (
	"fmt"
	"strings"
)

func Classify(fp ServiceFingerprint) ServiceFingerprint {
	out := fp
	service := strings.ToLower(fp.Service)
	product := strings.ToLower(fp.Product)

	out.Normalized = normalizeService(fp.Service, fp.Product)
	if strings.Contains(service, "http") ||
		strings.Contains(product, "nginx") ||
		strings.Contains(product, "apache") ||
		strings.Contains(product, "tomcat") ||
		strings.Contains(product, "iis") ||
		strings.Contains(product, "caddy") ||
		strings.Contains(product, "jetty") ||
		strings.Contains(product, "traefik") ||
		strings.Contains(product, "weblogic") {
		out.IsWeb = true
		scheme := "http"
		if fp.Tunnel == "ssl" || strings.Contains(service, "https") || strings.Contains(service, "ssl/http") {
			scheme = "https"
		}
		out.URL = fmt.Sprintf("%s://%s:%d", scheme, fp.IP, fp.Port)
	}

	return out
}
