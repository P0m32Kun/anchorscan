package web

import (
	"net/http"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
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
	opts  ServerOptions
	store *store.Store
}

func NewServer(opts ServerOptions) (http.Handler, error) {
	if opts.Listen == "" {
		opts.Listen = "127.0.0.1:8088"
	}
	scanStore, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}

	s := &server{opts: opts, store: scanStore}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServerFS(assets))
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
