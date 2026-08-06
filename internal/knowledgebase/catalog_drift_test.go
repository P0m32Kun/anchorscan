package knowledgebase

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// Single-source design (2026-08-05 rework, see docs/plans/catalog-json-
// knowledgebase/spec.md): the release archive no longer ships a catalog copy.
// config/catalog.json must not come back; the knowledge base is configured by
// pointing knowledge_base.path at a clone of the knowledge-base repo
// (Pentest-Playbook, handbook-v3/dist/catalog.json).
const shippedCatalogPath = "../../config/catalog.json"

// producerCatalogSHA256 is the SHA-256 of the frozen producer artifact
// (Pentest-Playbook commit 57d739e, handbook-v3/dist/catalog.json), recorded in
// testdata/README.md. The testdata fixture must stay byte-identical to it so
// the consumer contract remains traceable to the producer checksum.
const producerCatalogSHA256 = "7d8ce203a503f63b8d733e6c07fa10c2f1bbb1daf4d5c0619b61e553f374224e"

// TestNoShippedCatalogCopy guards against the packaged copy silently returning
// (e.g. via the Makefile cp list or a new config/catalog.json file).
func TestNoShippedCatalogCopy(t *testing.T) {
	if _, err := os.Stat(filepath.Clean(shippedCatalogPath)); !os.IsNotExist(err) {
		t.Fatalf("config/catalog.json must not be shipped in the release archive; remove the packaged copy (single-source design): %v", err)
	}
}

// TestFrozenProducerFixtureChecksum locks the testdata fixture to the producer
// artifact checksum. The fixture is test-only (never packaged).
func TestFrozenProducerFixtureChecksum(t *testing.T) {
	frozen, err := os.ReadFile(filepath.Join("testdata", "catalog-v2-real.json"))
	if err != nil {
		t.Fatalf("read frozen producer artifact: %v", err)
	}
	sum := sha256.Sum256(frozen)
	if got := hex.EncodeToString(sum[:]); got != producerCatalogSHA256 {
		t.Fatalf("testdata/catalog-v2-real.json sha256 = %s, want %s (frozen producer artifact)", got, producerCatalogSHA256)
	}
}

// TestEmptyPathDisablesKnowledgeBase asserts the default (empty)
// knowledge_base.path disables the KB with a clear diagnostic.
func TestEmptyPathDisablesKnowledgeBase(t *testing.T) {
	catalog := Load("config/default.yaml.example", "")
	if catalog.Status() != StatusDisabled {
		t.Fatalf("empty knowledge_base.path status = %q, want %q", catalog.Status(), StatusDisabled)
	}
	if got := len(catalog.Search("")); got != 0 {
		t.Fatalf("disabled catalog entries = %d, want 0", got)
	}
	found := false
	for _, d := range catalog.Diagnostics() {
		if d.Status == StatusDisabled && d.Reason != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("disabled catalog must carry a clear diagnostic: %#v", catalog.Diagnostics())
	}
}
