package web

import (
	"net/url"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestPaginateFingerprintsClampsPageAndPreservesFilters(t *testing.T) {
	items := make([]fingerprint.ServiceFingerprint, 11)
	page := paginateFingerprints(items, 99, url.Values{"q": {"redis"}}, "assets_page", "assets_size", 10)
	if page.Page != 2 || page.TotalPages != 2 || len(page.Items.([]fingerprint.ServiceFingerprint)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	if page.PrevURL != "?assets_page=1&q=redis" || page.NextURL != "?assets_page=3&q=redis" {
		t.Fatalf("urls = %q %q", page.PrevURL, page.NextURL)
	}
}
