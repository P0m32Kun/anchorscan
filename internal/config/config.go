package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ToolArgs struct {
	Rustscan []string `yaml:"rustscan_args"`
	Nmap     []string `yaml:"nmap_args"`
	Httpx    []string `yaml:"httpx_args"`
	Nuclei   []string `yaml:"nuclei_args"`
}

type ToolPaths struct {
	Rustscan string `yaml:"rustscan"`
	Nmap     string `yaml:"nmap"`
	Httpx    string `yaml:"httpx"`
	Nuclei   string `yaml:"nuclei"`
	// NucleiTemplates is the root of the community nuclei-templates checkout.
	NucleiTemplates string `yaml:"nuclei_templates"`
	Rdpscan         string `yaml:"rdpscan"`
	// Dameng enables the community-template-gated default-password detector.
	// It does not point to an external binary; any non-empty value enables it.
	Dameng string `yaml:"dameng"`
	// Fathom is the self-contained Rust recon binary (M4.1). It replaces the
	// rustscan + nmap -sV port/fingerprint stages, and since M4.4 also owns
	// IPv4 alive probing (ICMP + TCP fallback inside `fathom scan`; nmap -sn
	// survives for IPv6 only). It emits one JSON object per open port (see
	// internal/tools/fathom.go).
	Fathom string `yaml:"fathom"`
}

func (p ToolPaths) DamengTemplatePath() string {
	root := strings.TrimSpace(p.NucleiTemplates)
	if root == "~" || strings.HasPrefix(root, "~/") || strings.HasPrefix(root, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			root = filepath.Join(home, strings.TrimLeft(root[1:], `/\`))
		}
	}
	return filepath.Join(root, "javascript", "detection", "dameng-detect.yaml")
}

type ToolTimeouts struct {
	Rustscan string `yaml:"rustscan"`
	Nmap     string `yaml:"nmap"`
	Httpx    string `yaml:"httpx"`
	NSE      string `yaml:"nse"`
	Nuclei   string `yaml:"nuclei"`
	Rdpscan  string `yaml:"rdpscan"`
	Dameng   string `yaml:"dameng"`
	Fathom   string `yaml:"fathom"`
}

type ToolDurations struct {
	Rustscan time.Duration
	Nmap     time.Duration
	Httpx    time.Duration
	NSE      time.Duration
	Nuclei   time.Duration
	Rdpscan  time.Duration
	Dameng   time.Duration
	Fathom   time.Duration
}

func (timeouts ToolTimeouts) Durations() (ToolDurations, error) {
	parse := func(name, value string) (time.Duration, error) {
		if value == "" || value == "0" {
			return 0, nil
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 0 {
			return 0, fmt.Errorf("invalid %s timeout: %q", name, value)
		}
		return duration, nil
	}
	var out ToolDurations
	var err error
	if out.Rustscan, err = parse("rustscan", timeouts.Rustscan); err != nil {
		return out, err
	}
	if out.Nmap, err = parse("nmap", timeouts.Nmap); err != nil {
		return out, err
	}
	if out.Httpx, err = parse("httpx", timeouts.Httpx); err != nil {
		return out, err
	}
	if out.NSE, err = parse("nse", timeouts.NSE); err != nil {
		return out, err
	}
	if out.Nuclei, err = parse("nuclei", timeouts.Nuclei); err != nil {
		return out, err
	}
	if out.Rdpscan, err = parse("rdpscan", timeouts.Rdpscan); err != nil {
		return out, err
	}
	if out.Dameng, err = parse("dameng", timeouts.Dameng); err != nil {
		return out, err
	}
	if out.Fathom, err = parse("fathom", timeouts.Fathom); err != nil {
		return out, err
	}
	return out, nil
}

func (timeouts ToolTimeouts) Normalized() ToolTimeouts {
	for _, value := range []*string{&timeouts.Rustscan, &timeouts.Nmap, &timeouts.Httpx, &timeouts.NSE, &timeouts.Nuclei, &timeouts.Rdpscan, &timeouts.Dameng, &timeouts.Fathom} {
		if *value == "" {
			*value = "0"
		}
	}
	return timeouts
}

type Profile struct {
	HostWorkers int `yaml:"host_workers"`
	ToolArgs    `yaml:",inline"`
}

type Config struct {
	Tools         ToolPaths    `yaml:"tools"`
	Timeouts      ToolTimeouts `yaml:"timeouts"`
	KnowledgeBase struct {
		Path string `yaml:"path"`
	} `yaml:"knowledge_base"`
	Scan struct {
		Ports   string `yaml:"ports"`
		Profile string `yaml:"profile"`
	} `yaml:"scan"`
	Profiles map[string]Profile `yaml:"profiles"`
}

func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := EnsureInit(path); err != nil {
			return cfg, err
		}
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Tools.NucleiTemplates == "" {
		cfg.Tools.NucleiTemplates = "~/nuclei-templates"
	}
	if cfg.Scan.Ports == "" {
		cfg.Scan.Ports = "top1000"
	}
	if cfg.Scan.Profile == "" {
		cfg.Scan.Profile = "normal"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if _, err := cfg.Timeouts.Durations(); err != nil {
		return cfg, err
	}
	cfg.Timeouts = cfg.Timeouts.Normalized()

	return cfg, nil
}
