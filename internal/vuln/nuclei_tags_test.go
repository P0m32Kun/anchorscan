package vuln

import (
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestMatchNucleiTagsFormatsIPv6HostPort(t *testing.T) {
	fp := fingerprint.ServiceFingerprint{IP: "2001:db8::10", Port: 6379, Normalized: "redis"}
	rules := []TagRule{{Service: []string{"redis"}, Target: "hostport"}}
	if got := MatchNucleiTags(fp, HTTPResult{}, rules).Address; got != "[2001:db8::10]:6379" {
		t.Fatalf("Address = %q", got)
	}
}

func TestMatchNucleiTagsSelectsSSHViaTags(t *testing.T) {
	rules := []TagRule{{
		Name:        "ssh",
		Service:     []string{"ssh"},
		NucleiTags:  []string{"ssh"},
		ExcludeTags: []string{"default-login"},
		Target:      "hostport",
	}}
	fp := fingerprint.ServiceFingerprint{IP: "192.168.1.10", Port: 22, Normalized: "ssh"}
	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	if len(got.Tags) == 0 || got.Tags[0] != "ssh" {
		t.Fatalf("Tags = %#v, want [ssh]", got.Tags)
	}
	if len(got.ExcludeTags) == 0 {
		t.Fatalf("ExcludeTags = %#v, want to contain default-login", got.ExcludeTags)
	}
	foundDefaultLogin := false
	for _, et := range got.ExcludeTags {
		if et == "default-login" {
			foundDefaultLogin = true
		}
	}
	if !foundDefaultLogin {
		t.Fatalf("ExcludeTags = %#v, want to contain default-login", got.ExcludeTags)
	}
}

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
	want := MatchResult{Tags: []string{"redis"}, ExcludeTags: []string{"fuzz", "dos"}, Target: "hostport", Address: "192.168.1.10:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: got %#v want %#v", got, want)
	}
}
