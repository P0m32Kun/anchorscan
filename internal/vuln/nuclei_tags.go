package vuln

import (
	"net"
	"strconv"
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

// defaultExcludedNucleiTags are globally excluded from tag-based nuclei runs
// because they select templates unrelated to the intended service-specific
// check (fuzz/dos). Credential checks (default-login/brute) are intentionally
// not excluded here; services opt into them individually.
var defaultExcludedNucleiTags = []string{"fuzz", "dos"}

func MatchNucleiTags(fp fingerprint.ServiceFingerprint, http HTTPResult, rules []TagRule) MatchResult {
	for _, rule := range rules {
		if contains(rule.Service, fp.Normalized) || contains(rule.Product, fp.Product) || overlaps(rule.Tech, http.Tech) {
			address := net.JoinHostPort(fp.IP, strconv.Itoa(fp.Port))
			if rule.Target == "url" && http.URL != "" {
				address = http.URL
			}
			result := MatchResult{
				Tags:    append([]string(nil), rule.NucleiTags...),
				Target:  rule.Target,
				Address: address,
			}
			result.ExcludeTags = append([]string(nil), defaultExcludedNucleiTags...)
			result.ExcludeTags = append(result.ExcludeTags, rule.ExcludeTags...)
			return result
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
