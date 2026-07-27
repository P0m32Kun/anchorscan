package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestBackupCommandCreatesArchive(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "scans.sqlite")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.yaml"), []byte("tools:\n  nmap: /bin/nmap\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	deps := cliDeps{openStore: store.Open, now: func() time.Time { return time.Unix(1, 0) }}
	if err := runBackup([]string{
		"--db", dbPath,
		"--config", filepath.Join(configDir, "default.yaml"),
		"--output-dir", filepath.Join(dir, "backups"),
	}, &stdout, &stderr, deps); err != nil {
		t.Fatalf("runBackup returned error: %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "backup=") {
		t.Fatalf("unexpected output: %s", out)
	}
	archivePath := strings.TrimSpace(strings.TrimPrefix(out, "backup="))
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive not created: %v", err)
	}
}

func TestRestoreCommandRejectsMissingArchive(t *testing.T) {
	var stdout bytes.Buffer
	deps := cliDeps{openStore: store.Open, now: func() time.Time { return time.Unix(1, 0) }}
	if err := runRestore([]string{"--db", filepath.Join(t.TempDir(), "data", "scans.sqlite")}, &stdout, deps); err == nil {
		t.Fatal("expected restore to reject missing --archive")
	}
}
