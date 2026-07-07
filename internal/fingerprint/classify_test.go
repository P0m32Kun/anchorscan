package fingerprint

import "testing"

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
