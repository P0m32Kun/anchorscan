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

func TestMatchNucleiTagsReturnsTemplateForSSH(t *testing.T) {
	rules := []TagRule{{
		Name:       "ssh",
		Service:    []string{"ssh"},
		NucleiTags: []string{"ssh"},
		Template:   "/opt/anchorscan/config/nuclei-templates/ssh-mini-brute.yaml",
		Target:     "hostport",
	}}
	fp := fingerprint.ServiceFingerprint{IP: "192.168.1.10", Port: 22, Normalized: "ssh"}
	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	if got.Template != rules[0].Template {
		t.Fatalf("Template = %q, want %q", got.Template, rules[0].Template)
	}
	if len(got.ExcludeTags) != 0 {
		t.Fatalf("template-based match should not carry default excludes, got %#v", got.ExcludeTags)
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
