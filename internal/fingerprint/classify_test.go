package fingerprint

import "testing"

func TestClassifyFormatsIPv6WebURL(t *testing.T) {
	got := Classify(ServiceFingerprint{IP: "2001:db8::10", Port: 8443, Service: "https"})
	if got.URL != "https://[2001:db8::10]:8443" {
		t.Fatalf("URL = %q", got.URL)
	}
}

func TestClassifyMarksWebFromSSLHTTPService(t *testing.T) {
	fp := ServiceFingerprint{
		IP:      "192.168.1.10",
		Port:    8443,
		Service: "ssl/http",
		Product: "nginx",
		Tunnel:  "ssl",
	}

	got := Classify(fp)
	if !got.IsWeb {
		t.Fatalf("expected web classification: %#v", got)
	}
	if got.URL != "https://192.168.1.10:8443" {
		t.Fatalf("unexpected url: %q", got.URL)
	}
}

func TestIsTLSUsesClassifyEvidence(t *testing.T) {
	tests := []struct {
		name string
		fp   ServiceFingerprint
		want bool
	}{
		{name: "ssl tunnel", fp: ServiceFingerprint{Tunnel: "SSL"}, want: true},
		{name: "https service", fp: ServiceFingerprint{Service: "HTTPS"}, want: true},
		{name: "ssl http service", fp: ServiceFingerprint{Service: "ssl/http"}, want: true},
		{name: "plain http", fp: ServiceFingerprint{Service: "http"}, want: false},
		{name: "unknown", fp: ServiceFingerprint{Service: "unknown"}, want: false},
		{name: "tcpwrapped", fp: ServiceFingerprint{Service: "tcpwrapped"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTLS(tt.fp); got != tt.want {
				t.Fatalf("IsTLS(%#v) = %t, want %t", tt.fp, got, tt.want)
			}
		})
	}
}
