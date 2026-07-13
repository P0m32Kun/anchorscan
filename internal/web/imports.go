package web

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/P0m32Kun/anchorscan/internal/app"
)

func (s *server) importNmapForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.renderImportForm(w, "")
}

// renderImportForm renders the Nmap XML import page with the project list and
// an optional error message shown in a top banner.
func (s *server) renderImportForm(w http.ResponseWriter, errMsg string) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/import_nmap.html", map[string]any{
		"Projects": projects,
		"Error":    errMsg,
	})
}

// importNmapRun handles the POST submission of the Nmap XML import form. A
// valid upload is imported as a completed run and the client is redirected to
// its detail page; on any failure the form is re-rendered with an error banner
// (no run is persisted).
func (s *server) importNmapRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		s.renderImportForm(w, "文件过大或格式错误")
		return
	}

	file, _, err := r.FormFile("xml_file")
	if err != nil {
		s.renderImportForm(w, "请选择要导入的 Nmap XML 文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		s.renderImportForm(w, err.Error())
		return
	}
	if len(bytes.TrimSpace(data)) == 0 {
		s.renderImportForm(w, "XML 文件为空")
		return
	}

	projectID := r.FormValue("project_id")
	runID := newID("run", s.opts.Now())
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)

	if _, err := app.ImportNmap(context.Background(), s.store, app.ImportNmapOptions{
		XMLData:   data,
		RunID:     runID,
		ProjectID: projectID,
		JSONPath:  jsonPath,
	}); err != nil {
		s.renderImportForm(w, err.Error())
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
}
