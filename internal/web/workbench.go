package web

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

// workbenchCandidate wraps a positive candidate with its parent zone.
type workbenchCandidate struct {
	report.ProjectVulnerabilityCandidate
	ZoneID string
}

// negativeFingerprintGroup aggregates assets that share the same service fingerprint
// within one zone for negative verification.
type negativeFingerprintGroup struct {
	Key           string
	Title         string
	Service       string
	Product       string
	ZoneID        string
	Assets        []report.ProjectAsset
	NmapCommand   string
	NucleiCommand string
	PortsText     string
}

// incompleteWorkbenchItem wraps an incomplete check with its parent zone.
type incompleteWorkbenchItem struct {
	report.ProjectIncompleteCheck
	ZoneID string `json:"zone_id"`
}

// workbenchData is the shared aggregation result for both the HTML page and the
// JSON API. It is serialized into props for the Vue workbench component.
type workbenchData struct {
	ProjectID          string
	ProjectName        string
	Zones              []store.ProjectZone
	ZoneNames          map[string]string
	Candidates         []workbenchCandidate
	NegativeGroups     []negativeFingerprintGroup
	IncompleteChecks   []incompleteWorkbenchItem
	Verifications      []store.Verification
	VerificationMap  map[string]store.Verification
	PositiveCount      int
	NegativeCount      int
	IncompleteCount    int
	CatalogStatus      string
	CatalogDiagnostics []string
}

// workbenchPageData is the minimal data the server-rendered workbench template
// needs: project metadata for the header and JSON props for Vue.
type workbenchPageData struct {
	Project   store.Project
	PropsJSON string
}

// workbenchCounts mirrors the queue tab counters.
type workbenchCounts struct {
	Positive   int `json:"positive"`
	Negative   int `json:"negative"`
	Incomplete int `json:"incomplete"`
}

func negativeFingerprintKey(fp report.ProjectNegativeCandidate) string {
	service := strings.TrimSpace(fp.Fingerprint.Service)
	product := strings.TrimSpace(fp.Fingerprint.Product)
	if service == "" {
		service = "unknown"
	}
	parts := []string{strings.TrimSpace(fp.ZoneID), strings.ToLower(service)}
	if product != "" {
		parts = append(parts, strings.ToLower(product))
	}
	return strings.Join(parts, "|")
}

func groupNegativeCandidates(negatives []report.ProjectNegativeCandidate, nseRules map[string][]string, tagRules []vuln.TagRule) []negativeFingerprintGroup {
	groups := map[string]*negativeFingerprintGroup{}
	for _, n := range negatives {
		key := negativeFingerprintKey(n)
		g, ok := groups[key]
		if !ok {
			fp := n.Fingerprint
			title := fp.Service
			if title == "" {
				title = "unknown"
			}
			if fp.Product != "" {
				title += " / " + fp.Product
			}
			g = &negativeFingerprintGroup{
				Key:     key,
				Title:   title,
				Service: fp.Service,
				Product: fp.Product,
				ZoneID:  n.ZoneID,
				Assets:  []report.ProjectAsset{},
			}
			groups[key] = g
		}
		duplicate := false
		for _, asset := range g.Assets {
			if asset.IP == n.Asset.IP && asset.Port == n.Asset.Port && asset.Protocol == n.Asset.Protocol && asset.Target == n.Asset.Target {
				duplicate = true
				break
			}
		}
		if !duplicate {
			g.Assets = append(g.Assets, n.Asset)
		}
	}
	result := make([]negativeFingerprintGroup, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g.Assets, func(i, j int) bool {
			if g.Assets[i].IP != g.Assets[j].IP {
				return g.Assets[i].IP < g.Assets[j].IP
			}
			return g.Assets[i].Port < g.Assets[j].Port
		})

		ips := make([]string, 0, len(g.Assets))
		portNumbers := make([]int, 0, len(g.Assets))
		urls := make([]string, 0, len(g.Assets))
		seenIPs := map[string]bool{}
		seenPorts := map[int]bool{}
		seenURLs := map[string]bool{}
		for _, a := range g.Assets {
			if !seenIPs[a.IP] {
				seenIPs[a.IP] = true
				ips = append(ips, a.IP)
			}
			if a.Port != 0 && !seenPorts[a.Port] {
				seenPorts[a.Port] = true
				portNumbers = append(portNumbers, a.Port)
			}
			endpoint := a.Target
			if endpoint == "" && (a.Protocol == "http" || a.Protocol == "https") {
				endpoint = a.Protocol + "://" + net.JoinHostPort(a.IP, strconv.Itoa(a.Port))
			}
			if endpoint != "" && !seenURLs[endpoint] {
				seenURLs[endpoint] = true
				urls = append(urls, endpoint)
			}
		}
		sort.Ints(portNumbers)
		ports := make([]string, 0, len(portNumbers))
		for _, port := range portNumbers {
			ports = append(ports, strconv.Itoa(port))
		}
		g.PortsText = strings.Join(ports, "、")

		normalized := fingerprint.Classify(fingerprint.ServiceFingerprint{Service: g.Service, Product: g.Product}).Normalized
		if scripts, ok := nseRules[normalized]; ok && len(scripts) > 0 {
			if len(ports) > 0 {
				g.NmapCommand = "nmap -p " + strings.Join(ports, ",") + " --script " + strings.Join(scripts, ",") + " " + strings.Join(ips, " ")
			} else {
				g.NmapCommand = "nmap --script " + strings.Join(scripts, ",") + " " + strings.Join(ips, " ")
			}
		}

		first := g.Assets[0]
		match := vuln.MatchNucleiTags(
			fingerprintFromAsset(first, g),
			vuln.HTTPResult{URL: first.Target},
			tagRules,
		)
		if len(match.Tags) > 0 {
			tags := strings.Join(match.Tags, ",")
			var parts []string
			parts = append(parts, "nuclei", "-tags", tags)
			if len(match.ExcludeTags) > 0 {
				parts = append(parts, "-exclude-tags", strings.Join(match.ExcludeTags, ","))
			}
			if match.Target == "url" && len(urls) > 0 {
				parts = append(parts, "-u", strings.Join(urls, ","))
			} else {
				parts = append(parts, "-target", strings.Join(ips, ","))
			}
			g.NucleiCommand = strings.Join(parts, " ")
		}

		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Service != result[j].Service {
			return result[i].Service < result[j].Service
		}
		if result[i].Product != result[j].Product {
			return result[i].Product < result[j].Product
		}
		return result[i].Key < result[j].Key
	})
	return result
}

