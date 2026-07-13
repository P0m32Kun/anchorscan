package web

import (
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestExportAssetsTXTKeepsCurrentLineFormat(t *testing.T) {
	got := exportAssetsTXT([]fingerprint.ServiceFingerprint{{IP: "192.0.2.1", Port: 443}}, "ip_port")
	if got != "192.0.2.1:443\n" {
		t.Fatalf("export = %q", got)
	}
}
