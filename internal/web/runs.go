package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

type runDetailView struct {
	Run            store.ScanRun
	RunMeta        runMetaView
	CanRerun       bool
	IsToolRun      bool
	ReturnURL      string
	VerificationID string
	EvidenceURL    string
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
	if strings.HasSuffix(path, "/include") {
		id := strings.TrimSuffix(path, "/include")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		run, err := s.store.GetScanRun(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		include := r.FormValue("include") == "1"
		if err := s.store.UpdateScanRunIncludeInReport(id, include); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects/"+run.ProjectID, http.StatusSeeOther)
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
	isToolRun := run.Kind == "tool"
	returnURL := strings.TrimSpace(r.URL.Query().Get("return"))
	verificationID := strings.TrimSpace(r.URL.Query().Get("verification_id"))
	if returnURL == "" || verificationID == "" {
		var snapshot struct {
			ReturnURL      string `json:"return_url"`
			VerificationID string `json:"verification_id"`
		}
		_ = json.Unmarshal([]byte(run.ConfigSnapshot), &snapshot)
		if returnURL == "" {
			returnURL = snapshot.ReturnURL
		}
		if verificationID == "" {
			verificationID = snapshot.VerificationID
		}
	}
	if !isSafeReturnURL(returnURL) {
		returnURL = ""
	}
	var evidenceURL string
	if isToolRun && verificationID != "" && run.ProjectID != "" {
		evidenceURL = "/projects/" + run.ProjectID + "/verifications/" + verificationID + "/evidence"
	}
	render(w, "templates/run.html", runDetailView{
		Run:            run,
		RunMeta:        newRunMetaView(run),
		CanRerun:       (run.Status == "interrupted" || run.Status == "completed_with_errors") && run.ProjectID != "" && isScanProfile(run.Profile),
		IsToolRun:      isToolRun,
		ReturnURL:      returnURL,
		VerificationID: verificationID,
		EvidenceURL:    evidenceURL,
	})
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
		counts, err := s.store.CountDetectionChecksByStatus(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, status := range []string{"running", "completed", "skipped", "failed", "canceled", "interrupted"} {
			if _, ok := counts[status]; !ok {
				counts[status] = 0
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": run.Status, "detection_checks": counts})
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
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	projectNames := make(map[string]string, len(projects))
	for _, project := range projects {
		projectNames[project.ID] = project.Name
	}
	render(w, "templates/runs.html", map[string]any{
		"Runs":         runs,
		"ProjectNames": projectNames,
	})
}
