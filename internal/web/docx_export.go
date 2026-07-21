package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
		http.Error(w, "DOCX 导出未配置：缺少 docxtpl sidecar 或模板路径，请先运行 doctor 检查。", http.StatusServiceUnavailable)
		return
	}
	if _, err := os.Stat(s.opts.DocxTemplatePath); err != nil {
		http.Error(w, "DOCX 模板不存在："+s.opts.DocxTemplatePath, http.StatusServiceUnavailable)
		return
	}

	deliverable, project, err := s.buildProjectDeliverable(w, r, projectID)
	if err != nil {
		return
	}

	context := report.BuildDocxContext(deliverable, s.opts.Now())
	contextBytes, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpDir, err := os.MkdirTemp("", "anchorscan-docx-")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	contextPath := filepath.Join(tmpDir, "context.json")
	if err := os.WriteFile(contextPath, contextBytes, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	outPath := filepath.Join(tmpDir, safeReportFilename(project)+".docx")

	cmd := exec.Command("uv", "run", "--project", s.opts.DocxRenderProject, "python", "render_docx.py",
		"--template", s.opts.DocxTemplatePath,
		"--context", contextPath,
		"--out", outPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		http.Error(w, "DOCX 渲染失败（docxtpl sidecar 未安装或出错）："+strings.TrimSpace(stderr.String()), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.docx"`, safeReportFilename(project)))
	http.ServeFile(w, r, outPath)
	_ = time.Now
}
