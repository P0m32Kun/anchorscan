package ports

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveKeepsTop1000ForRustscanTop(t *testing.T) {
	got, err := Resolve("top1000", t.TempDir())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "top1000" {
		t.Fatalf("unexpected ports: %q", got)
	}
}

func TestResolveAcceptsRustscanRangeAndPortCSV(t *testing.T) {
	for input, want := range map[string]string{
		"1-65535":     "1-65535",
		"1000-100":    "100-1000",
		"80, 443, 22": "80,443,22",
	} {
		got, err := Resolve(input, t.TempDir())
		if err != nil {
			t.Fatalf("Resolve(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveRejectsRemovedAndInvalidFormats(t *testing.T) {
	for _, input := range []string{"top100", "full", "highrisk", "0", "65536", "80,abc", "80,100-200", "1-65536", "0-100"} {
		if _, err := Resolve(input, t.TempDir()); err == nil {
			t.Fatalf("Resolve(%q) unexpectedly succeeded", input)
		}
	}
}

func TestLoadPresetReadsHighriskFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports-highrisk.txt")
	if err := os.WriteFile(path, []byte("502,102,2404\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPreset("highrisk", dir)
	if err != nil {
		t.Fatalf("LoadPreset returned error: %v", err)
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
	got, err := ResolveForConfig("top1000", filepath.Join(root, "custom", "default.yaml"))
	if err != nil {
		t.Fatalf("ResolveForConfig returned error: %v", err)
	}
	if got != "top1000" {
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

	got, err := LoadPreset("highrisk", dir)
	if err != nil {
		t.Fatalf("LoadPreset returned error: %v", err)
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
	got, err := LoadPreset("highrisk", dir)
	if err != nil {
		t.Fatalf("LoadPreset returned error: %v", err)
	}
	if got != "502,102" {
		t.Fatalf("unexpected ports: %q", got)
	}
}
