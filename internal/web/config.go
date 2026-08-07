package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/doctor"
	"github.com/P0m32Kun/anchorscan/internal/ports"
)

type configPageData struct {
	Config        config.Config
	RawConfig     string
	Error         string
	HighriskPorts string
	Saved         bool
	ToolChecks    []doctor.Check
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
		render(w, "templates/config.html", configPageData{Config: cfg, RawConfig: string(raw), HighriskPorts: highriskPorts, Saved: r.URL.Query().Get("saved") == "1", ToolChecks: toolDiagnostics(s.opts.ConfigPath, s.opts.DBPath, cfg)})
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
		cfg.Tools.Fathom = r.FormValue("fathom")
		cfg.Tools.Rustscan = r.FormValue("rustscan")
		cfg.Tools.Nmap = r.FormValue("nmap")
		cfg.Tools.Httpx = r.FormValue("httpx")
		cfg.Tools.Nuclei = r.FormValue("nuclei")
		cfg.Tools.NucleiTemplates = r.FormValue("nuclei_templates")
		cfg.Tools.Rdpscan = r.FormValue("rdpscan")
		cfg.Timeouts.Fathom = r.FormValue("timeout_fathom")
		cfg.Timeouts.Rustscan = r.FormValue("timeout_rustscan")
		cfg.Timeouts.Nmap = r.FormValue("timeout_nmap")
		cfg.Timeouts.Httpx = r.FormValue("timeout_httpx")
		cfg.Timeouts.NSE = r.FormValue("timeout_nse")
		cfg.Timeouts.Nuclei = r.FormValue("timeout_nuclei")
		cfg.Timeouts.Rdpscan = r.FormValue("timeout_rdpscan")
		cfg.Timeouts = cfg.Timeouts.Normalized()
		cfg.KnowledgeBase.Path = r.FormValue("knowledge_base_path")
		cfg.Scan.Ports = r.FormValue("ports")
		cfg.Scan.Profile = r.FormValue("profile")
		if _, err := config.SaveWithBackup(s.opts.ConfigPath, cfg, s.opts.Now()); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
			render(w, "templates/config.html", configPageData{Config: cfg, RawConfig: string(raw), Error: err.Error(), HighriskPorts: highriskPorts})
			return
		}
		http.Redirect(w, r, "/config?saved=1", http.StatusSeeOther)
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

// toolDiagnostics runs the external-tool subset of the doctor checks and
// returns only the tool/rule results so the config page can surface clear,
// actionable availability hints (installed path vs missing/not-configured)
// without re-running database/report/docx diagnostics that are irrelevant
// here. dbPath is still forwarded so doctor's database check never receives
// an empty path (which would make store.Open create a junk "?_pragma=..." file
// in the working directory — ISSUE-001).
func toolDiagnostics(configPath, dbPath string, cfg config.Config) []doctor.Check {
	toolNames := map[string]bool{"fathom": true, "rustscan": true, "nmap": true, "httpx": true, "nuclei": true, "rdpscan": true, "nse rules": true, "tag rules": true}
	var tools []doctor.Check
	for _, check := range doctor.Run(doctor.Options{ConfigPath: configPath, DBPath: dbPath}) {
		if toolNames[check.Name] {
			tools = append(tools, check)
		}
	}
	return tools
}
