package ports

import (
	"os"
	"path/filepath"
	"testing"
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
