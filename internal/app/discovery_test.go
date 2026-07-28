package app

import (
	"context"
	"testing"
)

func TestDiscoveryModeFromConfigSnapshotDefaultsSafely(t *testing.T) {
	for _, test := range []struct {
		snapshot string
		want     string
	}{
		{snapshot: `{"discovery_mode":"assume-up"}`, want: DiscoveryAssumeUp},
		{snapshot: `{"discovery_mode":"unknown"}`, want: DiscoveryAuto},
		{snapshot: `not json`, want: DiscoveryAuto},
		{snapshot: `{}`, want: DiscoveryAuto},
	} {
		if got := DiscoveryModeFromConfigSnapshot(test.snapshot); got != test.want {
			t.Fatalf("DiscoveryModeFromConfigSnapshot(%q) = %q, want %q", test.snapshot, got, test.want)
		}
	}
}

func TestRunScanRejectsInvalidDiscoveryModeBeforeUsingDependencies(t *testing.T) {
	err := RunScan(context.Background(), nil, nil, ScanOptions{DiscoveryMode: "invalid"})
	if err == nil || err.Error() != "invalid discovery mode: invalid (expected auto or assume-up)" {
		t.Fatalf("RunScan error = %v", err)
	}
}
