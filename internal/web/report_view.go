package web

import (
	"net/url"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// reportViewModel is the shaped data the report.html template renders. It is the
// named view model behind what reportDetail previously assembled inline as a
// map[string]any, fusing HTTP plumbing with view shaping. Field names match the
// former map keys so html/template accesses them unchanged.
type reportViewModel struct {
	Run                    store.ScanRun
	RunMeta                runMetaView
	Filters                reportFilters
	Fingerprints           any
	Findings               any
	Risk                   riskSummary
	CommandTools           map[string]commandToolsView
	AssetPage              reportPage
	FindingPage            reportPage
	HostPage               reportPage
	AssetView              string
	Vulnerabilities        []report.VulnerabilityDelivery
	PendingVulnerabilities []report.VulnerabilityDelivery
	CatalogStatus          string
	CatalogDiagnostics     []knowledgebase.Diagnostic
	AssetTXTIP             string
	AssetTXTIPPort         string
	AssetTXTURL            string
	AssetCSV               string
	ExportJSON             string
	ExportHTML             string
	ExportCSV              string
}

// reportViewInput bundles what buildReportViewModel needs. Fingerprints and
// Findings are already filtered; CommandTools is computed by the handler (it
// depends on server-held tool config via buildCommand). Everything else the
// builder does is pure shaping over a plain url.Values.
type reportViewInput struct {
	Run          store.ScanRun
	Fingerprints []fingerprint.ServiceFingerprint
	Findings     []report.Finding
	Query        url.Values
	Catalog      *knowledgebase.Catalog
	CommandTools map[string]commandToolsView
}

// buildReportViewModel shapes the HTML report view: parses the view mode, builds
// vulnerability deliveries, paginates assets/findings/hosts, and renders the
// export/asset link URLs — all from a plain url.Values (no *http.Request).
// This is the pure shaping that previously lived inline in reportDetail; it is
// unit-testable without an HTTP server or a SQLite database.
func buildReportViewModel(in reportViewInput) reportViewModel {
	query := in.Query
	view := query.Get("view")
	if view != "hosts" && view != "vulnerabilities" {
		view = "ports"
	}
	var vulnerabilities []report.VulnerabilityDelivery
	var pendingVulnerabilities []report.VulnerabilityDelivery
	if view == "vulnerabilities" {
		vulnerabilities = report.BuildMatchedVulnerabilityDeliveries(in.Findings, in.Catalog)
		pendingVulnerabilities = report.BuildPendingVulnerabilityDeliveries(in.Findings, in.Catalog)
	}
	assetPage := paginateFingerprints(in.Fingerprints, parsePage(query.Get("assets_page")), query, "assets_page", "assets_size", parseSize(query.Get("assets_size")))
	findingPage := paginateFindings(in.Findings, parsePage(query.Get("findings_page")), query, "findings_page", "findings_size", parseSize(query.Get("findings_size")))
	hostPage := paginateHostAssets(groupFingerprintsByHost(in.Fingerprints), parsePage(query.Get("assets_page")), query, "assets_page", "assets_size", parseSize(query.Get("assets_size")))
	copyBase := cloneValues(query)
	copyBase.Del("assets_page")
	copyBase.Del("findings_page")
	copyBase.Del("assets_size")
	copyBase.Del("findings_size")
	runID := in.Run.RunID
	return reportViewModel{
		Run:                    in.Run,
		RunMeta:                newRunMetaView(in.Run),
		Filters:                reportFiltersFromValues(query),
		Fingerprints:           assetPage.Items,
		Findings:               findingPage.Items,
		Risk:                   summarizeRisk(in.Findings),
		CommandTools:           in.CommandTools,
		AssetPage:              assetPage,
		FindingPage:            findingPage,
		HostPage:               hostPage,
		AssetView:              view,
		Vulnerabilities:        vulnerabilities,
		PendingVulnerabilities: pendingVulnerabilities,
		CatalogStatus:          string(in.Catalog.Status()),
		CatalogDiagnostics:     in.Catalog.Diagnostics(),
		AssetTXTIP:             "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "ip"),
		AssetTXTIPPort:         "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "ip_port"),
		AssetTXTURL:            "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "url"),
		AssetCSV:               "/reports/" + runID + "/assets.csv?" + copyBase.Encode(),
		ExportJSON:             "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "json"),
		ExportHTML:             "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "html"),
		ExportCSV:              "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "csv"),
	}
}
