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
	rules := []TagRule{{
		Name:       "redis",
		Service:    []string{"redis"},
		Product:    []string{"redis"},
		NucleiTags: []string{"redis"},
		Target:     "hostport",
	}}
	fp := fingerprint.ServiceFingerprint{
		IP: "192.168.1.10", Port: 6379, Service: "redis", Product: "redis", Normalized: "redis",
	}

	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	want := MatchResult{Tags: []string{"redis"}, ExcludeTags: []string{"fuzz", "dos"}, Target: "hostport", Address: "192.168.1.10:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: got %#v want %#v", got, want)
	}
}

func TestMatchNucleiTagsDoesNotDuplicateSSLTag(t *testing.T) {
	fp := fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: "https", Normalized: "http"}
	rules := []TagRule{{Service: []string{"http"}, NucleiTags: []string{"http", "SSL"}, Target: "hostport"}}

	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	want := []string{"http", "SSL"}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Fatalf("Tags = %#v, want %#v", got.Tags, want)
	}
}

func TestMatchNucleiTagsDoesNotAddSSLToUnidentifiedService(t *testing.T) {
	rules := []TagRule{{Product: []string{"apache spark"}, NucleiTags: []string{"spark"}, Target: "hostport"}}
	for _, service := range []string{"", "unknown", "tcpwrapped"} {
		t.Run(service, func(t *testing.T) {
			fp := fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: service, Product: "Apache Spark", Tunnel: "ssl"}
			got := MatchNucleiTags(fp, HTTPResult{}, rules)
			if !reflect.DeepEqual(got.Tags, []string{"spark"}) {
				t.Fatalf("Tags = %#v, want [spark]", got.Tags)
			}
		})
	}
}

func TestMatchNucleiTagsAddsSSLToMatchedNonWebTLS(t *testing.T) {
	fp := fingerprint.ServiceFingerprint{
		IP: "192.0.2.25", Port: 636, Service: "ldap", Normalized: "ldap", Tunnel: "ssl",
	}
	rules := []TagRule{{
		Service:     []string{"ldap"},
		NucleiTags:  []string{"ldap"},
		ExcludeTags: []string{"default-login"},
		Target:      "hostport",
	}}

	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	want := MatchResult{
		Tags:        []string{"ldap", "ssl"},
		ExcludeTags: []string{"fuzz", "dos", "default-login"},
		Target:      "hostport",
		Address:     "192.0.2.25:636",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchNucleiTags() = %#v, want %#v", got, want)
	}
}

func TestMatchNucleiTagsAddsSSLOnlyForConfirmedTLS(t *testing.T) {
	rules := []TagRule{{
		Name:        "web",
		Service:     []string{"http"},
		NucleiTags:  []string{"http", "misconfig"},
		ExcludeTags: []string{"default-login"},
		Target:      "url",
	}}
	tests := []struct {
		name string
		fp   fingerprint.ServiceFingerprint
		want []string
	}{
		{name: "ssl tunnel", fp: fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: "http", Normalized: "http", Tunnel: "ssl"}, want: []string{"http", "misconfig", "ssl"}},
		{name: "https service", fp: fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: "https", Normalized: "http"}, want: []string{"http", "misconfig", "ssl"}},
		{name: "ssl http service", fp: fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: "ssl/http", Normalized: "http"}, want: []string{"http", "misconfig", "ssl"}},
		{name: "plain http", fp: fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 80, Service: "http", Normalized: "http"}, want: []string{"http", "misconfig"}},
		{name: "unknown", fp: fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: "unknown", Normalized: "unknown"}, want: nil},
		{name: "tcpwrapped", fp: fingerprint.ServiceFingerprint{IP: "192.0.2.1", Port: 443, Service: "tcpwrapped", Normalized: "tcpwrapped"}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchNucleiTags(tt.fp, HTTPResult{URL: "https://192.0.2.1"}, rules)
			if !reflect.DeepEqual(got.Tags, tt.want) {
				t.Fatalf("Tags = %#v, want %#v", got.Tags, tt.want)
			}
			if len(got.Tags) > 0 && (!reflect.DeepEqual(got.ExcludeTags, []string{"fuzz", "dos", "default-login"}) || got.Address != "https://192.0.2.1") {
				t.Fatalf("match invariants changed: %#v", got)
			}
		})
	}
}
