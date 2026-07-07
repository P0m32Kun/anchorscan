package config

import (
	"reflect"
	"testing"
)

func TestResolveScanAppliesProfileAndOverrides(t *testing.T) {
	cfg := Config{Profiles: map[string]Profile{
		"slow": {
			HostWorkers: 1,
			ToolArgs: ToolArgs{
				Nmap:   []string{"-T2"},
				Nuclei: []string{"-rate-limit", "10"},
			},
		},
	}}
	cfg.Scan.Profile = "slow"

	got, err := ResolveScan(cfg, Overrides{HostWorkers: 2, NmapArgs: `-T3 --max-retries 2`})
	if err != nil {
		t.Fatalf("ResolveScan returned error: %v", err)
	}
	if got.ProfileName != "slow" || got.HostWorkers != 2 {
		t.Fatalf("unexpected effective scan: %#v", got)
	}
	if !reflect.DeepEqual(got.Nmap, []string{"-T3", "--max-retries", "2"}) {
		t.Fatalf("nmap args mismatch: %#v", got.Nmap)
	}
	if !reflect.DeepEqual(got.Nuclei, []string{"-rate-limit", "10"}) {
		t.Fatalf("nuclei args mismatch: %#v", got.Nuclei)
	}
}

func TestResolveScanRejectsUnknownProfile(t *testing.T) {
	cfg := Config{Profiles: map[string]Profile{"normal": {HostWorkers: 3}}}
	cfg.Scan.Profile = "missing"
	_, err := ResolveScan(cfg, Overrides{})
	if err == nil {
		t.Fatal("expected error")
	}
}
