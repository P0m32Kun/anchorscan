package report

import (
	"embed"
	"html/template"
	"os"
)

//go:embed templates/report.html
var reportTemplates embed.FS

func WriteHTML(path string, scanReport ScanReport) error {
	tpl, err := template.ParseFS(reportTemplates, "templates/report.html")
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return tpl.ExecuteTemplate(file, "report.html", scanReport)
}
