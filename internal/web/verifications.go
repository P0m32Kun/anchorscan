package web

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/google/uuid"
)

const (
	maxEvidenceSize = 10 << 20 // 10 MiB
)

func (s *server) projectVerifications(w http.ResponseWriter, r *http.Request, projectID string, segments []string) {
	if _, err := s.store.GetProject(projectID); err != nil {
		http.NotFound(w, r)
		return
	}

	// /projects/{id}/verifications
	if len(segments) == 2 {
		switch r.Method {
		case http.MethodGet:
			s.listVerifications(w, r, projectID)
		case http.MethodPost:
			s.createVerification(w, r, projectID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	verificationID := segments[2]
	// /projects/{id}/verifications/{vid}
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodGet:
			s.getVerification(w, r, projectID, verificationID)
		case http.MethodPost:
			s.updateVerification(w, r, projectID, verificationID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /projects/{id}/verifications/{vid}/evidence
	if len(segments) == 4 && segments[3] == "evidence" {
		switch r.Method {
		case http.MethodGet:
			s.listEvidence(w, r, projectID, verificationID)
		case http.MethodPost:
			s.uploadEvidence(w, r, projectID, verificationID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /projects/{id}/verifications/{vid}/evidence/{eid}
	if len(segments) == 5 && segments[3] == "evidence" {
		evidenceID := segments[4]
		switch r.Method {
		case http.MethodGet:
			s.serveEvidence(w, r, projectID, verificationID, evidenceID)
		case http.MethodPost:
			s.updateEvidence(w, r, projectID, verificationID, evidenceID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.NotFound(w, r)
}

func (s *server) listVerifications(w http.ResponseWriter, r *http.Request, projectID string) {
	verifications, err := s.store.ListProjectVerifications(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(verifications)
}

type verificationCreateRequest struct {
	ZoneID           string                     `json:"zone_id"`
	VulnerabilityKey string                     `json:"vulnerability_key"`
	Outcome          string                     `json:"outcome"`
	Title            string                     `json:"title"`
	Severity         string                     `json:"severity"`
	Description      string                     `json:"description"`
	Remediation      string                     `json:"remediation"`
	Notes            string                     `json:"notes"`
	Included         bool                       `json:"included"`
	Position         int                        `json:"position"`
	Assets           []store.VerificationAsset  `json:"assets"`
	Sources          []store.VerificationSource `json:"sources"`
}

func (s *server) createVerification(w http.ResponseWriter, r *http.Request, projectID string) {
	var req verificationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Outcome) == "" {
		http.Error(w, "outcome is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if req.Included && (req.Outcome == "confirmed" || req.Outcome == "not_observed") {
		http.Error(w, "cannot include verification without evidence", http.StatusBadRequest)
		return
	}

	now := s.opts.Now()
	v := store.Verification{
		ID:               uuid.New().String(),
		ProjectID:        projectID,
		ZoneID:           req.ZoneID,
		VulnerabilityKey: req.VulnerabilityKey,
		Outcome:          req.Outcome,
		Title:            req.Title,
		Severity:         req.Severity,
		Description:      req.Description,
		Remediation:      req.Remediation,
		Notes:            req.Notes,
		Included:         req.Included,
		Position:         req.Position,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.store.CreateVerification(v, req.Assets, req.Sources); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) getVerification(w http.ResponseWriter, r *http.Request, projectID, verificationID string) {
	v, err := s.store.GetVerification(verificationID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if v.Verification.ProjectID != projectID {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type verificationUpdateRequest struct {
	ZoneID           string `json:"zone_id"`
	VulnerabilityKey string `json:"vulnerability_key"`
	Outcome          string `json:"outcome"`
	Title            string `json:"title"`
	Severity         string `json:"severity"`
	Description      string `json:"description"`
	Remediation      string `json:"remediation"`
	Notes            string `json:"notes"`
	Included         bool   `json:"included"`
	Position         int    `json:"position"`
}

func (s *server) updateVerification(w http.ResponseWriter, r *http.Request, projectID, verificationID string) {
	existing, err := s.store.GetVerification(verificationID)
	if err != nil || existing.Verification.ProjectID != projectID {
		http.NotFound(w, r)
		return
	}

	var req verificationUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Included && (req.Outcome == "confirmed" || req.Outcome == "not_observed") {
		count, err := s.store.CountVerificationEvidence(verificationID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if count == 0 {
			http.Error(w, "cannot include verification without evidence", http.StatusBadRequest)
			return
		}
	}

	v := existing.Verification
	v.ZoneID = req.ZoneID
	v.VulnerabilityKey = req.VulnerabilityKey
	v.Outcome = req.Outcome
	v.Title = req.Title
	v.Severity = req.Severity
	v.Description = req.Description
	v.Remediation = req.Remediation
	v.Notes = req.Notes
	v.Included = req.Included
	v.Position = req.Position
	v.UpdatedAt = s.opts.Now()
	if err := s.store.UpdateVerification(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) listEvidence(w http.ResponseWriter, r *http.Request, projectID, verificationID string) {
	v, err := s.store.GetVerification(verificationID)
	if err != nil || v.Verification.ProjectID != projectID {
		http.NotFound(w, r)
		return
	}
	evidence, err := s.store.ListVerificationEvidence(verificationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(evidence)
}

func (s *server) uploadEvidence(w http.ResponseWriter, r *http.Request, projectID, verificationID string) {
	v, err := s.store.GetVerification(verificationID)
	if err != nil || v.Verification.ProjectID != projectID {
		http.NotFound(w, r)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEvidenceSize)
	if err := r.ParseMultipartForm(maxEvidenceSize); err != nil {
		http.Error(w, "request too large or invalid multipart form", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	fileHeader := getMultipartFile(r.MultipartForm, "file")
	if fileHeader == nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	if fileHeader.Size > maxEvidenceSize {
		http.Error(w, "file exceeds maximum size", http.StatusBadRequest)
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(data) > maxEvidenceSize {
		http.Error(w, "file exceeds maximum size", http.StatusBadRequest)
		return
	}

	caption := strings.TrimSpace(r.FormValue("caption"))
	position := 0
	if p, err := strconv.Atoi(r.FormValue("position")); err == nil {
		position = p
	}

	evidence, err := s.store.CreateEvidence(projectID, store.CreateEvidenceInput{
		VerificationID: verificationID,
		Data:           data,
		Caption:        caption,
		Position:       position,
	})
	if err != nil {
		if err.Error() == "only PNG and JPEG images are accepted" || strings.Contains(err.Error(), "could not decode image") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = v
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(evidence)
}

func getMultipartFile(form *multipart.Form, key string) *multipart.FileHeader {
	files := form.File[key]
	if len(files) == 0 {
		return nil
	}
	return files[0]
}

func (s *server) serveEvidence(w http.ResponseWriter, r *http.Request, projectID, verificationID, evidenceID string) {
	v, err := s.store.GetVerification(verificationID)
	if err != nil || v.Verification.ProjectID != projectID {
		http.NotFound(w, r)
		return
	}
	evidence, err := s.store.GetEvidence(evidenceID)
	if err != nil || evidence.VerificationID != verificationID {
		http.NotFound(w, r)
		return
	}
	absPath := s.store.EvidenceFilePath(evidence, projectID)
	f, err := os.Open(absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", evidence.MediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filepath.Base(absPath)))
	_, _ = io.Copy(w, f)
}

func (s *server) updateEvidence(w http.ResponseWriter, r *http.Request, projectID, verificationID, evidenceID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v, err := s.store.GetVerification(verificationID)
	if err != nil || v.Verification.ProjectID != projectID {
		http.NotFound(w, r)
		return
	}
	evidence, err := s.store.GetEvidence(evidenceID)
	if err != nil || evidence.VerificationID != verificationID {
		http.NotFound(w, r)
		return
	}

	if r.FormValue("_method") == "delete" {
		if err := s.store.DeleteEvidence(evidenceID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if caption := r.FormValue("caption"); caption != "" || r.Form.Has("caption") {
		if err := s.store.UpdateEvidenceCaption(evidenceID, strings.TrimSpace(caption)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if posStr := r.FormValue("position"); posStr != "" {
		pos, err := strconv.Atoi(posStr)
		if err != nil {
			http.Error(w, "invalid position", http.StatusBadRequest)
			return
		}
		if err := s.store.UpdateEvidenceOrder(evidenceID, pos); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	evidence, err = s.store.GetEvidence(evidenceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(evidence)
}
