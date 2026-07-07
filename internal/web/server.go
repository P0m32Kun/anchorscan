package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
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
	mux.HandleFunc("/runs", s.runs)
	mux.HandleFunc("/runs/", s.runDetail)
	mux.HandleFunc("/api/runs/", s.runAPI)
	mux.HandleFunc("/reports/", s.reportDetail)
	mux.HandleFunc("/config", s.configPage)
	mux.HandleFunc("/", s.home)
	return mux, nil
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
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		now := s.opts.Now()
		project := store.Project{
			ID:             newID("project", now),
			Name:           r.FormValue("name"),
			Description:    r.FormValue("description"),
			DefaultTargets: r.FormValue("default_targets"),
			DefaultPorts:   r.FormValue("default_ports"),
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
		"Title":   "New Project",
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
			"Title":   "Edit Project",
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
		if err := r.ParseForm(); err != nil {
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
		project.Name = r.FormValue("name")
		project.Description = r.FormValue("description")
		project.DefaultTargets = r.FormValue("default_targets")
		project.DefaultPorts = r.FormValue("default_ports")
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
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/scan_new.html", map[string]any{"Projects": projects})
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
	targets, err := target.Parse(r.FormValue("target"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	portValue := r.FormValue("ports")
	if portValue == "" {
		portValue = cfg.Scan.Ports
	}
	resolvedPorts, err := resolvePorts(portValue, s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	effective, err := config.ResolveScan(cfg, config.Overrides{
		ProfileName:  r.FormValue("profile"),
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
	runID := path
	if strings.HasSuffix(path, ".json") {
		format = "json"
		runID = strings.TrimSuffix(path, ".json")
	}
	if strings.HasSuffix(path, ".html") {
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
		filters := reportFilters{
			IP:       r.URL.Query().Get("ip"),
			Port:     r.URL.Query().Get("port"),
			Service:  r.URL.Query().Get("service"),
			Severity: r.URL.Query().Get("severity"),
			Source:   r.URL.Query().Get("source"),
		}
		render(w, "templates/report.html", map[string]any{
			"Run":          run,
			"Filters":      filters,
			"Fingerprints": filterFingerprints(fps, filters),
			"Findings":     filterFindings(findings, filters),
		})
	}
}

func (s *server) configPage(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		render(w, "templates/config.html", cfg)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
