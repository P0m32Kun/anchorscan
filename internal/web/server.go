package web

import (
	"bytes"
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
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
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

type toolPageData struct {
	Projects []store.Project
	Tools    []manualTool
	Tool     manualTool
}

type manualTool struct {
	Name    string
	Title   string
	Summary string
	Help    []string
	Presets []toolPreset
}

type toolPreset struct {
	Label   string
	Hint    string
	RawArgs string
}

const reportPageSize = 50

func managedDataRoot(dbPath string) string {
	return filepath.Dir(dbPath)
}

func managedReportPath(dbPath string, projectID string, runID string) string {
	root := managedDataRoot(dbPath)
	if strings.TrimSpace(projectID) == "" {
		return filepath.Join(root, "runs", runID, "report.json")
	}
	return filepath.Join(root, "projects", projectID, "runs", runID, "report.json")
}

func managedArtifactRoot(dbPath string) string {
	return filepath.Join(managedDataRoot(dbPath), "artifacts")
}

func managedProjectDir(dbPath string, projectID string) string {
	return filepath.Join(managedDataRoot(dbPath), "projects", projectID)
}

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
	mux.HandleFunc("/tools/new", s.toolNew)
	mux.HandleFunc("/tools/", s.toolPage)
	mux.HandleFunc("/tools", s.toolCreate)
	mux.HandleFunc("/runs", s.runs)
	mux.HandleFunc("/runs/", s.runDetail)
	mux.HandleFunc("/api/runs/", s.runAPI)
	mux.HandleFunc("/reports/", s.reportDetail)
	mux.HandleFunc("/config", s.configPage)
	mux.HandleFunc("/import/nmap", s.importNmapForm)
	mux.HandleFunc("/import/nmap/run", s.importNmapRun)
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
			hasRunning, err := s.store.ProjectHasRunningRuns(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if hasRunning {
				http.Error(w, "project has running scans", http.StatusConflict)
				return
			}
			artifactDirs, err := s.store.ListProjectArtifactDirs(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.store.DeleteProjectCascade(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, dir := range artifactDirs {
				if strings.TrimSpace(dir) == "" {
					continue
				}
				if err := os.RemoveAll(dir); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if err := os.RemoveAll(managedProjectDir(s.opts.DBPath, id)); err != nil {
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

// renderScanForm renders the new-scan page with the project list, the managed
// artifact root, and an optional preflight result used to surface validation
// errors when a scan submission is rejected.
func (s *server) renderScanForm(w http.ResponseWriter, preflightResult preflight.Result) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/scan_new.html", map[string]any{
		"Projects":     projects,
		"ArtifactRoot": managedArtifactRoot(s.opts.DBPath),
		"Preflight":    preflightResult,
	})
}

func (s *server) toolNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/tool_new.html", toolPageData{Projects: projects, Tools: manualTools()})
}

func (s *server) toolPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	toolName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/tools/"), "/")
	tool, ok := manualToolByName(toolName)
	if !ok {
		http.NotFound(w, r)
		return
	}
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/tool_page.html", toolPageData{Projects: projects, Tool: tool})
}

func (s *server) toolCreate(w http.ResponseWriter, r *http.Request) {
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

	toolName := strings.TrimSpace(r.FormValue("tool"))
	if !isManualTool(toolName) {
		http.Error(w, "unknown tool", http.StatusBadRequest)
		return
	}
	_, useNativeArgs := r.Form["raw_args"]
	var nativeArgs []string
	if rawArgs := strings.TrimSpace(r.FormValue("raw_args")); rawArgs != "" {
		nativeArgs, err = config.SplitArgs(rawArgs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	mode := coalesce(strings.TrimSpace(r.FormValue("mode")), "service")
	portsValue := strings.TrimSpace(r.FormValue("ports"))
	projectID := strings.TrimSpace(r.FormValue("project_id"))
	targetValue := strings.TrimSpace(r.FormValue("target"))
	urlValue := strings.TrimSpace(r.FormValue("url"))
	tagsValue := r.FormValue("tags")
	templateValue := strings.TrimSpace(r.FormValue("template"))
	extraArgsText := r.FormValue("extra_args")
	if !useNativeArgs && (toolName == "rustscan" || (toolName == "nmap" && mode != "alive")) {
		if portsValue == "" {
			portsValue = cfg.Scan.Ports
		}
		portsValue, err = ports.ResolveForConfig(portsValue, s.opts.ConfigPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	extraArgs, err := config.SplitArgs(extraArgsText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	runID := newID("tool-"+toolName, s.opts.Now())
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	opts := app.ToolRunOptions{
		RunID:         runID,
		ProjectID:     projectID,
		Tool:          toolName,
		Mode:          mode,
		Target:        targetValue,
		Ports:         portsValue,
		UseNativeArgs: useNativeArgs,
		NativeArgs:    nativeArgs,
		URL:           urlValue,
		Tags:          splitCSV(tagsValue),
		Template:      templateValue,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		JSONReportPath: jsonPath,
	}
	applyToolExtraArgs(&opts, toolName, extraArgs)
	if _, err := s.manager.StartTool(context.Background(), opts); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": runID})
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
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
	resolvedPorts, err := ports.ResolveForConfig(portValue, s.opts.ConfigPath)
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
	nseRules, err := config.LoadNSERulesForConfig(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tagRules, err := config.LoadTagRulesForConfig(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	runID := newID("run", s.opts.Now())
	projectID := r.FormValue("project_id")
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)
	artifactRoot := strings.TrimSpace(r.FormValue("artifact_root"))
	if artifactRoot == "" {
		artifactRoot = managedArtifactRoot(s.opts.DBPath)
	}
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
		ProjectID: projectID,
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
		ArtifactRoot:   artifactRoot,
		NSERules:       nseRules,
		TagRules:       tagRules,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
}

func (s *server) importNmapForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.renderImportForm(w, "")
}

// renderImportForm renders the Nmap XML import page with the project list and
// an optional error message shown in a top banner.
func (s *server) renderImportForm(w http.ResponseWriter, errMsg string) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/import_nmap.html", map[string]any{
		"Projects": projects,
		"Error":    errMsg,
	})
}

// importNmapRun handles the POST submission of the Nmap XML import form. A
// valid upload is imported as a completed run and the client is redirected to
// its detail page; on any failure the form is re-rendered with an error banner
// (no run is persisted).
func (s *server) importNmapRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		s.renderImportForm(w, "文件过大或格式错误")
		return
	}

	file, _, err := r.FormFile("xml_file")
	if err != nil {
		s.renderImportForm(w, "请选择要导入的 Nmap XML 文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		s.renderImportForm(w, err.Error())
		return
	}
	if len(bytes.TrimSpace(data)) == 0 {
		s.renderImportForm(w, "XML 文件为空")
		return
	}

	projectID := r.FormValue("project_id")
	runID := newID("run", s.opts.Now())
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)

	if _, err := app.ImportNmap(context.Background(), s.store, app.ImportNmapOptions{
		XMLData:   data,
		RunID:     runID,
		ProjectID: projectID,
		JSONPath:  jsonPath,
	}); err != nil {
		s.renderImportForm(w, err.Error())
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
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
	if r.Method == http.MethodPost && r.FormValue("_method") == "delete" {
		run, err := s.store.GetScanRun(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if run.Status == "running" {
			http.Error(w, "scan is running", http.StatusConflict)
			return
		}
		if err := s.store.DeleteScanRunCascade(path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(run.ArtifactDir) != "" {
			if err := os.RemoveAll(run.ArtifactDir); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if err := os.RemoveAll(filepath.Dir(managedReportPath(s.opts.DBPath, run.ProjectID, run.RunID))); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/runs", http.StatusSeeOther)
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
	render(w, "templates/run.html", map[string]any{"Run": run, "RunMeta": newRunMetaView(run)})
}

func (s *server) runAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(path, "/status") {
		id := strings.TrimSuffix(path, "/status")
		run, err := s.store.GetScanRun(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": run.Status})
		return
	}
	if !strings.HasSuffix(path, "/events") {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimSuffix(path, "/events")
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
	filters := reportFilters{
		IP:         r.URL.Query().Get("ip"),
		Port:       r.URL.Query().Get("port"),
		Service:    r.URL.Query().Get("service"),
		Keyword:    r.URL.Query().Get("q"),
		Severity:   r.URL.Query().Get("severity"),
		Severities: parseSeverityFilters(r.URL.Query()),
		Source:     r.URL.Query().Get("source"),
	}
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
		if view == "" {
			view = "ports"
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
			"Run":            run,
			"RunMeta":        newRunMetaView(run),
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
			"ExportJSON":     "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "json"),
			"ExportHTML":     "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "html"),
			"ExportCSV":      "/reports/" + runID + "/export?" + withQuery(copyBase, "format", "csv"),
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

func isManualTool(toolName string) bool {
	switch toolName {
	case "rustscan", "nmap", "httpx", "nuclei":
		return true
	default:
		return false
	}
}

func manualToolByName(name string) (manualTool, bool) {
	for _, tool := range manualTools() {
		if tool.Name == name {
			return tool, true
		}
	}
	return manualTool{}, false
}

func manualTools() []manualTool {
	return []manualTool{
		{
			Name:    "rustscan",
			Title:   "Rustscan 单工具调用",
			Summary: "快速发现主机开放端口，适合先摸清资产入口。",
			Help: []string{
				"参数框填写 rustscan 原生参数，例如 -a 192.168.1.10 -p 80,443。",
				"这里不会再包装参数；你输入什么就拼到 rustscan 后面执行。",
				"需要调速、批量或脚本参数时，直接按 rustscan 原生命令写。",
			},
			Presets: []toolPreset{
				{Label: "快速 Web", Hint: "常见 Web 端口", RawArgs: "-a 192.168.1.10 -p 80,443,8080,8443"},
				{Label: "常见内网", Hint: "管理与中间件端口", RawArgs: "-a 192.168.1.10 -p 22,80,443,445,3389,6379,8080"},
				{Label: "全端口慢扫", Hint: "覆盖完整端口", RawArgs: "-a 192.168.1.10 -r 1-65535"},
			},
		},
		{
			Name:    "nmap",
			Title:   "Nmap 单工具调用",
			Summary: "做主机存活验证或已知端口的服务指纹识别。",
			Help: []string{
				"参数框填写 nmap 原生参数，例如 -sn 192.168.1.10。",
				"存活检测常用 -sn；服务识别常用 -sV 加目标和端口。",
				"限速、重试、时序参数也直接写，例如 -T2 --max-retries 2。",
			},
			Presets: []toolPreset{
				{Label: "存活检测", Hint: "只验证主机在线", RawArgs: "-sn 192.168.1.10"},
				{Label: "服务识别", Hint: "识别常见服务", RawArgs: "-sV -p 22,80,443,3389 192.168.1.10"},
				{Label: "轻一点", Hint: "降低重试", RawArgs: "-sV -T2 --max-retries 2 -p 80,443 192.168.1.10"},
			},
		},
		{
			Name:    "httpx",
			Title:   "Httpx 单工具调用",
			Summary: "识别单个 Web URL 的状态码、标题和技术栈。",
			Help: []string{
				"参数框填写 httpx 原生参数，例如 -u http://192.168.1.10:8080。",
				"URL 要写完整协议，适合在发现 Web 端口后单独做指纹补充。",
				"限速、线程、标题和技术识别参数直接按 httpx 原生命令写。",
			},
			Presets: []toolPreset{
				{Label: "基础识别", Hint: "默认参数", RawArgs: "-u http://192.168.1.10:8080"},
				{Label: "限速稳定", Hint: "低并发少误伤", RawArgs: "-u http://192.168.1.10:8080 -rate-limit 20 -threads 5"},
				{Label: "显示更多", Hint: "标题/状态码/技术栈", RawArgs: "-u http://192.168.1.10:8080 -tech-detect -title -status-code"},
			},
		},
		{
			Name:    "nuclei",
			Title:   "Nuclei 单工具调用",
			Summary: "对单个 URL 按 tags 或指定 template 做漏洞模板探测。",
			Help: []string{
				"参数框填写 nuclei 原生参数，例如 -u http://192.168.1.10:8080 -tags cve。",
				"按 nuclei 原生命令习惯填写 -tags、-t、-u 等参数。",
				"限速和并发参数直接写，例如 -rate-limit 5 -c 5。",
			},
			Presets: []toolPreset{
				{Label: "CVE 检测", Hint: "常见 CVE 模板", RawArgs: "-u http://192.168.1.10:8080 -tags cve"},
				{Label: "暴露面检测", Hint: "配置暴露类", RawArgs: "-u http://192.168.1.10:8080 -tags exposure"},
				{Label: "稳定限速", Hint: "低速低并发", RawArgs: "-u http://192.168.1.10:8080 -tags cve -rate-limit 5 -c 5"},
			},
		},
	}
}

func applyToolExtraArgs(opts *app.ToolRunOptions, toolName string, args []string) {
	switch toolName {
	case "rustscan":
		opts.ExtraArgs.Rustscan = args
	case "nmap":
		opts.ExtraArgs.Nmap = args
	case "httpx":
		opts.ExtraArgs.Httpx = args
	case "nuclei":
		opts.ExtraArgs.Nuclei = args
	}
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
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
