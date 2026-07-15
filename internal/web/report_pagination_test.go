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

func TestBuildPageURLsIncludesBoundariesAndWindow(t *testing.T) {
	values := url.Values{"q": {"redis"}}
	links := buildPageURLs(values, "assets_page", 5, 10)
	// Current page 5 of 10: expect first(1), window 3-7, last(10) with gaps.
	labels := make([]string, 0, len(links))
	currentSeen := false
	for _, l := range links {
		labels = append(labels, l.Label)
		if l.Page == 5 && !l.IsCurrent {
			t.Fatalf("page 5 should be marked current")
		}
		if l.Page == 5 {
			currentSeen = true
		}
	}
	if !currentSeen {
		t.Fatalf("current page 5 missing from links: %v", labels)
	}
	// Must contain first and last page numbers.
	want := map[string]bool{"1": false, "10": false, "5": false}
	for _, label := range labels {
		if _, ok := want[label]; ok {
			want[label] = true
		}
	}
	for label, seen := range want {
		if !seen {
			t.Fatalf("page label %q missing from %v", label, labels)
		}
	}
}

func TestBuildPageURLsOmitsGapsForContiguousRange(t *testing.T) {
	values := url.Values{}
	links := buildPageURLs(values, "assets_page", 1, 3)
	for _, l := range links {
		if l.Label == "..." {
			t.Fatalf("unexpected gap for small contiguous range: %#v", links)
		}
	}
	if len(links) != 3 {
		t.Fatalf("expected 3 links for 3 pages, got %d: %#v", len(links), links)
	}
}

func TestBuildPageURLsReturnsNilForZeroTotal(t *testing.T) {
	if links := buildPageURLs(url.Values{}, "assets_page", 1, 0); links != nil {
		t.Fatalf("expected nil for zero total, got %#v", links)
	}
}
