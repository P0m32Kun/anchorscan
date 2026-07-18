package config

import (
	"errors"
	"fmt"
	"os"
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
}

type ToolTimeouts struct {
	Rustscan string `yaml:"rustscan"`
	Nmap     string `yaml:"nmap"`
	Httpx    string `yaml:"httpx"`
	NSE      string `yaml:"nse"`
	Nuclei   string `yaml:"nuclei"`
}

type ToolDurations struct {
	Rustscan time.Duration
	Nmap     time.Duration
	Httpx    time.Duration
	NSE      time.Duration
	Nuclei   time.Duration
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
	return out, nil
}

func (timeouts ToolTimeouts) Normalized() ToolTimeouts {
	for _, value := range []*string{&timeouts.Rustscan, &timeouts.Nmap, &timeouts.Httpx, &timeouts.NSE, &timeouts.Nuclei} {
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
