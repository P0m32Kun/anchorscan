package vuln

import (
	"fmt"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

type HTTPResult struct {
	URL  string
	Tech []string
}

type TagRule struct {
	Name        string   `yaml:"name"`
	Service     []string `yaml:"service"`
	Product     []string `yaml:"product"`
	Tech        []string `yaml:"tech"`
	NucleiTags  []string `yaml:"nuclei_tags"`
	ExcludeTags []string `yaml:"exclude_tags"`
	Target      string   `yaml:"target"`
}

type MatchResult struct {
	Tags        []string
	ExcludeTags []string
	Target      string
	Address     string
}

func MatchNucleiTags(fp fingerprint.ServiceFingerprint, http HTTPResult, rules []TagRule) MatchResult {
	for _, rule := range rules {
		if contains(rule.Service, fp.Normalized) || contains(rule.Product, fp.Product) || overlaps(rule.Tech, http.Tech) {
			address := fmt.Sprintf("%s:%d", fp.IP, fp.Port)
			if rule.Target == "url" && http.URL != "" {
				address = http.URL
			}
			return MatchResult{
				Tags:        append([]string(nil), rule.NucleiTags...),
				ExcludeTags: append([]string(nil), rule.ExcludeTags...),
				Target:      rule.Target,
				Address:     address,
			}
		}
	}
	return MatchResult{}
}

func contains(items []string, value string) bool {
	value = strings.ToLower(value)
	for _, item := range items {
		if strings.ToLower(item) == value {
			return true
		}
	}
	return false
}

func overlaps(left []string, right []string) bool {
	for _, item := range right {
		if contains(left, item) {
			return true
		}
	}
	return false
}
