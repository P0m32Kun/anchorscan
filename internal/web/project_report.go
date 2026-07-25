package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

// projectReportHTML renders the single-file formal project report.
func (s *server) projectReportHTML(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deliverable, err := app.BuildProjectDeliverable(s.store, projectID, s.opts.Now())
	if err != nil {
		writeProjectReportError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.html"`, safeReportFilename(deliverable.Project.Name)))
	if err := report.RenderProjectHTML(w, deliverable); err != nil {
		http.Error(w, "暂时无法生成报告，请稍后重试。", http.StatusInternalServerError)
	}
}

func writeProjectReportError(w http.ResponseWriter, r *http.Request, err error) {
	var perr *app.ProjectReportError
	if errors.As(err, &perr) {
		switch perr.Code {
		case app.CodeProjectNotFound:
			http.Error(w, perr.Message, http.StatusNotFound)
		case app.CodeProjectReportInvalid:
			http.Error(w, perr.Message, http.StatusBadRequest)
		default:
			http.Error(w, perr.Message, http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, "暂时无法生成报告，请稍后重试。", http.StatusInternalServerError)
}

func safeReportFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "project-report"
	}
	return strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_").Replace(name)
}
