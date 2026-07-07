package config

import (
	"os"

	"github.com/P0m32Kun/anchorscan/internal/vuln"
	"gopkg.in/yaml.v3"
)

func LoadNSERules(path string) (map[string][]string, error) {
	var rules map[string][]string
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func LoadTagRules(path string) ([]vuln.TagRule, error) {
	var rules []vuln.TagRule
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}
