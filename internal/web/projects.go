package web

import (
	"net/http"
	"os"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

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
	highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
	render(w, "templates/project_form.html", map[string]any{
		"Title":         "新建项目",
		"Action":        "/projects",
		"Project":       store.Project{DefaultProfile: "normal"},
		"HighriskPorts": highriskPorts,
	})
}

func (s *server) projectDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/projects/")
	id := strings.TrimSuffix(path, "/edit")

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/scans/new"):
		id := strings.TrimSuffix(path, "/scans/new")
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
		render(w, "templates/scan_project.html", map[string]any{
			"Title":         "发起扫描",
			"Project":       project,
			"ArtifactRoot":  "",
			"Preflight":     preflight.Result{},
			"HighriskPorts": highriskPorts,
		})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/edit"):
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
		render(w, "templates/project_form.html", map[string]any{
			"Title":         "编辑项目",
			"Action":        "/projects/" + id,
			"Project":       project,
			"HighriskPorts": highriskPorts,
		})
	case r.Method == http.MethodGet:
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		runs, err := s.store.ListProjectScanRuns(id, 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
		render(w, "templates/project_detail.html", map[string]any{
			"Title":         project.Name,
			"Project":       project,
			"Runs":          runs,
			"HighriskPorts": highriskPorts,
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

func parseProjectRequest(r *http.Request) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return r.ParseMultipartForm(8 << 20)
	}
	return r.ParseForm()
}
