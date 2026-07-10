package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "default.yaml")

	if err := Init(path); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Scan.Ports != "top1000" {
		t.Fatalf("expected default ports top1000, got %q", cfg.Scan.Ports)
	}
	if cfg.Scan.Profile != "normal" {
		t.Fatalf("expected default profile normal, got %q", cfg.Scan.Profile)
	}
	if len(cfg.Profiles) != 3 {
		t.Fatalf("expected 3 built-in profiles, got %d", len(cfg.Profiles))
	}
	for _, name := range []string{"slow", "normal", "fast"} {
		if _, ok := cfg.Profiles[name]; !ok {
			t.Fatalf("missing built-in profile %q", name)
		}
	}
}

func TestLoadAutoInitsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("precondition: config should not exist")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load of missing config returned error: %v", err)
	}
	if cfg.Scan.Profile != "normal" {
		t.Fatalf("expected auto-generated profile normal, got %q", cfg.Scan.Profile)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}
}

func TestLoadDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")

	original := []byte("scan:\n  ports: 100-1000\n  profile: fast\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Scan.Ports != "100-1000" {
		t.Fatalf("existing config was overwritten: expected ports 100-1000, got %q", cfg.Scan.Ports)
	}
	if cfg.Scan.Profile != "fast" {
		t.Fatalf("existing config was overwritten: expected profile fast, got %q", cfg.Scan.Profile)
	}
}
