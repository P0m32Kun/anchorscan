package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// renderProjectScanForm renders the in-project scan form with the project
// context and an optional preflight result used to surface validation errors
// when a scan submission is rejected. Scans are always bound to a project.
func (s *server) renderProjectScanForm(w http.ResponseWriter, project store.Project, preflightResult preflight.Result) {
	highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
	render(w, "templates/scan_project.html", map[string]any{
		"Title":         "发起扫描",
		"Project":       project,
		"ArtifactRoot":  "",
		"Preflight":     preflightResult,
		"HighriskPorts": highriskPorts,
	})
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
	if _, err := config.Load(s.opts.ConfigPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	project, err := s.loadProjectForScan(r.FormValue("project_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Web scans must belong to a project; CLI scans are unaffected.
	if project == nil {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}
	targetValue := strings.TrimSpace(r.FormValue("target"))
	if targetValue == "" {
		targetValue = project.DefaultTargets
	}
	portValue := r.FormValue("ports")
	if portValue == "" {
		portValue = project.DefaultPorts
	}

	runID := newID("run", s.opts.Now())
	projectID := project.ID
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)
	artifactRoot := strings.TrimSpace(r.FormValue("artifact_root"))
	if artifactRoot == "" {
		artifactRoot = managedArtifactParent(jsonPath)
	}
	prepared, err := app.PrepareScan(app.PrepareScanRequest{
		ConfigPath:     s.opts.ConfigPath,
		TargetSpec:     targetValue,
		PortSpec:       portValue,
		ExcludeTargets: project.ExcludeTargets,
		ExcludePorts:   project.ExcludePorts,
		Overrides: config.Overrides{
			ProfileName:  coalesce(strings.TrimSpace(r.FormValue("profile")), defaultProjectProfile(project)),
			RustscanArgs: r.FormValue("rustscan_args"),
			NmapArgs:     r.FormValue("nmap_args"),
			HttpxArgs:    r.FormValue("httpx_args"),
			NucleiArgs:   r.FormValue("nuclei_args"),
		},
		DBPath:         s.opts.DBPath,
		JSONReportPath: jsonPath,
		ArtifactRoot:   artifactRoot,
		RunID:          runID,
		ProjectID:      projectID,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if prepared.Preflight.HasErrors() {
		w.WriteHeader(http.StatusBadRequest)
		s.renderProjectScanForm(w, *project, prepared.Preflight)
		return
	}
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = s.manager.Start(context.Background(), prepared.Options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
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

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
