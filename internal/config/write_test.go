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
	if err := os.WriteFile(path, []byte("scan:\n  ports: top1000\n"), 0o644); err != nil {
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

func TestSaveRawWithBackupWritesValidatedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	if err := os.WriteFile(path, []byte("scan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	raw := "tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\nscan:\n  ports: 8080,6379\n  profile: slow\nprofiles:\n  slow:\n    host_workers: 1\n"

	backup, err := SaveRawWithBackup(path, raw, time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SaveRawWithBackup returned error: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("expected backup: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Scan.Ports != "8080,6379" || loaded.Scan.Profile != "slow" {
		t.Fatalf("unexpected config: %#v", loaded)
	}
}

func TestSaveRawWithBackupRejectsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	original := "scan:\n  ports: top1000\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := SaveRawWithBackup(path, "scan: [broken\n", time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected invalid yaml error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("config should remain unchanged: %s", data)
	}
}

func TestSaveWithBackupRejectsInvalidToolTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	if err := os.WriteFile(path, []byte("scan:\n  ports: top1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveWithBackup(path, Config{Timeouts: ToolTimeouts{Nmap: "-1s"}}, time.Now()); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}

func TestSaveRawWithBackupRejectsInvalidToolTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	original := "scan:\n  ports: top1000\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveRawWithBackup(path, "timeouts:\n  httpx: nonsense\n", time.Now()); err == nil {
		t.Fatal("expected invalid timeout error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("config should remain unchanged: %s", data)
	}
}
