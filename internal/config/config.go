package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Tools struct {
		Rustscan string `yaml:"rustscan"`
		Nmap     string `yaml:"nmap"`
		Httpx    string `yaml:"httpx"`
		Nuclei   string `yaml:"nuclei"`
	} `yaml:"tools"`
	Scan struct {
		Ports string `yaml:"ports"`
	} `yaml:"scan"`
}

func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Scan.Ports == "" {
		cfg.Scan.Ports = "top100"
	}

	return cfg, nil
}
