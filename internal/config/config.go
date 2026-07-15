package config

import (
	"errors"
	"os"

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

type Profile struct {
	HostWorkers int `yaml:"host_workers"`
	ToolArgs    `yaml:",inline"`
}

type Config struct {
	Tools         ToolPaths `yaml:"tools"`
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

	return cfg, nil
}
