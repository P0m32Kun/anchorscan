package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/report"
)

func (s *server) reportDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/reports/")
	if strings.HasSuffix(path, "/commands") {
		s.reportNucleiCommand(w, r, strings.TrimSuffix(path, "/commands"))
		return
	}
	format := ""
	exportFormat := ""
	assetExport := ""
	runID := path
	if strings.HasSuffix(path, "/export") {
		exportFormat = strings.TrimSpace(r.URL.Query().Get("format"))
		runID = strings.TrimSuffix(path, "/export")
	}
	if strings.HasSuffix(path, "/assets.txt") {
		assetExport = "txt"
		runID = strings.TrimSuffix(path, "/assets.txt")
	}
	if strings.HasSuffix(path, "/assets.csv") {
		assetExport = "csv"
		runID = strings.TrimSuffix(path, "/assets.csv")
	}
	if assetExport == "" && strings.HasSuffix(path, ".json") {
		format = "json"
		runID = strings.TrimSuffix(path, ".json")
	}
	if assetExport == "" && strings.HasSuffix(path, ".html") {
		format = "html"
		runID = strings.TrimSuffix(path, ".html")
	}

	run, err := s.store.GetScanRun(runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fps, err := s.store.ListFingerprints(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	findings, err := s.store.ListFindings(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filters := reportFiltersFromValues(r.URL.Query())
	filteredFingerprints := filterFingerprints(fps, filters)
	filteredFindings := filterFindings(findings, fps, filters)
	filteredBuilt := report.Build(filteredFingerprints, filteredFindings)
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(filteredBuilt)
	case "html":
		tmp := filepath.Join(os.TempDir(), "anchorscan-report-"+runID+".html")
		if err := report.WriteHTML(tmp, filteredBuilt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmp)
		http.ServeFile(w, r, tmp)
	default:
		if exportFormat != "" {
			filename := "anchorscan-" + runID
			switch exportFormat {
			case "json":
				w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.json"`)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(filteredBuilt)
				return
			case "html":
				tmp := filepath.Join(os.TempDir(), "anchorscan-export-"+runID+".html")
				if err := report.WriteHTML(tmp, filteredBuilt); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer os.Remove(tmp)
				w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.html"`)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				http.ServeFile(w, r, tmp)
				return
			case "csv":
				data, err := exportFindingsCSV(filteredFindings, filteredFingerprints)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				_, _ = io.WriteString(w, data)
				return
			default:
				http.Error(w, "unknown export format", http.StatusBadRequest)
				return
			}
		}
		if assetExport == "txt" {
			w.Header().Set("Content-Disposition", `attachment; filename="`+runID+`-assets.txt"`)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, exportAssetsTXT(filteredFingerprints, r.URL.Query().Get("kind")))
			return
		}
		if assetExport == "csv" {
			data, err := exportAssetsCSV(filteredFingerprints)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Disposition", `attachment; filename="`+runID+`-assets.csv"`)
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			_, _ = io.WriteString(w, data)
			return
		}
		query := r.URL.Query()
		view := query.Get("view")
		if view != "hosts" && view != "vulnerabilities" {
			view = "ports"
		}
		var vulnerabilities []report.VulnerabilityDelivery
		var pendingVulnerabilities []report.VulnerabilityDelivery
		if view == "vulnerabilities" {
			vulnerabilities = report.BuildMatchedVulnerabilityDeliveries(filteredFindings, s.catalog)
			pendingVulnerabilities = report.BuildPendingVulnerabilityDeliveries(filteredFindings, s.catalog)
		}
		assetPage := paginateFingerprints(filteredFingerprints, parsePage(query.Get("assets_page")), query, "assets_page", "assets_size", parseSize(query.Get("assets_size")))
		findingPage := paginateFindings(filteredFindings, parsePage(query.Get("findings_page")), query, "findings_page", "findings_size", parseSize(query.Get("findings_size")))
		hostPage := paginateHostAssets(groupFingerprintsByHost(filteredFingerprints), parsePage(query.Get("assets_page")), query, "assets_page", "assets_size", parseSize(query.Get("assets_size")))
		copyBase := cloneValues(query)
		copyBase.Del("assets_page")
		copyBase.Del("findings_page")
		copyBase.Del("assets_size")
		copyBase.Del("findings_size")
		render(w, "templates/report.html", map[string]any{
			"Run":                    run,
			"RunMeta":                newRunMetaView(run),
			"Filters":                filters,
			"Fingerprints":           assetPage.Items,
			"Findings":               findingPage.Items,
			"AssetPage":              assetPage,
			"FindingPage":            findingPage,
			"HostPage":               hostPage,
			"AssetView":              view,
			"Vulnerabilities":        vulnerabilities,
			"PendingVulnerabilities": pendingVulnerabilities,
			"CatalogStatus":          string(s.catalog.Status()),
			"CatalogDiagnostics":     s.catalog.Diagnostics(),
			"AssetTXTIP":             "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "ip"),
			"AssetTXTIPPort":         "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "ip_port"),
			"AssetTXTURL":            "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "url"),
			"AssetCSV":               "/reports/" + runID + "/assets.csv?" + copyBase.Encode(),
			"ExportJSON":             "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "json"),
			"ExportHTML":             "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "html"),
			"ExportCSV":              "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "csv"),
		})
	}
}

func (s *server) reportNucleiCommand(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(r.FormValue("tool")) != "nuclei" {
		http.Error(w, "unsupported tool", http.StatusBadRequest)
		return
	}
	findings, err := s.store.ListFindings(runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fingerprints, err := s.store.ListFingerprints(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	findings = filterFindings(findings, fingerprints, reportFiltersFromValues(r.URL.Query()))
	key := strings.TrimSpace(r.FormValue("finding_key"))
	var matches []report.Finding
	for _, finding := range findings {
		if report.FindingKey(finding) == key {
			matches = append(matches, finding)
		}
	}
	if len(matches) != 1 {
		http.Error(w, "finding unavailable or ambiguous", http.StatusBadRequest)
		return
	}
	command, err := report.BuildNucleiCommand(matches[0], s.catalog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(command)
}

func reportFiltersFromValues(values url.Values) reportFilters {
	return reportFilters{
		IP:         values.Get("ip"),
		Port:       values.Get("port"),
		Service:    values.Get("service"),
		Keyword:    values.Get("q"),
		Severity:   values.Get("severity"),
		Severities: parseSeverityFilters(values),
		Source:     values.Get("source"),
	}
}
