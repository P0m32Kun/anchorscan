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
	ZoneID         string `json:"zone_id"`
	Target         string `json:"target"`
	ExcludeTargets string `json:"exclude_targets"`
	Ports          string `json:"ports"`
	ExcludePorts   string `json:"exclude_ports"`
	Profile        string `json:"profile"`
	Label          string `json:"label"`
	AccessPoint    string `json:"access_point"`
	TesterIP       string `json:"tester_ip"`
	Notes          string `json:"notes"`
	RustscanArgs   string `json:"rustscan_args"`
	NmapArgs       string `json:"nmap_args"`
	HttpxArgs      string `json:"httpx_args"`
	NucleiArgs     string `json:"nuclei_args"`
	IsRerun        bool   `json:"-"`
}

// renderProjectScanForm renders the in-project scan form with the project
// context and an optional preflight result used to surface validation errors
// when a scan submission is rejected. Scans are always bound to a project.
func (s *server) renderProjectScanForm(w http.ResponseWriter, project store.Project, zones []store.ProjectZone, preflightResult preflight.Result, form scanForm) {
	highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
	render(w, "templates/scan_project.html", map[string]any{
		"Title":         "发起扫描",
		"Project":       project,
		"Zones":         zones,
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
	zones, err := s.store.ListProjectZones(project.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	form := scanFormFromRequest(r)
	if form.Profile == "" {
		form.Profile = "normal"
	}

	if err := validateScanForm(form, zones); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.renderProjectScanForm(w, *project, zones, preflight.Result{
			Errors: []preflight.Message{{Field: err.Field, Message: err.Message}},
		}, form)
		return
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
		TargetSpec:     form.Target,
		PortSpec:       form.Ports,
		ExcludeTargets: form.ExcludeTargets,
		ExcludePorts:   form.ExcludePorts,
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
		ZoneID:         form.ZoneID,
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
		s.renderProjectScanForm(w, *project, zones, preflightResult, form)
		return
	}
	if prepared.Preflight.HasErrors() {
		w.WriteHeader(http.StatusBadRequest)
		s.renderProjectScanForm(w, *project, zones, prepared.Preflight, form)
		return
	}
	form.Target = strings.Join(prepared.Options.Targets, ",")
	form.Ports = prepared.Options.Ports
	form.Profile = prepared.Options.ProfileName
	prepared.Options.ConfigSnapshot = scanFormSnapshot(form)
	prepared.Options.Kind = "scan"
	prepared.Options.Label = form.Label
	prepared.Options.AccessPoint = form.AccessPoint
	prepared.Options.TesterIP = form.TesterIP
	prepared.Options.Notes = form.Notes
	prepared.Options.IncludeInReport = false
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

func scanFormFromRequest(r *http.Request) scanForm {
	return scanForm{
		ZoneID:         strings.TrimSpace(r.FormValue("zone_id")),
		Target:         strings.TrimSpace(r.FormValue("target")),
		ExcludeTargets: strings.TrimSpace(r.FormValue("exclude_targets")),
		Ports:          strings.TrimSpace(r.FormValue("ports")),
		ExcludePorts:   strings.TrimSpace(r.FormValue("exclude_ports")),
		Profile:        strings.TrimSpace(r.FormValue("profile")),
		Label:          strings.TrimSpace(r.FormValue("label")),
		AccessPoint:    strings.TrimSpace(r.FormValue("access_point")),
		TesterIP:       strings.TrimSpace(r.FormValue("tester_ip")),
		Notes:          strings.TrimSpace(r.FormValue("notes")),
		RustscanArgs:   r.FormValue("rustscan_args"),
		NmapArgs:       r.FormValue("nmap_args"),
		HttpxArgs:      r.FormValue("httpx_args"),
		NucleiArgs:     r.FormValue("nuclei_args"),
	}
}

type scanFormError struct {
	Field   string
	Message string
}

func validateScanForm(form scanForm, zones []store.ProjectZone) *scanFormError {
	if form.ZoneID == "" {
		return &scanFormError{Field: "zone_id", Message: "请选择网络分区"}
	}
	zoneOK := false
	for _, z := range zones {
		if z.ZoneID == form.ZoneID {
			zoneOK = true
			break
		}
	}
	if !zoneOK {
		return &scanFormError{Field: "zone_id", Message: "所选网络分区不属于该项目"}
	}
	if form.Target == "" {
		return &scanFormError{Field: "target", Message: "目标不能为空"}
	}
	if form.Ports == "" {
		return &scanFormError{Field: "ports", Message: "端口不能为空"}
	}
	if form.AccessPoint == "" {
		return &scanFormError{Field: "access_point", Message: "接入点不能为空"}
	}
	if form.TesterIP == "" {
		return &scanFormError{Field: "tester_ip", Message: "测试机 IP 不能为空"}
	}
	if !isScanProfile(form.Profile) {
		return &scanFormError{Field: "profile", Message: "请选择有效的扫描档位"}
	}
	return nil
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
	if form.ZoneID == "" {
		form.ZoneID = run.ZoneID
	}
	if form.Label == "" && run.Label != "" {
		form.Label = run.Label
	}
	if form.AccessPoint == "" && run.AccessPoint != "" {
		form.AccessPoint = run.AccessPoint
	}
	if form.TesterIP == "" && run.TesterIP != "" {
		form.TesterIP = run.TesterIP
	}
	if form.Notes == "" && run.Notes != "" {
		form.Notes = run.Notes
	}
	if !completeScanForm(form) || !isScanProfile(form.Profile) {
		return scanForm{}, errors.New("prior run has an incomplete scan snapshot")
	}
	form.IsRerun = true
	return form, nil
}

func completeScanForm(form scanForm) bool {
	return strings.TrimSpace(form.ZoneID) != "" &&
		strings.TrimSpace(form.Target) != "" &&
		strings.TrimSpace(form.Ports) != "" &&
		strings.TrimSpace(form.Profile) != ""
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
