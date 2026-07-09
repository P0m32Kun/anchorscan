package config

import (
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Init writes a default config file to path when none exists yet. Tool paths
// are auto-detected from the system PATH; the scan defaults and profiles mirror
// the built-in values so a freshly generated config behaves like the shipped
// defaults. The parent directory is created if missing.
func Init(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out, err := yaml.Marshal(defaultConfig())
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// defaultConfig builds the Config written by Init. Tool binaries are resolved
// from PATH so a fresh clone works without manual path editing.
func defaultConfig() Config {
	cfg := Config{
		Scan: struct {
			Ports   string `yaml:"ports"`
			Profile string `yaml:"profile"`
		}{
			Ports:   "top100",
			Profile: "normal",
		},
		Profiles: builtInProfiles(),
	}
	cfg.Tools.Rustscan = detectToolPath("rustscan")
	cfg.Tools.Nmap = detectToolPath("nmap")
	cfg.Tools.Httpx = detectToolPath("httpx")
	cfg.Tools.Nuclei = detectToolPath("nuclei")
	return cfg
}

// detectToolPath returns the absolute path of a tool found on PATH, or an empty
// string when it is not installed. An empty value lets doctor surface a clear
// "not configured" message instead of silently failing at scan time.
func detectToolPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// EnsureInit is a convenience used by Load: if path does not exist, generate it.
func EnsureInit(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return Init(path)
}