func fingerprintFromAsset(asset report.ProjectAsset, g *negativeFingerprintGroup) fingerprint.ServiceFingerprint {
	return fingerprint.Classify(fingerprint.ServiceFingerprint{
		IP:       asset.IP,
		Port:     asset.Port,
		Protocol: asset.Protocol,
		Service:  g.Service,
		Product:  g.Product,
		IsWeb:    asset.Protocol == "http" || asset.Protocol == "https" || asset.Target != "",
		URL:      asset.Target,
	})
}

func (s *server) buildWorkbenchData(projectID string) (workbenchData, error) {
	var data workbenchData
	zones, err := s.store.ListProjectZones(projectID)
	if err != nil {
		return data, err
	}
	input, err := s.store.BuildProjectReportInput(projectID, s.catalog)
	if err != nil {
		return data, err
	}
	projReport, err := report.BuildProjectReport(input)
	if err != nil {
		return data, err
	}
	verifications, err := s.store.ListProjectVerifications(projectID)
	if err != nil {
		return data, err
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
	var negatives []report.ProjectNegativeCandidate
	var incompletes []incompleteWorkbenchItem
	var posCount, negCount, incCount int
	for _, zone := range projReport.Zones {
		for _, c := range zone.PositiveCandidates {
			candidates = append(candidates, workbenchCandidate{ProjectVulnerabilityCandidate: c, ZoneID: zone.Zone.ZoneID})
			posCount++
		}
		for _, nc := range zone.NegativeCandidates {
			negatives = append(negatives, nc)
			negCount++
		}
		for _, ic := range zone.IncompleteChecks {
			incompletes = append(incompletes, incompleteWorkbenchItem{ProjectIncompleteCheck: ic, ZoneID: zone.Zone.ZoneID})
			incCount++
		}
	}
	nseRules, err := config.LoadNSERulesForConfig(s.opts.ConfigPath)
	if err != nil {
		return data, err
	}
	tagRules, err := config.LoadTagRulesForConfig(s.opts.ConfigPath)
	if err != nil {
		return data, err
	}
	negativeGroups := groupNegativeCandidates(negatives, nseRules, tagRules)

	diagnostics := make([]string, 0, len(s.catalog.Diagnostics()))
	for _, d := range s.catalog.Diagnostics() {
		diagnostics = append(diagnostics, d.Reason)
	}

	return workbenchData{
		Zones:              zones,
		ZoneNames:          zoneNames,
		Candidates:         candidates,
		NegativeGroups:     negativeGroups,
		IncompleteChecks:   incompletes,
		Verifications:      verifications,
		VerificationMap:    verificationMap,
		PositiveCount:      posCount,
		NegativeCount:      len(negativeGroups),
		IncompleteCount:    incCount,
		CatalogStatus:      string(s.catalog.Status()),
		CatalogDiagnostics: diagnostics,
	}, nil
}

func (s *server) projectWorkbench(w http.ResponseWriter, r *http.Request, projectID string) {
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.buildWorkbenchData(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.ProjectID = project.ID
	data.ProjectName = project.Name
	props := struct {
		ProjectID          string                     `json:"project_id"`
		ProjectName        string                     `json:"project_name"`
		Zones              []store.ProjectZone        `json:"zones"`
		ZoneNames          map[string]string          `json:"zone_names"`
		Candidates         []workbenchCandidate       `json:"candidates"`
		NegativeGroups     []negativeFingerprintGroup `json:"negative_groups"`
		IncompleteChecks   []incompleteWorkbenchItem  `json:"incomplete_checks"`
		Verifications      []store.Verification       `json:"verifications"`
		Counts             workbenchCounts            `json:"counts"`
		CatalogStatus      string                     `json:"catalog_status"`
		CatalogDiagnostics []string                   `json:"catalog_diagnostics"`
	}{
		ProjectID:          data.ProjectID,
		ProjectName:        data.ProjectName,
		Zones:              data.Zones,
		ZoneNames:          data.ZoneNames,
		Candidates:         data.Candidates,
		NegativeGroups:     data.NegativeGroups,
		IncompleteChecks:   data.IncompleteChecks,
		Verifications:      data.Verifications,
		Counts:             workbenchCounts{Positive: data.PositiveCount, Negative: data.NegativeCount, Incomplete: data.IncompleteCount},
		CatalogStatus:      data.CatalogStatus,
		CatalogDiagnostics: data.CatalogDiagnostics,
	}
	propsJSON, err := json.Marshal(props)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/workbench.html", workbenchPageData{
		Project:   project,
		PropsJSON: string(propsJSON),
	})
}

