package web

import (
	"net/url"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// runReportReadingInput collects the raw facts and query for a run report
// reading. Handlers load facts from Store, then call buildRunReportReading
// once to obtain the filtered, built, and shaped result.
type runReportReadingInput struct {
	Run             store.ScanRun
	Fingerprints    []fingerprint.ServiceFingerprint
	Findings        []report.Finding
	DetectionChecks []report.DetectionCheck
	Query           url.Values
	Catalog         *knowledgebase.Catalog
}

// runReportReading is the unified reading model shared by the console page,
// HTML export, assets.txt, and command endpoints. Handlers grab whichever
// fields they need; command generation stays in internal/report.
type runReportReading struct {
	FilteredFingerprints []fingerprint.ServiceFingerprint
	FilteredFindings     []report.Finding
	FilteredChecks       []report.DetectionCheck
	ServiceFacets        []serviceFacet
	DetectionCoverage    *report.DetectionCoverage
	Built                report.ScanReport
	ViewInput            reportViewInput
}

func buildRunReportReading(in runReportReadingInput) runReportReading {
	filters := reportFiltersFromValues(in.Query)
	view := in.Query.Get("view")
	vulnerabilityView := view == "vulnerabilities"

	filteredFingerprints := filterFingerprints(in.Fingerprints, filters)
	serviceFacets := buildServiceFacets(filterFingerprints(in.Fingerprints, filters.withoutServiceFilters()))
	filteredFindings := filterFindingsForView(in.Findings, in.Fingerprints, filters, in.Catalog, vulnerabilityView)
	filteredChecks := filterDetectionChecks(in.DetectionChecks, filteredFingerprints)

	built := report.Build(filteredFingerprints, filteredFindings)
	var detectionCoverage *report.DetectionCoverage
	if in.Run.Status != "running" {
		built = report.BuildWithScanDataAndDetectionChecks(filteredFingerprints, filteredFindings, report.ScanData{
			DiscoveryMode: app.DiscoveryModeFromConfigSnapshot(in.Run.ConfigSnapshot),
		}, filteredChecks)
		detectionCoverage = built.DetectionCoverage
	}

	viewInput := reportViewInput{
		Run:               in.Run,
		Fingerprints:      filteredFingerprints,
		Findings:          filteredFindings,
		DetectionChecks:   built.DetectionChecks,
		DetectionCoverage: detectionCoverage,
		ServiceFacets:     serviceFacets,
		Query:             in.Query,
		Catalog:           in.Catalog,
		// CommandTools must be computed by the handler — it depends on
		// server-held tool config via buildCommand.
	}

	return runReportReading{
		FilteredFingerprints: filteredFingerprints,
		FilteredFindings:     filteredFindings,
		FilteredChecks:       filteredChecks,
		ServiceFacets:        serviceFacets,
		DetectionCoverage:    detectionCoverage,
		Built:                built,
		ViewInput:            viewInput,
	}
}
