package app

import (
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestManualReviewFindingsAddsBlueKeepForRDP3389(t *testing.T) {
	findings := ManualReviewFindings(fingerprint.ServiceFingerprint{IP: "192.0.2.10", Port: 3389, Service: "ms-wbt-server", Product: "Microsoft Terminal Services"})

	if len(findings) != 1 {
		t.Fatalf("findings = %#v", findings)
	}
	got := findings[0]
	if got.Source != "manual-review" || got.ID != "manual-review:CVE-2019-0708" || got.Severity != "critical" {
		t.Fatalf("finding = %#v", got)
	}
	if !strings.Contains(got.Output, "external validation") {
		t.Fatalf("output = %q", got.Output)
	}
}

func TestManualReviewFindingsSkipsNonRDP(t *testing.T) {
	findings := ManualReviewFindings(fingerprint.ServiceFingerprint{IP: "192.0.2.10", Port: 22, Service: "ssh"})
	if len(findings) != 0 {
		t.Fatalf("findings = %#v", findings)
	}
}
