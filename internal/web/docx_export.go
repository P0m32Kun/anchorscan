package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

// projectReportDOCX renders the formal DOCX via the docxtpl sidecar. It shares
// the same deliverable as the HTML exporter; evidence file paths are passed to
// the sidecar. When the sidecar or template is not configured, or docxtpl is
// unavailable, it returns a clear 503 without affecting the HTML export.
func (s *server) projectReportDOCX(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.opts.DocxTemplatePath == "" || s.opts.DocxRenderProject == "" {
		http.Error(w, docxUnavailable("DOCX 导出未配置，请先运行 doctor 检查部署环境。"), http.StatusServiceUnavailable)
		return
	}
	if _, err := os.Stat(s.opts.DocxTemplatePath); err != nil {
		log.Printf("docx export: template not found at %s: %v", s.opts.DocxTemplatePath, err)
		http.Error(w, docxUnavailable("DOCX 模板不可用，请检查部署环境。"), http.StatusServiceUnavailable)
		return
	}

	deliverable, err := app.BuildProjectDeliverable(s.store, projectID, s.opts.Now())
	if err != nil {
		writeProjectReportError(w, r, err)
		return
	}

	context := report.BuildDocxContext(deliverable, s.opts.Now())
	contextBytes, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		log.Printf("docx export: marshal context: %v", err)
		http.Error(w, projectReportMessage(app.CodeProjectReportUnavailable, "暂时无法生成报告，请稍后重试。"), http.StatusInternalServerError)
		return
	}

	tmpDir, err := os.MkdirTemp("", "anchorscan-docx-")
	if err != nil {
		log.Printf("docx export: create temp dir: %v", err)
		http.Error(w, projectReportMessage(app.CodeProjectReportUnavailable, "暂时无法生成报告，请稍后重试。"), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	contextPath := filepath.Join(tmpDir, "context.json")
	if err := os.WriteFile(contextPath, contextBytes, 0o644); err != nil {
		log.Printf("docx export: write context: %v", err)
		http.Error(w, projectReportMessage(app.CodeProjectReportUnavailable, "暂时无法生成报告，请稍后重试。"), http.StatusInternalServerError)
		return
	}
	outPath := filepath.Join(tmpDir, safeReportFilename(deliverable.Project.Name)+".docx")

	cmd := exec.Command("uv", "run", "--project", s.opts.DocxRenderProject, "python", filepath.Join(s.opts.DocxRenderProject, "render_docx.py"),
		"--template", s.opts.DocxTemplatePath,
		"--context", contextPath,
		"--out", outPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("docx export: sidecar failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
		http.Error(w, docxUnavailable("DOCX 渲染失败，请检查部署环境。"), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.docx"`, safeReportFilename(deliverable.Project.Name)))
	http.ServeFile(w, r, outPath)
}

func docxUnavailable(message string) string {
	return projectReportMessage(app.CodeDocxUnavailable, message)
}
