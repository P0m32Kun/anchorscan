package web

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestBuildRunReportReadingFiltersAndView(t *testing.T) {
	fps := make([]fingerprint.ServiceFingerprint, 12)
	for i := range fps {
		fps[i] = fingerprint.ServiceFingerprint{IP: "10.0.0." + strconv.Itoa(i+1), Port: 80, Service: "http"}
	}
	findings := []report.Finding{{IP: "10.0.0.1", Port: 80, Source: "nuclei", Severity: "info"}}
	run := store.ScanRun{RunID: "run-x", Status: "completed", ConfigSnapshot: `{"discovery_mode":"assume-up"}`}
	checks := []report.DetectionCheck{
		{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nse", Status: "completed"},
	}

	q := url.Values{}
	q.Set("view", "ports")
	q.Set("assets_size", "10")
	q.Set("assets_page", "2")

	reading := buildRunReportReading(runReportReadingInput{
		Run:             run,
		Fingerprints:    fps,
		Findings:        findings,
		DetectionChecks: checks,
		Query:           q,
		Catalog:         &knowledgebase.Catalog{},
	})

	// All 12 fingerprints make it through filters (no filter applied)
	if len(reading.FilteredFingerprints) != 12 {
		t.Fatalf("FilteredFingerprints = %d, want 12", len(reading.FilteredFingerprints))
	}
	if reading.ViewInput.Run.RunID != "run-x" {
		t.Fatalf("RunID = %q", reading.ViewInput.Run.RunID)
	}
	// filtered findings still there
	if len(reading.FilteredFindings) != 1 {
		t.Fatalf("FilteredFindings = %d", len(reading.FilteredFindings))
	}
	// coverage populated for non-running run
	if reading.DetectionCoverage == nil {
		t.Fatal("DetectionCoverage is nil")
	}
	if reading.Built.DiscoveryMode != "assume-up" {
		t.Fatalf("DiscoveryMode = %q, want assume-up", reading.Built.DiscoveryMode)
	}
}

func TestBuildRunReportReadingInfersView(t *testing.T) {
	q := url.Values{}
	q.Set("view", "vulnerabilities")
	reading := buildRunReportReading(runReportReadingInput{
		Run:          store.ScanRun{RunID: "r"},
		Fingerprints: nil,
		Findings:     nil,
		Query:        q,
		Catalog:      &knowledgebase.Catalog{},
	})
	model := buildReportViewModel(reading.ViewInput)
	if model.AssetView != "vulnerabilities" {
		t.Fatalf("AssetView = %q", model.AssetView)
	}
}

func TestBuildRunReportReadingFiltersFindingsForCommands(t *testing.T) {
	fps := []fingerprint.ServiceFingerprint{
		{IP: "10.0.0.1", Port: 80, Service: "http"},
	}
	findings := []report.Finding{
		{IP: "10.0.0.1", Port: 80, Source: "nuclei", Severity: "high", ID: "redis"},
		{IP: "10.0.0.2", Port: 443, Source: "nse", Severity: "info", ID: "http-title"},
	}
	q := url.Values{}
	q.Set("source", "nuclei")
	q.Set("ip", "10.0.0.1")

	reading := buildRunReportReading(runReportReadingInput{
		Run:          store.ScanRun{RunID: "r"},
		Fingerprints: fps,
		Findings:     findings,
		Query:        q,
		Catalog:      &knowledgebase.Catalog{},
	})

	if len(reading.FilteredFindings) != 1 || reading.FilteredFindings[0].ID != "redis" {
		t.Fatalf("FilteredFindings = %#v", reading.FilteredFindings)
	}
}

func TestBuildRunReportReadingExportLinks(t *testing.T) {
	run := store.ScanRun{RunID: "run-links", Status: "completed"}
	q := url.Values{}
	q.Set("view", "ports")
	reading := buildRunReportReading(runReportReadingInput{
		Run:             run,
		Fingerprints:    nil,
		Findings:        nil,
		DetectionChecks: nil,
		Query:           q,
		Catalog:         &knowledgebase.Catalog{},
	})
	model := buildReportViewModel(reading.ViewInput)
	if !strings.Contains(model.ExportHTML, "/reports/run-links/export") {
		t.Fatalf("ExportHTML = %q", model.ExportHTML)
	}
	if !strings.Contains(model.AssetTXTIPPort, "/reports/run-links/assets.txt") {
		t.Fatalf("AssetTXTIPPort = %q", model.AssetTXTIPPort)
	}
}

func TestBuildRunReportReadingSkipsDetectionCoverageForRunning(t *testing.T) {
	run := store.ScanRun{RunID: "run-live", Status: "running"}
	q := url.Values{}

	reading := buildRunReportReading(runReportReadingInput{
		Run:             run,
		Fingerprints:    nil,
		Findings:        nil,
		DetectionChecks: []report.DetectionCheck{{IP: "10.0.0.1", Port: 80, Engine: "nse", Status: "completed"}},
		Query:           q,
		Catalog:         &knowledgebase.Catalog{},
	})

	// running runs skip detection checks in the built report
	if reading.DetectionCoverage != nil {
		t.Fatal("DetectionCoverage should be nil for running runs")
	}
}

func TestBuiltReportForHTMLExport(t *testing.T) {
	fps := []fingerprint.ServiceFingerprint{
		{IP: "10.0.0.1", Port: 80, Service: "http", Protocol: "tcp"},
	}
	findings := []report.Finding{
		{IP: "10.0.0.1", Port: 80, Source: "nuclei", Severity: "critical", ID: "cve-2024", Summary: "RCE"},
	}
	checks := []report.DetectionCheck{
		{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nse", Status: "completed", StartedAt: report.DetectionCheckTime(time.Unix(1, 0)), FinishedAt: report.DetectionCheckTime(time.Unix(2, 0))},
	}

	reading := buildRunReportReading(runReportReadingInput{
		Run:             store.ScanRun{RunID: "r", Status: "completed"},
		Fingerprints:    fps,
		Findings:        findings,
		DetectionChecks: checks,
		Query:           url.Values{},
		Catalog:         &knowledgebase.Catalog{},
	})

	if len(reading.Built.Hosts) != 1 {
		t.Fatalf("Built.Hosts = %d", len(reading.Built.Hosts))
	}
	if reading.Built.DetectionCoverage == nil {
		t.Fatal("Built.DetectionCoverage is nil")
	}
}

func TestFilteredChecksMatchFilteredFingerprints(t *testing.T) {
	fps := []fingerprint.ServiceFingerprint{
		{IP: "10.0.0.1", Port: 80, Service: "http", Protocol: "tcp"},
		{IP: "10.0.0.2", Port: 443, Service: "https", Protocol: "tcp"},
	}
	checks := []report.DetectionCheck{
		{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nse", Status: "completed"},
		{IP: "10.0.0.2", Port: 443, Protocol: "tcp", Engine: "nuclei", Status: "failed"},
	}

	q := url.Values{}
	q.Set("ip", "10.0.0.1")

	reading := buildRunReportReading(runReportReadingInput{
		Run:             store.ScanRun{RunID: "r", Status: "completed"},
		Fingerprints:    fps,
		Findings:        nil,
		DetectionChecks: checks,
		Query:           q,
		Catalog:         &knowledgebase.Catalog{},
	})

	if len(reading.FilteredChecks) != 1 || reading.FilteredChecks[0].Engine != "nse" {
		t.Fatalf("FilteredChecks = %#v", reading.FilteredChecks)
	}
}
