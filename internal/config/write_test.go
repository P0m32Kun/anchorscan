package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveWithBackupCreatesTimestampedBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	if err := os.WriteFile(path, []byte("scan:\n  ports: top100\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var cfg Config
	cfg.Scan.Ports = "8080"
	cfg.Scan.Profile = "normal"
	cfg.Profiles = map[string]Profile{"normal": {HostWorkers: 1}}

	backup, err := SaveWithBackup(path, cfg, time.Date(2026, 7, 7, 21, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SaveWithBackup returned error: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("expected backup: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Scan.Ports != "8080" {
		t.Fatalf("ports mismatch: %#v", loaded.Scan)
	}
}
