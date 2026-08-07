package config

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestInitDefaultKnowledgeBasePathIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.yaml")
	if err := Init(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Single-source design: the release archive ships no catalog and the
	// freshly generated config must not point at any packaged file.
	if cfg.KnowledgeBase.Path != "" {
		t.Fatalf("default knowledge_base.path = %q, want %q (empty = disabled)", cfg.KnowledgeBase.Path, "")
	}
}

func TestInitProfilesMatchShippedExample(t *testing.T) {
	generatedPath := filepath.Join(t.TempDir(), "default.yaml")
	if err := Init(generatedPath); err != nil {
		t.Fatal(err)
	}
	generated, err := Load(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	example, err := Load(filepath.Join("..", "..", "config", "default.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(generated.Profiles, example.Profiles) {
		t.Fatalf("generated profiles = %#v, example profiles = %#v", generated.Profiles, example.Profiles)
	}
}

func TestInitTimeoutsMatchShippedExample(t *testing.T) {
	generatedPath := filepath.Join(t.TempDir(), "default.yaml")
	if err := Init(generatedPath); err != nil {
		t.Fatal(err)
	}
	generated, err := Load(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	example, err := Load(filepath.Join("..", "..", "config", "default.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	if generated.Timeouts.Dameng != "15s" || example.Timeouts.Dameng != "15s" {
		t.Fatalf("dameng timeouts = generated %q, example %q; want 15s", generated.Timeouts.Dameng, example.Timeouts.Dameng)
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

// TestInitFathomConfig asserts the M4.1 fathom additions are present in both
// the generated default config and the shipped example, and that the fathom
// timeout parses as a duration. The fathom tool path is PATH-detected at init
// time (it may be empty when fathom is not installed), so the test only pins
// the timeout field and structural presence, not the resolved path.
func TestInitFathomConfig(t *testing.T) {
	generatedPath := filepath.Join(t.TempDir(), "default.yaml")
	if err := Init(generatedPath); err != nil {
		t.Fatal(err)
	}
	generated, err := Load(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	example, err := Load(filepath.Join("..", "..", "config", "default.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	// fathom timeout must be present and identical between generated and
	// example (single source of truth; see defaultConfig() in init.go).
	if generated.Timeouts.Fathom != example.Timeouts.Fathom {
		t.Fatalf("fathom timeout mismatch: generated %q vs example %q", generated.Timeouts.Fathom, example.Timeouts.Fathom)
	}
	if generated.Timeouts.Fathom == "" {
		t.Fatal("generated fathom timeout is empty; defaultConfig must set it")
	}
	// The timeout must parse. Fathom defaults to "0" (no standalone timeout,
	// same policy as nmap), which Durations() maps to a zero duration.
	durs, err := generated.Timeouts.Durations()
	if err != nil {
		t.Fatalf("Durations() failed: %v", err)
	}
	if durs.Fathom != 0 {
		t.Fatalf("default fathom duration = %v, want 0 (no standalone timeout)", durs.Fathom)
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
