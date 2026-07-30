package fingerprint

import (
	"net"
	"net/url"
	"strconv"
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
		if IsTLS(fp) {
			scheme = "https"
		}
		out.URL = (&url.URL{Scheme: scheme, Host: net.JoinHostPort(fp.IP, strconv.Itoa(fp.Port))}).String()
	}

	return out
}

// IsTLS identifies the Nmap fingerprint evidence that AnchorScan treats as TLS.
// Keep this shared with Classify so URL construction and Nuclei routing agree.
func IsTLS(fp ServiceFingerprint) bool {
	service := strings.ToLower(fp.Service)
	return strings.EqualFold(strings.TrimSpace(fp.Tunnel), "ssl") ||
		strings.Contains(service, "https") ||
		strings.Contains(service, "ssl/http")
}
