package web

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// TestBuildReportViewModel exercises the report view shaping directly — no HTTP
// handler, no SQLite. This is the testability payoff of candidate #2: the
// pagination / view-mode / URL-building that was previously locked behind
// reportDetail's end-to-end setup is now reachable through buildReportViewModel.
func TestBuildReportViewModel(t *testing.T) {
	// 12 fingerprints with a page size of 10 → page 2 holds the last 2.
	fps := make([]fingerprint.ServiceFingerprint, 12)
	for i := range fps {
		fps[i] = fingerprint.ServiceFingerprint{IP: "10.0.0." + strconv.Itoa(i+1), Port: 80, Service: "http"}
	}
	findings := []report.Finding{{IP: "10.0.0.1", Port: 80, Source: "nuclei", Severity: "info"}}
	run := store.ScanRun{RunID: "run-x"}

	q := url.Values{}
	q.Set("view", "ports")
	q.Set("assets_size", "10")
	q.Set("assets_page", "2")

	model := buildReportViewModel(reportViewInput{
		Run:          run,
		Fingerprints: fps,
		Findings:     findings,
		Query:        q,
		Catalog:      &knowledgebase.Catalog{},
	})

	if model.AssetView != "ports" {
		t.Errorf("AssetView = %q, want ports", model.AssetView)
	}
	// ports view does not build vulnerability deliveries
	if model.Vulnerabilities != nil {
		t.Errorf("Vulnerabilities = %v, want nil for ports view", model.Vulnerabilities)
	}
	// pagination: 12 fingerprints, size 10, page 2 → exactly 2 items, has prev
	pageItems, ok := model.AssetPage.Items.([]fingerprint.ServiceFingerprint)
	if !ok {
		t.Fatalf("AssetPage.Items type = %T, want []fingerprint.ServiceFingerprint", model.AssetPage.Items)
	}
	if len(pageItems) != 2 {
		t.Errorf("AssetPage.Items len = %d, want 2 (page 2 of 12, size 10)", len(pageItems))
	}
	if !model.AssetPage.HasPrev {
		t.Error("AssetPage.HasPrev = false, want true on page 2")
	}
	// export / asset link URLs carry the run id and path
	for _, u := range []string{model.ExportHTML} {
		if !strings.Contains(u, "/reports/run-x/export") {
			t.Errorf("export URL %q missing /reports/run-x/export", u)
		}
	}
	if !strings.Contains(model.AssetTXTIPPort, "/reports/run-x/assets.txt") {
		t.Errorf("AssetTXTIPPort = %q, want /reports/run-x/assets.txt", model.AssetTXTIPPort)
	}
	// catalog status/diagnostics flow through from the catalog (zero catalog → empty)
	if model.CatalogStatus != "" {
		t.Errorf("CatalogStatus = %q, want empty for zero catalog", model.CatalogStatus)
	}
}

// TestBuildReportViewModelDefaultsInvalidView confirms an unknown view value
// falls back to the default "ports" view rather than rendering nothing.
func TestBuildReportViewModelDefaultsInvalidView(t *testing.T) {
	q := url.Values{}
	q.Set("view", "bogus")
	model := buildReportViewModel(reportViewInput{
		Run:          store.ScanRun{RunID: "r"},
		Fingerprints: nil,
		Findings:     nil,
		Query:        q,
		Catalog:      &knowledgebase.Catalog{},
	})
	if model.AssetView != "ports" {
		t.Errorf("AssetView = %q, want ports fallback", model.AssetView)
	}
}

func TestVulnerabilityAssetCopyTextCombinesFilteredGroups(t *testing.T) {
	got := vulnerabilityAssetCopyText(
		[]report.VulnerabilityDelivery{{AssetCopyText: "192.0.2.2:443\n192.0.2.1:80"}},
		[]report.VulnerabilityDelivery{{AssetCopyText: "192.0.2.1:80\n[2001:db8::1]:22"}},
	)
	if got != "192.0.2.1:80\n192.0.2.2:443\n[2001:db8::1]:22" {
		t.Fatalf("copy text = %q", got)
	}
}
