package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
)

type configPageData struct {
	Config        config.Config
	RawConfig     string
	Error         string
	HighriskPorts string
}

func (s *server) configPage(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	raw, err := os.ReadFile(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
		render(w, "templates/config.html", configPageData{Config: cfg, RawConfig: string(raw), HighriskPorts: highriskPorts})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("mode") == "raw" {
			rawConfig := r.FormValue("raw_config")
			if _, err := config.SaveRawWithBackup(s.opts.ConfigPath, rawConfig, s.opts.Now()); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
				render(w, "templates/config.html", configPageData{
					Config:        cfg,
					RawConfig:     rawConfig,
					Error:         "invalid YAML: " + err.Error(),
					HighriskPorts: highriskPorts,
				})
				return
			}
			http.Redirect(w, r, "/config", http.StatusSeeOther)
			return
		}
		cfg.Tools.Rustscan = r.FormValue("rustscan")
		cfg.Tools.Nmap = r.FormValue("nmap")
		cfg.Tools.Httpx = r.FormValue("httpx")
		cfg.Tools.Nuclei = r.FormValue("nuclei")
		cfg.Scan.Ports = r.FormValue("ports")
		cfg.Scan.Profile = r.FormValue("profile")
		if _, err := config.SaveWithBackup(s.opts.ConfigPath, cfg, s.opts.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/config", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// configPorts handles saving the highrisk port preset file from the config page.
func (s *server) configPorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dir := filepath.Dir(s.opts.ConfigPath)
	normalized := normalizePortCSV(r.FormValue("highrisk_ports"))
	if _, err := ports.SavePresetWithBackup("highrisk", dir, normalized, s.opts.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/config", http.StatusSeeOther)
}

// normalizePortCSV accepts free-form port input (comma / newline / space
// separated) and returns a single trimmed, comma-separated CSV line.
func normalizePortCSV(value string) string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return strings.Join(out, ",")
}
