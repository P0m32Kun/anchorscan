package web

import (
	"embed"
	"html/template"

	"github.com/P0m32Kun/anchorscan/internal/version"
)

//go:embed templates/*.html static/*
var assets embed.FS

func parseTemplates(files ...string) (*template.Template, error) {
	all := append([]string{"templates/base.html"}, files...)
	return template.New("base").Funcs(template.FuncMap{
		"eq":      func(left string, right string) bool { return left == right },
		"version": func() string { return version.Version },
	}).ParseFS(assets, all...)
}
