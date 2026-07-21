package web

import (
	"context"
	"encoding/json"
	"errors"
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

// scanForm is the small, explicitly allowed subset of a prior scan that can
// be shown again for user-confirmed reruns.
type scanForm struct {
	Target       string `json:"target"`
	Ports        string `json:"ports"`
	Profile      string `json:"profile"`
	RustscanArgs string `json:"rustscan_args"`
	NmapArgs     string `json:"nmap_args"`
	HttpxArgs    string `json:"httpx_args"`
	NucleiArgs   string `json:"nuclei_args"`
	IsRerun      bool   `json:"-"`
}

// renderProjectScanForm renders the in-project scan form with the project
// context and an optional preflight result used to surface validation errors
// when a scan submission is rejected. Scans are always bound to a project.
func (s *server) renderProjectScanForm(w http.ResponseWriter, project store.Project, preflightResult preflight.Result, form scanForm) {
	highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
	render(w, "templates/scan_project.html", map[string]any{
		"Title":         "发起扫描",
		"Project":       project,
		"ArtifactRoot":  "",
		"Form":          form,
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
	form := scanForm{
		Target:       targetValue,
		Ports:        portValue,
		Profile:      coalesce(strings.TrimSpace(r.FormValue("profile")), defaultProjectProfile(project)),
		RustscanArgs: r.FormValue("rustscan_args"),
		NmapArgs:     r.FormValue("nmap_args"),
		HttpxArgs:    r.FormValue("httpx_args"),
		NucleiArgs:   r.FormValue("nuclei_args"),
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
			ProfileName:  form.Profile,
			RustscanArgs: form.RustscanArgs,
			NmapArgs:     form.NmapArgs,
			HttpxArgs:    form.HttpxArgs,
			NucleiArgs:   form.NucleiArgs,
		},
		DBPath:         s.opts.DBPath,
		JSONReportPath: jsonPath,
		ArtifactRoot:   artifactRoot,
		RunID:          runID,
		ProjectID:      projectID,
	})
	if err != nil {
		field := "target"
		errMsg := err.Error()
		if strings.Contains(strings.ToLower(errMsg), "port") {
			field = "ports"
		}
		preflightResult := preflight.Result{
			Errors: []preflight.Message{
				{Field: field, Message: errMsg},
			},
		}
		w.WriteHeader(http.StatusBadRequest)
		s.renderProjectScanForm(w, *project, preflightResult, form)
		return
	}
	if prepared.Preflight.HasErrors() {
		w.WriteHeader(http.StatusBadRequest)
		s.renderProjectScanForm(w, *project, prepared.Preflight, form)
		return
	}
	form.Target = strings.Join(prepared.Options.Targets, ",")
	form.Ports = prepared.Options.Ports
	form.Profile = prepared.Options.ProfileName
	prepared.Options.ConfigSnapshot = scanFormSnapshot(form)
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

func scanFormSnapshot(form scanForm) string {
	snapshot, _ := json.Marshal(form)
	return string(snapshot)
}

func (s *server) rerunScanForm(projectID, runID string) (scanForm, error) {
	run, err := s.store.GetScanRun(runID)
	if err != nil {
		return scanForm{}, err
	}
	if run.ProjectID != projectID || (run.Status != "interrupted" && run.Status != "completed_with_errors") {
		return scanForm{}, errors.New("rerun is only available for an interrupted or incomplete project scan")
	}
	var form scanForm
	if err := json.Unmarshal([]byte(run.ConfigSnapshot), &form); err != nil || !completeScanForm(form) {
		// Runs created before snapshots existed can still safely reuse their
		// persisted scan columns; advanced arguments were never retained.
		form = scanForm{Target: run.Target, Ports: run.Ports, Profile: run.Profile}
	}
	if !completeScanForm(form) || !isScanProfile(form.Profile) {
		return scanForm{}, errors.New("prior run has an incomplete scan snapshot")
	}
	form.IsRerun = true
	return form, nil
}

func completeScanForm(form scanForm) bool {
	return strings.TrimSpace(form.Target) != "" && strings.TrimSpace(form.Ports) != "" && strings.TrimSpace(form.Profile) != ""
}

func isScanProfile(profile string) bool {
	switch strings.TrimSpace(profile) {
	case "slow", "normal", "fast":
		return true
	default:
		return false
	}
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
