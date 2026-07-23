package web

import (
	"encoding/json"
	"html/template"
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

// workbenchViewModel is the data passed to the verification workbench template.
// It contains the aggregated project report and existing verifications so the UI
// can render positive candidates, negative candidates and incomplete checks.
type workbenchCandidate struct {
	report.ProjectVulnerabilityCandidate
	ZoneID string
}

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

type incompleteWorkbenchItem struct {
	report.ProjectIncompleteCheck
	ZoneID string `json:"zone_id"`
}

type workbenchViewModel struct {
	Project              store.Project
	Zones                []store.ProjectZone
	ZoneNames            map[string]string
	Report               report.ProjectReport
	Verifications        []store.Verification
	VerificationMap      map[string]store.Verification
	NegativeGroups       []negativeFingerprintGroup
	CandidatesJSON       template.JS
	NegativeGroupsJSON   template.JS
	IncompleteChecksJSON template.JS
	PositiveCount        int
	NegativeCount        int
	IncompleteCount      int
	CatalogStatus        string
	CatalogDiagnostics   []string
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tagRules, err := config.LoadTagRulesForConfig(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	negativeGroups := groupNegativeCandidates(negatives, nseRules, tagRules)

	candidatesJSON, err := json.Marshal(candidates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	negativeGroupsJSON, err := json.Marshal(negativeGroups)
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
		Project:              project,
		Zones:                zones,
		ZoneNames:            zoneNames,
		Report:               projReport,
		Verifications:        verifications,
		VerificationMap:      verificationMap,
		NegativeGroups:       negativeGroups,
		CandidatesJSON:       template.JS(candidatesJSON),
		NegativeGroupsJSON:   template.JS(negativeGroupsJSON),
		IncompleteChecksJSON: template.JS(incompletesJSON),
		PositiveCount:        posCount,
		NegativeCount:        len(negativeGroups),
		IncompleteCount:      incCount,
		CatalogStatus:        string(s.catalog.Status()),
		CatalogDiagnostics:   diagnostics,
	})
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
	for _, zone := range projReport.Zones {
		for i := range zone.PositiveCandidates {
			if zone.PositiveCandidates[i].GroupKey == key {
				c := zone.PositiveCandidates[i]
				cand = &c
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
		q := url.Values{}
		q.Set("raw_args", commands[0].ToolArgs)
		q.Set("return", returnPath)
		resp.ToolLink = "/tools/" + tool + "?" + q.Encode()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
