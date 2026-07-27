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
