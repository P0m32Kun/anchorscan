package web

import (
	"net/http"
	"strings"
)

func (s *server) knowledgeBaseList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	render(w, "templates/knowledgebase.html", map[string]any{"Status": s.catalog.Status(), "Diagnostics": s.catalog.Diagnostics(), "Entries": s.catalog.Search(r.URL.Query().Get("q")), "Query": r.URL.Query().Get("q")})
}

func (s *server) knowledgeBaseDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/kb/")
	entry, ok := s.catalog.Entry(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	render(w, "templates/knowledgebase_detail.html", map[string]any{"Entry": entry})
}
