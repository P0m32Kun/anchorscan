package ports

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveReadsPresetFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports-top100.txt")
	if err := os.WriteFile(path, []byte("80,443,8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("top100", dir)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "80,443,8080" {
		t.Fatalf("unexpected ports: %q", got)
	}
}

func TestResolveHighriskReadsPresetFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports-highrisk.txt")
	if err := os.WriteFile(path, []byte("502,102,2404\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("highrisk", dir)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "502,102,2404" {
		t.Fatalf("unexpected ports: %q", got)
	}
}

func TestResolveForConfigFallsBackToRootConfig(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "ports-top100.txt"), []byte("22,80\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveForConfig("top100", filepath.Join(root, "custom", "default.yaml"))
	if err != nil {
		t.Fatalf("ResolveForConfig returned error: %v", err)
	}
	if got != "22,80" {
		t.Fatalf("unexpected ports: %q", got)
	}
}

func TestSavePresetWithBackupWritesAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports-highrisk.txt")
	if err := os.WriteFile(path, []byte("22,80\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	backup, err := SavePresetWithBackup("highrisk", dir, "  502,102,2404  ", time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SavePresetWithBackup returned error: %v", err)
	}
	if !strings.HasSuffix(backup, ".bak.20260709-120000") {
		t.Fatalf("unexpected backup name: %q", backup)
	}

	got, err := Resolve("highrisk", dir)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "502,102,2404" {
		t.Fatalf("unexpected ports after save: %q", got)
	}
	// backup must preserve the original content
	bak, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if strings.TrimSpace(string(bak)) != "22,80" {
		t.Fatalf("unexpected backup content: %q", bak)
	}
}

func TestSavePresetWithBackupCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := SavePresetWithBackup("highrisk", dir, "502,102", time.Now()); err != nil {
		t.Fatalf("SavePresetWithBackup returned error: %v", err)
	}
	got, err := Resolve("highrisk", dir)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "502,102" {
		t.Fatalf("unexpected ports: %q", got)
	}
}
