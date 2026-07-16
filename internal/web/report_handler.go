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
		s.reportCommand(w, r, strings.TrimSuffix(path, "/commands"))
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
			"CommandTools":           s.commandTools(filteredFindings),
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

type commandToolView struct {
	Name  string
	Label string
}

func (s *server) commandTools(findings []report.Finding) map[string][]commandToolView {
	counts := map[string]int{}
	for _, finding := range findings {
		counts[report.FindingKey(finding)]++
	}
	available := map[string][]commandToolView{}
	for _, finding := range findings {
		key := report.FindingKey(finding)
		if counts[key] != 1 {
			continue
		}
		for _, tool := range []commandToolView{{Name: "nuclei", Label: "Nuclei"}, {Name: "nmap", Label: "Nmap NSE"}, {Name: "msf", Label: "MSF"}} {
			var err error
			switch tool.Name {
			case "nuclei":
				_, err = report.BuildNucleiCommand(finding, s.catalog)
			case "nmap":
				_, err = report.BuildNmapCommand(finding, s.catalog)
			case "msf":
				_, err = report.BuildMSFCommand(finding, s.catalog)
			}
			if err == nil {
				available[key] = append(available[key], tool)
			}
		}
	}
	return available
}

func (s *server) reportCommand(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tool := strings.TrimSpace(r.FormValue("tool"))
	if tool != "nuclei" && tool != "nmap" && tool != "msf" {
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
	var command report.NucleiCommand
	switch tool {
	case "nuclei":
		command, err = report.BuildNucleiCommand(matches[0], s.catalog)
	case "nmap":
		command, err = report.BuildNmapCommand(matches[0], s.catalog)
	case "msf":
		command, err = report.BuildMSFCommand(matches[0], s.catalog)
	}
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
