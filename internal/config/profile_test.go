package config

import (
	"os"
	"path/filepath"
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

func TestResolveScanDefaultsV1ConfigWithoutProfilesSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\nscan:\n  ports: top1000\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	got, err := ResolveScan(cfg, Overrides{})
	if err != nil {
		t.Fatalf("ResolveScan returned error: %v", err)
	}
	if got.ProfileName != "normal" || got.HostWorkers != 3 {
		t.Fatalf("unexpected fallback profile: %#v", got)
	}
}

func TestValidateScopeSafeToolArgsRejectsTargetSelectors(t *testing.T) {
	for _, args := range []ToolArgs{
		{Nmap: []string{"198.51.100.10"}},
		{Nmap: []string{"-iL", "targets.txt"}},
		{Nmap: []string{"-iLtargets.txt"}},
		{Rustscan: []string{"-a", "198.51.100.10"}},
		{Httpx: []string{"-u", "https://example.test"}},
		{Httpx: []string{"-target=https://example.test"}},
		{Nuclei: []string{"-target", "https://example.test"}},
		{Nuclei: []string{"-u=https://example.test"}},
		{Nmap: []string{"--max-retries", "-iL"}},
		{Nuclei: []string{"-unknown", "https://example.test"}},
	} {
		if err := ValidateScopeSafeToolArgs(args); err == nil {
			t.Fatalf("ValidateScopeSafeToolArgs(%#v) returned nil", args)
		}
	}
	if err := ValidateScopeSafeToolArgs(builtInProfiles()["normal"].ToolArgs); err != nil {
		t.Fatalf("built-in profile rejected: %v", err)
	}
	if err := ValidateScopeSafeToolArgs(ToolArgs{Nmap: []string{"-T4", "--max-rate=50"}}); err != nil {
		t.Fatalf("allowed tuning args rejected: %v", err)
	}
}
