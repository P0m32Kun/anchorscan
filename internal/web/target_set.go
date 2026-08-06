package web

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/target"
)

// targetSetImportResult is the outcome surfaced to the project page after an
// import: accepted / duplicate / rejected counts plus the rejection reasons.
type targetSetImportResult struct {
	Accepted   int                    `json:"accepted"`
	Duplicates int                    `json:"duplicates"`
	Rejected   []target.RejectedTarget `json:"rejected"`
}

// projectTargetSet handles POST /projects/{id}/target-set: parse, validate,
// normalize, dedup, persist, then redirect back to the project page with the
// import statistics in the query string so the user sees what was accepted,
// duplicated, and rejected.
func (s *server) projectTargetSet(w http.ResponseWriter, r *http.Request, projectID string) {
	if _, err := s.store.GetProject(projectID); err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	input := r.FormValue("targets")
	result := target.ParseTargetSet(input)
	acceptedText := strings.Join(result.Accepted, "\n")

	if len(result.Accepted) > 0 {
		if _, err := s.store.SaveTargetSet(projectID, name, acceptedText); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// An import that accepted nothing still replaces the target set: the
		// project's intake list becomes empty until a valid import succeeds.
		if _, err := s.store.SaveTargetSet(projectID, name, ""); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	reasonsJSON, _ := json.Marshal(result.Rejected)
	redirect := "/projects/" + projectID + "?targets_imported=1" +
		"&accepted=" + strconv.Itoa(len(result.Accepted)) +
		"&duplicates=" + strconv.Itoa(len(result.Duplicates)) +
		"&rejected=" + strconv.Itoa(len(result.Rejected)) +
		"&reasons=" + url.QueryEscape(string(reasonsJSON))
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// loadTargetSet returns the project's Target Set or nil when absent.
func (s *server) loadTargetSet(projectID string) *store.TargetSet {
	ts, err := s.store.GetTargetSet(projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return nil
	}
	return &ts
}

// targetSetImportFromQuery reconstructs the import statistics from the
// redirect query string produced by projectTargetSet.
func targetSetImportFromQuery(query url.Values) *targetSetImportResult {
	if query.Get("targets_imported") != "1" {
		return nil
	}
	result := &targetSetImportResult{}
	if accepted, err := strconv.Atoi(query.Get("accepted")); err == nil {
		result.Accepted = accepted
	}
	if duplicates, err := strconv.Atoi(query.Get("duplicates")); err == nil {
		result.Duplicates = duplicates
	}
	if reasons := query.Get("reasons"); reasons != "" {
		_ = json.Unmarshal([]byte(reasons), &result.Rejected)
	}
	result.Rejected = result.Rejected[:min(len(result.Rejected), 20)]
	return result
}
