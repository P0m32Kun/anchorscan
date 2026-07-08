package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/target"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type ServerOptions struct {
	ConfigPath string
	DBPath     string
	Listen     string
	Runner     tools.Runner
	Now        func() time.Time
}

type server struct {
	opts    ServerOptions
	store   *store.Store
	manager *app.Manager
	mux     *http.ServeMux
}

type configPageData struct {
	Config    config.Config
	RawConfig string
	Error     string
}

const reportPageSize = 50

func NewServer(opts ServerOptions) (http.Handler, error) {
	if opts.Listen == "" {
		opts.Listen = "127.0.0.1:8088"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Runner == nil {
		opts.Runner = tools.NewExecRunner()
	}
	scanStore, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}

	s := &server{opts: opts, store: scanStore, manager: app.NewManager(opts.Runner, scanStore)}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServerFS(assets))
	mux.HandleFunc("/projects", s.projects)
	mux.HandleFunc("/projects/new", s.projectNew)
	mux.HandleFunc("/projects/", s.projectDetail)
	mux.HandleFunc("/scan/new", s.scanNew)
	mux.HandleFunc("/scan", s.scanCreate)
	mux.HandleFunc("/runs", s.runs)
	mux.HandleFunc("/runs/", s.runDetail)
	mux.HandleFunc("/api/runs/", s.runAPI)
	mux.HandleFunc("/reports/", s.reportDetail)
	mux.HandleFunc("/config", s.configPage)
	mux.HandleFunc("/", s.home)
	s.mux = mux
	return s, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *server) Close() error {
	return s.store.Close()
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	projects, _ := s.store.ListProjects()
	runs, _ := s.store.ListScanRuns(10)
	tmpl, err := parseTemplates("templates/home.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.ExecuteTemplate(w, "base", map[string]any{
		"Projects": projects,
		"Runs":     runs,
	})
}

