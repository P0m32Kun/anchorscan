package config

import (
	"errors"
	"os"
	"path/filepath"

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

func LoadNSERulesForConfig(configPath string) (map[string][]string, error) {
	return loadRuleFileForConfig(configPath, "nse.yaml", LoadNSERules)
}

func LoadTagRulesForConfig(configPath string) ([]vuln.TagRule, error) {
	return loadRuleFileForConfig(configPath, "service-tags.yaml", LoadTagRules)
}

func loadRuleFileForConfig[T any](configPath string, fileName string, loader func(string) (T, error)) (T, error) {
	var zero T
	for _, candidate := range []string{filepath.Join(filepath.Dir(configPath), fileName), filepath.Join("config", fileName)} {
		value, err := loader(candidate)
		if err == nil {
			return value, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return zero, err
	}
	return zero, nil
}