func (s *server) projectWorkbenchAPI(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.buildWorkbenchData(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.ProjectID = project.ID
	data.ProjectName = project.Name
	resp := struct {
		ProjectID          string                     `json:"project_id"`
		ProjectName        string                     `json:"project_name"`
		Zones              []store.ProjectZone        `json:"zones"`
		ZoneNames          map[string]string          `json:"zone_names"`
		Candidates         []workbenchCandidate       `json:"candidates"`
		NegativeGroups     []negativeFingerprintGroup `json:"negative_groups"`
		IncompleteChecks   []incompleteWorkbenchItem  `json:"incomplete_checks"`
		Verifications      []store.Verification       `json:"verifications"`
		Counts             workbenchCounts            `json:"counts"`
		CatalogStatus      string                     `json:"catalog_status"`
		CatalogDiagnostics []string                   `json:"catalog_diagnostics"`
	}{
		ProjectID:          data.ProjectID,
		ProjectName:        data.ProjectName,
		Zones:              data.Zones,
		ZoneNames:          data.ZoneNames,
		Candidates:         data.Candidates,
		NegativeGroups:     data.NegativeGroups,
		IncompleteChecks:   data.IncompleteChecks,
		Verifications:      data.Verifications,
		Counts:             workbenchCounts{Positive: data.PositiveCount, Negative: data.NegativeCount, Incomplete: data.IncompleteCount},
		CatalogStatus:      data.CatalogStatus,
		CatalogDiagnostics: data.CatalogDiagnostics,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type projectCommandResponse struct {
	Commands []report.CandidateCommand `json:"commands"`
	Warning  string                    `json:"warning"`
	ToolLink string                    `json:"tool_link"`
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
	var candZoneID string
	for _, zone := range projReport.Zones {
		for i := range zone.PositiveCandidates {
			if zone.PositiveCandidates[i].GroupKey == key {
				c := zone.PositiveCandidates[i]
				cand = &c
				candZoneID = zone.Zone.ZoneID
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
		returnPath := strings.TrimSpace(r.FormValue("return"))
		if returnPath == "" {
			returnPath = "/projects/" + projectID + "/workbench"
		}
		verificationID := strings.TrimSpace(r.FormValue("verification_id"))
		q := url.Values{}
		q.Set("project_id", projectID)
		q.Set("zone_id", candZoneID)
		if verificationID != "" {
			q.Set("verification_id", verificationID)
		}
		q.Set("raw_args", commands[0].ToolArgs)
		q.Set("return", returnPath)
		resp.ToolLink = "/tools/" + tool + "?" + q.Encode()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
