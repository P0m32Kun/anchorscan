package vuln

import (
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestMatchNSEReturnsConfiguredScriptsForNormalizedService(t *testing.T) {
	rules := map[string][]string{
		"redis": {"redis-info"},
	}

	fp := fingerprint.ServiceFingerprint{Normalized: "redis"}
	got := MatchNSE(fp, rules)
	want := []string{"redis-info"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected scripts: got %#v want %#v", got, want)
	}
}
