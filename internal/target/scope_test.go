package target

import (
	"reflect"
	"testing"
)

func TestParseScopeExcludesAddressInsideCIDR(t *testing.T) {
	scope, err := ParseScope("192.0.2.0/24", "192.0.2.20")
	if err != nil {
		t.Fatalf("ParseScope returned error: %v", err)
	}

	if got, want := scope.Targets(), []string{"192.0.2.0/24"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Targets = %#v, want %#v", got, want)
	}
	if got, want := scope.Excludes(), []string{"192.0.2.20/32"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Excludes = %#v, want %#v", got, want)
	}
	for _, address := range []string{"192.0.2.1", "192.0.2.254"} {
		if !scope.Allows(address) {
			t.Fatalf("Allows(%q) = false, want true", address)
		}
	}
	if scope.Allows("192.0.2.20") {
		t.Fatal("excluded address remains allowed")
	}
	if scope.Allows("198.51.100.1") {
		t.Fatal("address outside scope remains allowed")
	}
	if got, want := scope.Filter([]string{"192.0.2.20", "192.0.2.1", "198.51.100.1", "192.0.2.1"}), []string{"192.0.2.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Filter = %#v, want %#v", got, want)
	}
}

func TestParseScopeSplitsMixedAddressFamiliesForDiscovery(t *testing.T) {
	scope, err := ParseScope("192.0.2.10,2001:db8::10", "")
	if err != nil {
		t.Fatalf("ParseScope returned error: %v", err)
	}
	discovery := scope.DiscoveryScopes()
	if len(discovery) != 2 || discovery[0].IsIPv6() || !discovery[1].IsIPv6() {
		t.Fatalf("DiscoveryScopes = %#v", discovery)
	}
	if got, want := discovery[0].NmapTargets(), []string{"192.0.2.10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IPv4 targets = %#v, want %#v", got, want)
	}
	if got, want := discovery[1].NmapTargets(), []string{"2001:db8::10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IPv6 targets = %#v, want %#v", got, want)
	}
}

func TestParseScopeNormalizesIPv6AndPrefixExclusions(t *testing.T) {
	scope, err := ParseScope("2001:db8:1::1, 2001:db8:2::ff/120", "2001:db8:2::80/121,2001:db8:1::1")
	if err != nil {
		t.Fatalf("ParseScope returned error: %v", err)
	}
	if got, want := scope.Targets(), []string{"2001:db8:2::/120"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Targets = %#v, want %#v", got, want)
	}
	if got, want := scope.Excludes(), []string{"2001:db8:1::1/128", "2001:db8:2::80/121"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Excludes = %#v, want %#v", got, want)
	}
	if scope.Allows("2001:db8:1::1") || scope.Allows("2001:db8:2::81") {
		t.Fatal("prefix exclusion remains allowed")
	}
	if !scope.Allows("2001:db8:2::7f") {
		t.Fatal("address outside excluded IPv6 prefix is not allowed")
	}
}

func TestParseScopeRejectsUnsafeOrUnsupportedInput(t *testing.T) {
	for _, spec := range []string{"", "host.local", "192.0.2.1-10", "-sV", "192.0.2.1,-Pn"} {
		t.Run(spec, func(t *testing.T) {
			if _, err := ParseScope(spec, ""); err == nil {
				t.Fatalf("ParseScope(%q) returned nil error", spec)
			}
		})
	}
}

func TestParseScopeRejectsOversizedAddressCountBeforeExpansion(t *testing.T) {
	for _, spec := range []string{"192.0.2.0/19", "2001:db8::/115", "192.0.2.1,192.0.2.2,192.0.2.3,192.0.2.4,192.0.2.5"} {
		t.Run(spec, func(t *testing.T) {
			_, err := ParseScope(spec, "")
			if spec == "192.0.2.1,192.0.2.2,192.0.2.3,192.0.2.4,192.0.2.5" {
				if err != nil {
					t.Fatalf("ParseScope(%q) returned error: %v", spec, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseScope(%q) returned nil error", spec)
			}
		})
	}
}
