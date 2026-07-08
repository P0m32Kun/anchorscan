package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html static/*
var assets embed.FS

func parseTemplates(files ...string) (*template.Template, error) {
	all := append([]string{"templates/base.html"}, files...)
	return template.New("base").Funcs(template.FuncMap{
		"eq": func(left string, right string) bool { return left == right },
	}).ParseFS(assets, all...)
}
