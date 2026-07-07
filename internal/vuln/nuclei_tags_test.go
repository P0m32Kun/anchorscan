package vuln

import (
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestMatchNucleiTagsUsesServiceAndProductRules(t *testing.T) {
	rules := []TagRule{
		{
			Name:       "redis",
			Service:    []string{"redis"},
			Product:    []string{"redis"},
			NucleiTags: []string{"redis"},
			Target:     "hostport",
		},
	}

	fp := fingerprint.ServiceFingerprint{
		IP:         "192.168.1.10",
		Port:       6379,
		Service:    "redis",
		Product:    "redis",
		Normalized: "redis",
	}

	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	want := MatchResult{Tags: []string{"redis"}, Target: "hostport", Address: "192.168.1.10:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: got %#v want %#v", got, want)
	}
}
