package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/vuln"
	"gopkg.in/yaml.v3"
)

var ErrEmptyRuleFile = errors.New("rule file contains no rules")

func LoadNSERules(path string) (map[string][]string, error) {
	var rules map[string][]string
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("NSE rules file %q: %w", path, ErrEmptyRuleFile)
	}
	return rules, nil
}

func LoadTagRules(path string) ([]vuln.TagRule, error) {
	var rules []vuln.TagRule
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rawRules []map[string]any
	if err := yaml.Unmarshal(data, &rawRules); err != nil {
		return nil, err
	}
	for _, rawRule := range rawRules {
		if _, ok := rawRule["template"]; ok {
			return nil, fmt.Errorf("tag rules file %q uses deprecated template; migrate to nuclei_tags", path)
		}
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("tag rules file %q: %w", path, ErrEmptyRuleFile)
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
	candidates := []string{filepath.Join(filepath.Dir(configPath), fileName), filepath.Join("config", fileName)}
	for _, candidate := range candidates {
		value, err := loader(candidate)
		if err == nil {
			return value, nil
		}
		if !os.IsNotExist(err) {
			return zero, err
		}
	}
	return zero, fmt.Errorf("required rule file %q was not found; checked %s: %w", fileName, strings.Join(candidates, ", "), os.ErrNotExist)
}
