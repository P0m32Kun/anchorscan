package app

import "testing"

func TestDiscoveryModeFromConfigSnapshot(t *testing.T) {
	for _, test := range []struct {
		name     string
		snapshot string
		want     string
	}{
		{name: "assume up", snapshot: `{"discovery_mode":"assume-up"}`, want: "assume-up"},
		{name: "legacy", snapshot: `{"includes":["192.0.2.1/32"]}`, want: "auto"},
		{name: "invalid", snapshot: `{`, want: "auto"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := DiscoveryModeFromConfigSnapshot(test.snapshot); got != test.want {
				t.Fatalf("DiscoveryModeFromConfigSnapshot() = %q, want %q", got, test.want)
			}
		})
	}
}
