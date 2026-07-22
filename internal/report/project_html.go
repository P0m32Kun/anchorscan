package report

import (
	"embed"
	"html/template"
	"io"
	"os"
	"time"
)

//go:embed templates/project_report.html
var projectReportTemplates embed.FS

func projectReportFuncs() template.FuncMap {
	return template.FuncMap{
		"rfc3339": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		// safeURL marks an evidence data URI as a trusted URL so html/template does
		// not rewrite the data: scheme to #ZgotmplZ. Only builder-produced data URIs
		// ever reach this function.
		"safeURL": func(value string) template.URL {
			return template.URL(value)
		},
	}
}

func parseProjectReportTemplate() (*template.Template, error) {
	return template.New("project_report.html").Funcs(projectReportFuncs()).ParseFS(projectReportTemplates, "templates/project_report.html")
}

// RenderProjectHTML writes the single-file project report to w. Evidence images
// are already embedded as data URIs, so the output is fully offline-readable.
func RenderProjectHTML(w io.Writer, deliverable ProjectDeliverable) error {
	tpl, err := parseProjectReportTemplate()
	if err != nil {
		return err
	}
	return tpl.ExecuteTemplate(w, "project_report.html", deliverable)
}

// WriteProjectHTML renders the single-file project report to path.
func WriteProjectHTML(path string, deliverable ProjectDeliverable) error {
	tpl, err := parseProjectReportTemplate()
	if err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return tpl.ExecuteTemplate(file, "project_report.html", deliverable)
}