func (s *server) projects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects, err := s.store.ListProjects()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, "templates/projects.html", map[string]any{"Projects": projects})
	case http.MethodPost:
		if err := parseProjectRequest(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defaultTargets, err := mergedTargetsInput(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		now := s.opts.Now()
		project := store.Project{
			ID:             newID("project", now),
			Name:           r.FormValue("name"),
			Description:    r.FormValue("description"),
			DefaultTargets: defaultTargets,
			DefaultPorts:   r.FormValue("default_ports"),
			ExcludeTargets: r.FormValue("exclude_targets"),
			ExcludePorts:   r.FormValue("exclude_ports"),
			DefaultProfile: r.FormValue("default_profile"),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := s.store.SaveProject(project); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) projectNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	render(w, "templates/project_form.html", map[string]any{
		"Title":   "新建项目",
		"Action":  "/projects",
		"Project": store.Project{DefaultProfile: "normal"},
	})
}

func (s *server) projectDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/projects/")
	id := strings.TrimSuffix(path, "/edit")

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/edit"):
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		render(w, "templates/project_form.html", map[string]any{
			"Title":   "编辑项目",
			"Action":  "/projects/" + id,
			"Project": project,
		})
	case r.Method == http.MethodGet:
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		render(w, "templates/project_form.html", map[string]any{
			"Title":   project.Name,
			"Action":  "/projects/" + id,
			"Project": project,
		})
	case r.Method == http.MethodPost:
		if err := parseProjectRequest(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("_method") == "delete" {
			if err := s.store.DeleteProject(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/projects", http.StatusSeeOther)
			return
		}
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defaultTargets, err := mergedTargetsInput(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		project.Name = r.FormValue("name")
		project.Description = r.FormValue("description")
		project.DefaultTargets = defaultTargets
		project.DefaultPorts = r.FormValue("default_ports")
		project.ExcludeTargets = r.FormValue("exclude_targets")
		project.ExcludePorts = r.FormValue("exclude_ports")
		project.DefaultProfile = r.FormValue("default_profile")
		project.UpdatedAt = s.opts.Now()
		if err := s.store.SaveProject(project); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) scanNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.renderScanForm(w, preflight.Result{})
}

func (s *server) scanCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := config.Load(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	project, err := s.loadProjectForScan(r.FormValue("project_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targetValue := strings.TrimSpace(r.FormValue("target"))
	if targetValue == "" && project != nil {
		targetValue = project.DefaultTargets
	}
	targets, err := target.Parse(targetValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	portValue := r.FormValue("ports")
	if portValue == "" && project != nil {
		portValue = project.DefaultPorts
	}
	if portValue == "" {
		portValue = cfg.Scan.Ports
	}
	if project != nil {
		targets, err = excludeTargets(targets, project.ExcludeTargets)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		portValue, err = excludePorts(portValue, project.ExcludePorts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	resolvedPorts, err := resolvePorts(portValue, s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	effective, err := config.ResolveScan(cfg, config.Overrides{
		ProfileName:  coalesce(strings.TrimSpace(r.FormValue("profile")), defaultProjectProfile(project)),
		RustscanArgs: r.FormValue("rustscan_args"),
		NmapArgs:     r.FormValue("nmap_args"),
		HttpxArgs:    r.FormValue("httpx_args"),
		NucleiArgs:   r.FormValue("nuclei_args"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	nseRules, err := loadRuleFile(s.opts.ConfigPath, "nse.yaml", config.LoadNSERules)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tagRules, err := loadRuleFile(s.opts.ConfigPath, "service-tags.yaml", config.LoadTagRules)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	runID := newID("run", s.opts.Now())
	jsonPath := filepath.Join("reports", "scan-"+runID+".json")
	preflightResult := preflight.Run(preflight.Options{
		ConfigDir: filepath.Dir(s.opts.ConfigPath),
		DBPath:    s.opts.DBPath,
		JSONPath:  jsonPath,
		ReportDir: filepath.Dir(jsonPath),
		Targets:   targets,
		PortSpec:  portValue,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		Profile: effective.ProfileName,
		Workers: effective.HostWorkers,
		ExtraArgs: app.ToolExtraArgs{
			Rustscan: effective.Rustscan,
			Nmap:     effective.Nmap,
			Httpx:    effective.Httpx,
			Nuclei:   effective.Nuclei,
		},
		NSERuleCount: len(nseRules),
		TagRuleCount: len(tagRules),
	})
	if preflightResult.HasErrors() {
		w.WriteHeader(http.StatusBadRequest)
		s.renderScanForm(w, preflightResult)
		return
	}
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = s.manager.Start(context.Background(), app.ScanOptions{
		RunID:     runID,
		ProjectID: r.FormValue("project_id"),
		Targets:   targets,
		Ports:     resolvedPorts,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		ProfileName: effective.ProfileName,
		HostWorkers: effective.HostWorkers,
		ExtraArgs: app.ToolExtraArgs{
			Rustscan: effective.Rustscan,
			Nmap:     effective.Nmap,
			Httpx:    effective.Httpx,
			Nuclei:   effective.Nuclei,
		},
		JSONReportPath: jsonPath,
		NSERules:       nseRules,
		TagRules:       tagRules,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
}

func (s *server) renderScanForm(w http.ResponseWriter, preflightResult preflight.Result) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/scan_new.html", map[string]any{
		"Projects":  projects,
		"Preflight": preflightResult,
	})
}

func (s *server) runDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/runs/")
	if strings.HasSuffix(path, "/cancel") {
		id := strings.TrimSuffix(path, "/cancel")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.manager.Cancel(id); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_ = s.store.AppendScanEvent(store.ScanEvent{RunID: id, Time: s.opts.Now(), Level: "info", Stage: "cancel", Message: "cancel requested"})
		http.Redirect(w, r, "/runs/"+id, http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	run, err := s.store.GetScanRun(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, "templates/run.html", map[string]any{"Run": run})
}

func (s *server) runAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	id := strings.TrimSuffix(path, "/events")
	if r.Method != http.MethodGet || !strings.HasSuffix(path, "/events") {
		http.NotFound(w, r)
		return
	}
	events, err := s.store.ListScanEvents(id, 1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *server) runs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runs, err := s.store.ListScanRuns(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/runs.html", map[string]any{"Runs": runs})
}

func (s *server) reportDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/reports/")
	format := ""
	assetExport := ""
	runID := path
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
	built := report.Build(fps, findings)
	filters := reportFilters{
		IP:       r.URL.Query().Get("ip"),
		Port:     r.URL.Query().Get("port"),
		Service:  r.URL.Query().Get("service"),
		Keyword:  r.URL.Query().Get("q"),
		Severity: r.URL.Query().Get("severity"),
		Source:   r.URL.Query().Get("source"),
	}
	filteredFingerprints := filterFingerprints(fps, filters)
	filteredFindings := filterFindings(findings, fps, filters)
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(built)
	case "html":
		tmp := filepath.Join(os.TempDir(), "anchorscan-report-"+runID+".html")
		if err := report.WriteHTML(tmp, built); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmp)
		http.ServeFile(w, r, tmp)
	default:
		if assetExport == "txt" {
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
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			_, _ = io.WriteString(w, data)
			return
		}
		query := r.URL.Query()
		view := query.Get("view")
		if view == "" {
			view = "ports"
		}
		assetPage := paginateFingerprints(filteredFingerprints, parsePage(query.Get("assets_page")), query, "assets_page", reportPageSize)
		findingPage := paginateFindings(filteredFindings, parsePage(query.Get("findings_page")), query, "findings_page", reportPageSize)
		hostPage := paginateHostAssets(groupFingerprintsByHost(filteredFingerprints), parsePage(query.Get("assets_page")), query, "assets_page", reportPageSize)
		copyBase := cloneValues(query)
		copyBase.Del("assets_page")
		copyBase.Del("findings_page")
		render(w, "templates/report.html", map[string]any{
			"Run":            run,
			"Filters":        filters,
			"Fingerprints":   assetPage.Items,
			"Findings":       findingPage.Items,
			"AssetPage":      assetPage,
			"FindingPage":    findingPage,
			"HostPage":       hostPage,
			"AssetView":      view,
			"AssetTXTIP":     "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "ip"),
			"AssetTXTIPPort": "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "ip_port"),
			"AssetTXTURL":    "/reports/" + runID + "/assets.txt?" + withQuery(copyBase, "kind", "url"),
			"AssetCSV":       "/reports/" + runID + "/assets.csv?" + copyBase.Encode(),
		})
	}
}

func (s *server) configPage(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	raw, err := os.ReadFile(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		render(w, "templates/config.html", configPageData{Config: cfg, RawConfig: string(raw)})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("mode") == "raw" {
			rawConfig := r.FormValue("raw_config")
			if _, err := config.SaveRawWithBackup(s.opts.ConfigPath, rawConfig, s.opts.Now()); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				render(w, "templates/config.html", configPageData{
					Config:    cfg,
					RawConfig: rawConfig,
					Error:     "invalid YAML: " + err.Error(),
				})
				return
			}
			http.Redirect(w, r, "/config", http.StatusSeeOther)
			return
		}
		cfg.Tools.Rustscan = r.FormValue("rustscan")
		cfg.Tools.Nmap = r.FormValue("nmap")
		cfg.Tools.Httpx = r.FormValue("httpx")
		cfg.Tools.Nuclei = r.FormValue("nuclei")
		cfg.Scan.Ports = r.FormValue("ports")
		cfg.Scan.Profile = r.FormValue("profile")
		if _, err := config.SaveWithBackup(s.opts.ConfigPath, cfg, s.opts.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/config", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseProjectRequest(r *http.Request) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return r.ParseMultipartForm(8 << 20)
	}
	return r.ParseForm()
}

func mergedTargetsInput(r *http.Request) (string, error) {
	values := []string{strings.TrimSpace(r.FormValue("default_targets"))}
	file, _, err := r.FormFile("targets_file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return joinNonEmpty(values...), nil
		}
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	values = append(values, strings.TrimSpace(string(data)))
	return joinNonEmpty(values...), nil
}

func joinNonEmpty(values ...string) string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, "\n")
}

func (s *server) loadProjectForScan(id string) (*store.Project, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	project, err := s.store.GetProject(id)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func defaultProjectProfile(project *store.Project) string {
	if project == nil {
		return ""
	}
	return strings.TrimSpace(project.DefaultProfile)
}

func excludeTargets(targets []string, excludeValue string) ([]string, error) {
	excluded, err := target.Parse(excludeValue)
	if err != nil {
		return nil, err
	}
	if len(excluded) == 0 {
		return targets, nil
	}
	blocked := map[string]struct{}{}
	for _, item := range excluded {
		blocked[item] = struct{}{}
	}
	var out []string
	for _, item := range targets {
		if _, ok := blocked[item]; ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func excludePorts(portValue string, excludeValue string) (string, error) {
	portValue = strings.TrimSpace(portValue)
	excludeValue = strings.TrimSpace(excludeValue)
	if portValue == "" || excludeValue == "" {
		return portValue, nil
	}
	portsToUse, err := expandPortSpec(portValue)
	if err != nil {
		return "", err
	}
	portsToDrop, err := expandPortSpec(excludeValue)
	if err != nil {
		return "", err
	}
	blocked := map[int]struct{}{}
	for _, port := range portsToDrop {
		blocked[port] = struct{}{}
	}
	var filtered []int
	for _, port := range portsToUse {
		if _, ok := blocked[port]; ok {
			continue
		}
		filtered = append(filtered, port)
	}
	return compressPorts(filtered), nil
}

func expandPortSpec(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var out []int
	seen := map[int]struct{}{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}
			start, err := parsePortNumber(bounds[0])
			if err != nil {
				return nil, err
			}
			end, err := parsePortNumber(bounds[1])
			if err != nil {
				return nil, err
			}
			if end < start {
				start, end = end, start
			}
			for port := start; port <= end; port++ {
				if _, ok := seen[port]; ok {
					continue
				}
				seen[port] = struct{}{}
				out = append(out, port)
			}
			continue
		}
		port, err := parsePortNumber(part)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		out = append(out, port)
	}
	slices.Sort(out)
	return out, nil
}

func parsePortNumber(value string) (int, error) {
	var port int
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &port)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", value)
	}
	return port, nil
}

func compressPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, fmt.Sprintf("%d", port))
	}
	return strings.Join(parts, ",")
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func render(w http.ResponseWriter, file string, data any) {
	tmpl, err := parseTemplates(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.ExecuteTemplate(w, "base", data)
}

func newID(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%s", prefix, now.Format("20060102-150405.000000000"))
}

func resolvePorts(value string, configPath string) (string, error) {
	resolved, err := ports.Resolve(value, filepath.Dir(configPath))
	if err == nil {
		return resolved, nil
	}
	if filepath.Dir(configPath) != "config" {
		return ports.Resolve(value, "config")
	}
	return "", err
}

func loadRuleFile[T any](configPath string, fileName string, loader func(string) (T, error)) (T, error) {
	var zero T
	for _, candidate := range []string{filepath.Join(filepath.Dir(configPath), fileName), filepath.Join("config", fileName)} {
		value, err := loader(candidate)
		if err == nil {
			return value, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return zero, err
	}
	return zero, nil
}
