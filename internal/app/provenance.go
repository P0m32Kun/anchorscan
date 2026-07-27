package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/version"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

// RunProvenance records the historical facts needed to reproduce or audit a run.
// It is persisted as opaque JSON so the store layer stays independent of its schema.
type RunProvenance struct {
	Version        string              `json:"version"`
	StartedAt      time.Time           `json:"started_at"`
	FinishedAt     time.Time           `json:"finished_at,omitempty"`
	Scope          string              `json:"scope,omitempty"`
	ToolVersions   map[string]string   `json:"tool_versions,omitempty"`
	RuleHashes     map[string]string   `json:"rule_hashes,omitempty"`
	NSEScripts     map[string][]string `json:"nse_scripts,omitempty"`
	Tags           map[string][]string `json:"tags,omitempty"`
	TemplateIDs    []string            `json:"template_ids,omitempty"`
	ArtifactHashes map[string]string   `json:"artifact_hashes,omitempty"`
	RedactedConfig string              `json:"redacted_config,omitempty"`
}

// RunProvenanceJSON marshals the provenance record to JSON.
func (p RunProvenance) JSON() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// ProvenanceOptions supplies the context needed to build a RunProvenance.
type ProvenanceOptions struct {
	Version         string
	RulePaths       []string
	NSERules        map[string][]string
	TagRules        []vuln.TagRule
	TemplatePath    string
	Tags            []string
	Tools           config.ToolPaths
	ConfigSnapshot  string
	Scope           string
	VersionProvider func(name, path string) string
}

// BuildRunProvenance creates a provenance record for a completed run.
func BuildRunProvenance(opts ProvenanceOptions, startedAt, finishedAt time.Time, artifactHashes map[string]string) RunProvenance {
	p := RunProvenance{
		Version:        firstNonEmpty(opts.Version, version.Version),
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		Scope:          firstNonEmpty(opts.Scope, opts.ConfigSnapshot),
		ToolVersions:   toolVersions(opts.Tools, opts.VersionProvider),
		ArtifactHashes: artifactHashes,
		RedactedConfig: redactedConfigSnapshot(opts.ConfigSnapshot),
	}
	if len(opts.RulePaths) > 0 {
		p.RuleHashes = hashRulePaths(opts.RulePaths)
	}
	if opts.TemplatePath != "" {
		if p.RuleHashes == nil {
			p.RuleHashes = map[string]string{}
		}
		if h, err := hashFile(opts.TemplatePath); err == nil {
			p.RuleHashes[opts.TemplatePath] = h
		}
		p.TemplateIDs = append(p.TemplateIDs, opts.TemplatePath)
	}
	if len(opts.NSERules) > 0 {
		p.NSEScripts = shallowCopyMapSlice(opts.NSERules)
	}
	if len(opts.TagRules) > 0 {
		tags := make(map[string][]string, len(opts.TagRules))
		var templates []string
		for _, rule := range opts.TagRules {
			if len(rule.NucleiTags) > 0 {
				key := rule.Name
				if key == "" {
					key = strings.Join(rule.Service, ",")
				}
				tags[key] = append([]string(nil), rule.NucleiTags...)
			}
			if rule.Template != "" {
				templates = append(templates, rule.Template)
			}
		}
		if len(tags) > 0 {
			p.Tags = tags
		}
		p.TemplateIDs = append(p.TemplateIDs, templates...)
	}
	if len(opts.Tags) > 0 {
		if p.Tags == nil {
			p.Tags = map[string][]string{}
		}
		p.Tags["nuclei"] = append([]string(nil), opts.Tags...)
	}
	p.TemplateIDs = dedupeStrings(p.TemplateIDs)
	return p
}

// ReportProvenance derives a customer-facing provenance summary from the full
// run provenance. It excludes hashes and raw config.
func ReportProvenance(p RunProvenance, engines []string) report.Provenance {
	scope := p.Scope
	if p.RedactedConfig != "" {
		scope = p.RedactedConfig
	}
	return report.Provenance{
		Version:    p.Version,
		StartedAt:  formatProvenanceTime(p.StartedAt),
		FinishedAt: formatProvenanceTime(p.FinishedAt),
		Scope:      scope,
		Engines:    engines,
	}
}

