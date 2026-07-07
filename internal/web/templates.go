package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html static/*
var assets embed.FS

func parseTemplates(files ...string) (*template.Template, error) {
	all := append([]string{"templates/base.html"}, files...)
	return template.ParseFS(assets, all...)
}
