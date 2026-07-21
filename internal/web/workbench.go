package web

import (
	"encoding/json"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// workbenchViewModel is the data passed to the verification workbench template.
// It contains the aggregated project report and existing verifications so the UI
// can render positive candidates, negative candidates and incomplete checks.
type workbenchCandidate struct {
	report.ProjectVulnerabilityCandidate
	ZoneID string `json:"zone_id"`
}

type negativeWorkbenchItem struct {
	report.ProjectNegativeCandidate
	ZoneID string `json:"zone_id"`
}

type incompleteWorkbenchItem struct {
	report.ProjectIncompleteCheck
	ZoneID string `json:"zone_id"`
}

type workbenchViewModel struct {
	Project                store.Project
	Zones                  []store.ProjectZone
	ZoneNames              map[string]string
	Report                 report.ProjectReport
	Verifications          []store.Verification
	VerificationMap        map[string]store.Verification
	CandidatesJSON         template.JS
	NegativeCandidatesJSON template.JS
	IncompleteChecksJSON   template.JS
	PositiveCount          int
	NegativeCount          int
	IncompleteCount        int
	CatalogStatus          string
	CatalogDiagnostics     []string
}

func (s *server) projectWorkbench(w http.ResponseWriter, r *http.Request, projectID string) {
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	zones, err := s.store.ListProjectZones(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	input, err := s.store.BuildProjectReportInput(projectID, s.catalog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projReport, err := report.BuildProjectReport(input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	verifications, err := s.store.ListProjectVerifications(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	zoneNames := make(map[string]string, len(zones))
	for _, z := range zones {
		zoneNames[z.ZoneID] = z.Name
	}
	verificationMap := make(map[string]store.Verification, len(verifications))
	for _, v := range verifications {
		verificationMap[v.VulnerabilityKey] = v
	}
	var candidates []workbenchCandidate
	var negatives []negativeWorkbenchItem
	var incompletes []incompleteWorkbenchItem
	var posCount, negCount, incCount int
	for _, zone := range projReport.Zones {
		for _, c := range zone.PositiveCandidates {
			candidates = append(candidates, workbenchCandidate{ProjectVulnerabilityCandidate: c, ZoneID: zone.Zone.ZoneID})
			posCount++
		}
		for _, nc := range zone.NegativeCandidates {
			negatives = append(negatives, negativeWorkbenchItem{ProjectNegativeCandidate: nc, ZoneID: zone.Zone.ZoneID})
			negCount++
		}
		for _, ic := range zone.IncompleteChecks {
			incompletes = append(incompletes, incompleteWorkbenchItem{ProjectIncompleteCheck: ic, ZoneID: zone.Zone.ZoneID})
			incCount++
		}
	}
	candidatesJSON, err := json.Marshal(candidates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	negativesJSON, err := json.Marshal(negatives)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	incompletesJSON, err := json.Marshal(incompletes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	diagnostics := make([]string, 0, len(s.catalog.Diagnostics()))
	for _, d := range s.catalog.Diagnostics() {
		diagnostics = append(diagnostics, d.Reason)
	}
	render(w, "templates/workbench.html", workbenchViewModel{
		Project:                project,
		Zones:                  zones,
		ZoneNames:              zoneNames,
		Report:                 projReport,
		Verifications:          verifications,
		VerificationMap:        verificationMap,
		CandidatesJSON:         template.JS(candidatesJSON),
		NegativeCandidatesJSON: template.JS(negativesJSON),
		IncompleteChecksJSON:   template.JS(incompletesJSON),
		PositiveCount:          posCount,
		NegativeCount:          negCount,
		IncompleteCount:        incCount,
		CatalogStatus:          string(s.catalog.Status()),
		CatalogDiagnostics:     diagnostics,
	})
}

type projectCommandResponse struct {
	Commands []report.CandidateCommand `json:"commands"`
	Warning  string                      `json:"warning"`
	ToolLink string                      `json:"tool_link"`
}

func (s *server) projectCandidateCommand(w http.ResponseWriter, r *http.Request, projectID string, key string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input, err := s.store.BuildProjectReportInput(projectID, s.catalog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projReport, err := report.BuildProjectReport(input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var cand *report.ProjectVulnerabilityCandidate
	var zoneID string
	for _, zone := range projReport.Zones {
		for i := range zone.PositiveCandidates {
			if zone.PositiveCandidates[i].GroupKey == key {
				c := zone.PositiveCandidates[i]
				cand = &c
				zoneID = zone.Zone.ZoneID
				break
			}
		}
		if cand != nil {
			break
		}
	}
	if cand == nil {
		http.NotFound(w, r)
		return
	}

	tool := strings.TrimSpace(r.FormValue("tool"))
	assetParam := strings.TrimSpace(r.FormValue("asset"))
	var assets []report.ProjectAsset
	if assetParam == "" || assetParam == "all" {
		assets = cand.Assets
	} else {
		host, portStr, err := net.SplitHostPort(assetParam)
		if err != nil {
			// allow plain ip:port without brackets
			parts := strings.SplitN(assetParam, ":", 2)
			if len(parts) != 2 {
				http.Error(w, "invalid asset format", http.StatusBadRequest)
				return
			}
			host = parts[0]
			portStr = parts[1]
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			http.Error(w, "invalid asset port", http.StatusBadRequest)
			return
		}
		found := false
		for _, a := range cand.Assets {
			if a.IP == host && a.Port == port {
				assets = append(assets, a)
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "asset not found in candidate", http.StatusBadRequest)
			return
		}
	}

	commands, warning, err := report.BuildCandidateCommands(*cand, tool, assets, s.catalog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	resp := projectCommandResponse{Commands: commands, Warning: warning}
	if len(commands) == 1 && commands[0].TargetFile == "" && (tool == "nuclei" || tool == "nmap") {
		verificationID := strings.TrimSpace(r.FormValue("verification_id"))
		returnPath := strings.TrimSpace(r.FormValue("return"))
		if returnPath == "" {
			returnPath = "/projects/" + projectID + "/workbench"
		}
		q := url.Values{}
		q.Set("raw_args", commands[0].ToolArgs)
		q.Set("project_id", projectID)
		q.Set("zone_id", zoneID)
		q.Set("return", returnPath)
		if verificationID != "" {
			q.Set("verification_id", verificationID)
		}
		resp.ToolLink = "/tools/" + tool + "?" + q.Encode()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