// EnginesFromDetectionChecks returns the distinct engine names that completed.
func EnginesFromDetectionChecks(checks []report.DetectionCheck) []string {
	seen := map[string]struct{}{}
	for _, check := range checks {
		if check.Status == "completed" {
			seen[check.Engine] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// HashArtifactFiles walks dir and returns SHA-256 hashes keyed by relative path.
// Empty or non-existent directories return an empty map without error.
func HashArtifactFiles(dir string) (map[string]string, error) {
	hashes := map[string]string{}
	if dir == "" {
		return hashes, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return hashes, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("artifact path is not a directory: %s", dir)
	}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		h, err := hashFile(path)
		if err != nil {
			return err
		}
		hashes[rel] = h
		return nil
	})
	return hashes, err
}

// HashFile returns the SHA-256 hex digest of a file.
func HashFile(path string) (string, error) {
	return hashFile(path)
}

func toolVersions(tools config.ToolPaths, provider func(name, path string) string) map[string]string {
	m := map[string]string{
		"rustscan": tools.Rustscan,
		"nmap":     tools.Nmap,
		"httpx":    tools.Httpx,
		"nuclei":   tools.Nuclei,
		"rdpscan":  tools.Rdpscan,
	}
	for name, path := range m {
		if path == "" {
			delete(m, name)
			continue
		}
		if provider != nil {
			m[name] = provider(name, path)
		}
	}
	return m
}

func hashRulePaths(paths []string) map[string]string {
	hashes := make(map[string]string, len(paths))
	for _, p := range paths {
		if h, err := hashFile(p); err == nil {
			hashes[p] = h
		}
	}
	return hashes
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func redactedConfigSnapshot(snapshot string) string {
	if snapshot == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(snapshot), &v); err != nil {
		return maskPlaintext(snapshot)
	}
	redactValue(v)
	b, _ := json.Marshal(v)
	return string(b)
}

func redactValue(v any) {
	switch x := v.(type) {
	case map[string]any:
		for key, val := range x {
			switch key {
			case "native_args", "extra_args", "args", "password", "secret", "token", "key":
				x[key] = "REDACTED"
			default:
				if s, ok := val.(string); ok {
					x[key] = maskURLUserinfo(s)
				} else {
					redactValue(val)
				}
			}
		}
	case []any:
		for i, val := range x {
			redactValue(val)
			_ = i
		}
	}
}

func maskPlaintext(s string) string {
	// Best-effort mask for non-JSON snapshots: strip URL userinfo and replace
	// anything that looks like a secret key/value pair.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = maskURLUserinfo(line)
		for _, key := range []string{"password", "secret", "token", "key"} {
			if idx := strings.Index(strings.ToLower(line), key+"="); idx != -1 {
				prefix := line[:idx+len(key)+1]
				lines[i] = prefix + "REDACTED"
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func maskURLUserinfo(s string) string {
	if !strings.Contains(s, "://") {
		return s
	}
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	u.User = url.User("REDACTED")
	return u.String()
}

func shallowCopyMapSlice(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	out := make(map[string][]string, len(src))
	for k, v := range src {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func dedupeStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := values[:0]
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func formatProvenanceTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// SaveRunProvenance persists the initial/final manifest for a run. It is safe to
// call multiple times; later calls overwrite the manifest for the same run_id.
func SaveRunProvenance(scanStore *store.Store, runID string, p RunProvenance) error {
	if scanStore == nil {
		return nil
	}
	return scanStore.SaveRunProvenance(runID, p.JSON())
}

// UpdateRunProvenanceArtifactHashes updates the artifact hashes in the stored
// manifest after the run has produced its artifacts.
func UpdateRunProvenanceArtifactHashes(scanStore *store.Store, runID string, p RunProvenance, artifactHashes map[string]string) (RunProvenance, error) {
	if scanStore == nil {
		return p, nil
	}
	p.FinishedAt = time.Now()
	p.ArtifactHashes = artifactHashes
	if err := scanStore.SaveRunProvenance(runID, p.JSON()); err != nil {
		return p, err
	}
	return p, nil
}

// redactedToolConfigSnapshot builds a customer-safe snapshot of a single-tool run.
func redactedToolConfigSnapshot(opts ToolRunOptions) string {
	extra := ""
	if hasExtraArgs(opts.ExtraArgs) {
		extra = "REDACTED"
	}
	m := map[string]any{
		"tool":       opts.Tool,
		"mode":       opts.Mode,
		"target":     opts.Target,
		"url":        maskURLUserinfo(opts.URL),
		"ports":      opts.Ports,
		"tags":       opts.Tags,
		"template":   opts.Template,
		"use_native": opts.UseNativeArgs,
		"extra_args": extra,
	}
	if opts.UseNativeArgs {
		m["native_args"] = []string{"REDACTED"}
	}
	b, _ := json.Marshal(m)
	return string(b)
}
