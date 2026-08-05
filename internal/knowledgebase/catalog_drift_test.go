package knowledgebase

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// packagedCatalogPath is the catalog shipped in the release archive next to
// the default config (config/catalog.json). It must stay byte-identical to the
// frozen producer artifact in testdata so the release catalog remains
// traceable to its producer checksum at all times.
const packagedCatalogPath = "../../config/catalog.json"

// producerCatalogSHA256 is the SHA-256 of the frozen producer artifact
// (Pentest-Playbook commit 57d739e, handbook-v3/dist/catalog.json), recorded in
// testdata/README.md.
const producerCatalogSHA256 = "7d8ce203a503f63b8d733e6c07fa10c2f1bbb1daf4d5c0619b61e553f374224e"

func TestShippedCatalogMatchesFrozenProducerArtifact(t *testing.T) {
	shipped, err := os.ReadFile(filepath.Clean(packagedCatalogPath))
	if err != nil {
		t.Fatalf("read shipped catalog %s: %v", packagedCatalogPath, err)
	}
	frozen, err := os.ReadFile(filepath.Join("testdata", "catalog-v2-real.json"))
	if err != nil {
		t.Fatalf("read frozen producer artifact: %v", err)
	}
	if string(shipped) != string(frozen) {
		t.Fatalf("config/catalog.json drifted from testdata/catalog-v2-real.json; both must stay byte-identical to the producer artifact")
	}
	sum := sha256.Sum256(shipped)
	if got := hex.EncodeToString(sum[:]); got != producerCatalogSHA256 {
		t.Fatalf("shipped catalog sha256 = %s, want %s (frozen producer artifact)", got, producerCatalogSHA256)
	}
}

func TestShippedCatalogLoadsReadyAsDefaultKnowledgeBase(t *testing.T) {
	configPath := filepath.Join(filepath.Dir(packagedCatalogPath), "default.yaml.example")
	catalog := Load(configPath, "catalog.json")
	if catalog.Status() != StatusReady {
		t.Fatalf("default knowledge_base.path catalog.json status = %q, want %q: %#v", catalog.Status(), StatusReady, catalog.Diagnostics())
	}
	if got := len(catalog.Search("")); got != 188 {
		t.Fatalf("default catalog entries = %d, want 188", got)
	}
}
