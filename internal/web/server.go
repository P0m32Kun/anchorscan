package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type ServerOptions struct {
	ConfigPath        string
	DBPath            string
	Listen            string
	Runner            tools.Runner
	Now               func() time.Time
	DocxTemplatePath  string
	DocxRenderProject string
}

type server struct {
	opts    ServerOptions
	store   *store.Store
	manager *app.Manager
	catalog *knowledgebase.Catalog
	mux     *http.ServeMux
}

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

func managedProjectDir(dbPath string, projectID string) string {
	return filepath.Join(managedDataRoot(dbPath), "projects", projectID)
}

func managedArtifactParent(jsonPath string) string {
	return filepath.Dir(filepath.Dir(jsonPath))
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
	if err := scanStore.ReconcileInterruptedRuns(opts.Now(), 30*time.Second); err != nil {
		_ = scanStore.Close()
		return nil, err
	}

	catalog := knowledgebase.Load(opts.ConfigPath, "")
	if cfg, err := config.Load(opts.ConfigPath); err == nil {
		catalog = knowledgebase.Load(opts.ConfigPath, cfg.KnowledgeBase.Path)
	}
	s := &server{opts: opts, store: scanStore, manager: app.NewManager(opts.Runner, scanStore), catalog: catalog}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServerFS(assets))
	mux.HandleFunc("/projects", s.projects)
	mux.HandleFunc("/api/projects/", s.projectAPI)
	mux.HandleFunc("/projects/new", s.projectNew)
	mux.HandleFunc("/projects/", s.projectDetail)
	mux.HandleFunc("/scan", s.scanCreate)
	mux.HandleFunc("/tools/new", s.toolNew)
	mux.HandleFunc("/tools/", s.toolPage)
	mux.HandleFunc("/tools", s.toolCreate)
	mux.HandleFunc("/runs", s.runs)
	mux.HandleFunc("/runs/", s.runDetail)
	mux.HandleFunc("/api/runs/", s.runAPI)
	mux.HandleFunc("/reports/", s.reportDetail)
	mux.HandleFunc("/config", s.configPage)
	mux.HandleFunc("/config/ports", s.configPorts)
	mux.HandleFunc("/kb/", s.knowledgeBaseDetail)
	mux.HandleFunc("/kb", s.knowledgeBaseList)
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
