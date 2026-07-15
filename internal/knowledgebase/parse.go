package knowledgebase

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"gopkg.in/yaml.v3"
)

var entryHeading = regexp.MustCompile(`^### (.+)（(严重|高|中|低)危）$`)

type entryMeta struct {
	ID      string   `yaml:"id"`
	Aliases []string `yaml:"aliases"`
	Match   struct {
		Nuclei []string `yaml:"nuclei"`
		NSE    []string `yaml:"nse"`
		Manual []string `yaml:"manual-review"`
		CVE    []string `yaml:"cve"`
	} `yaml:"match"`
}

func Load(configPath, configuredPath string) *Catalog {
	if strings.TrimSpace(configuredPath) == "" {
		return &Catalog{status: StatusDisabled, byID: map[string]Entry{}}
	}
	path := configuredPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(configPath), path)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return unavailable(err.Error())
	}
	return parseCatalog(string(data))
}

func parseCatalog(source string) *Catalog {
	if strings.Count(source, "<!-- anchorscan-catalog") != 1 || !strings.Contains(source, "version: 1") {
		return unavailable("缺少或不支持 anchorscan-catalog version: 1")
	}
	lines := strings.Split(source, "\n")
	entries := make([]Entry, 0)
	diagnostics := make([]Diagnostic, 0)
	ids := map[string]bool{}
	for start := 0; start < len(lines); {
		match := entryHeading.FindStringSubmatch(strings.TrimSpace(lines[start]))
		if match == nil {
			start++
			continue
		}
		end := start + 1
		for end < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[end]), "### ") {
			end++
		}
		entry, diagnostic := parseEntry(match[1], match[2], lines[start+1:end], start+1)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			if entry.ID != "" {
				entries = append(entries, entry)
			}
		} else if ids[entry.ID] {
			return unavailable("重复条目 ID: " + entry.ID)
		} else {
			ids[entry.ID] = true
			entries = append(entries, entry)
		}
		start = end
	}
	if len(entries) == 0 {
		return unavailable("没有有效漏洞条目")
	}
	catalog := newCatalog(entries)
	catalog.diagnostics = diagnostics
	if len(diagnostics) > 0 {
		catalog.status = StatusDegraded
	}
	return catalog
}

func parseEntry(name, chineseSeverity string, lines []string, line int) (Entry, *Diagnostic) {
	metaStart, metaEnd := -1, -1
	for i, value := range lines {
		if strings.TrimSpace(value) == "<!-- anchorscan-entry" {
			metaStart = i + 1
		}
		if metaStart >= 0 && strings.TrimSpace(value) == "-->" {
			metaEnd = i
			break
		}
	}
	if metaStart < 0 || metaEnd < metaStart {
		return Entry{}, diagnostic(line, "", "缺少条目元数据")
	}
	for _, value := range lines[:metaStart-1] {
		if strings.TrimSpace(value) != "" {
			return Entry{}, diagnostic(line, "", "标题与元数据之间只能包含空白行")
		}
	}
	var meta entryMeta
	decoder := yaml.NewDecoder(strings.NewReader(strings.Join(lines[metaStart:metaEnd], "\n")))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil || meta.ID == "" {
		return Entry{}, diagnostic(line, meta.ID, "元数据无效")
	}
	if !hasRequiredSectionOrder(lines[metaEnd+1:]) {
		return Entry{}, diagnostic(line, meta.ID, "固定章节必须唯一且按顺序出现")
	}
	description, okDescription := section(lines, "漏洞描述")
	_, okCommands := section(lines, "验证命令")
	remediation, okRemediation := section(lines, "修复建议")
	if !okDescription || !okCommands || !okRemediation || description == "" || remediation == "" {
		return Entry{}, diagnostic(line, meta.ID, "缺少固定章节")
	}
	entry := Entry{ID: meta.ID, Name: name, Severity: parseSeverity(chineseSeverity), Aliases: meta.Aliases, Match: MatchKeys{ToolIDs: append(meta.Match.Nuclei, meta.Match.NSE...), CVEs: meta.Match.CVE, Names: append([]string(nil), meta.Match.Manual...)}, Description: description, Remediation: remediation}
	if command, ok := commandBlock(lines, "Nuclei"); ok {
		if validNuclei(command) {
			entry.Commands.Nuclei = command
		} else {
			return entry, diagnostic(line, meta.ID, "Nuclei 命令无效")
		}
	}
	if strings.Contains(strings.Join(lines, "\n"), "-oX") || strings.Contains(strings.Join(lines, "\n"), " -o ") {
		return entry, diagnostic(line, meta.ID, "命令包含输出参数")
	}
	return entry, nil
}

func hasRequiredSectionOrder(lines []string) bool {
	var headings []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#### ") {
			headings = append(headings, strings.TrimPrefix(line, "#### "))
		}
	}
	return len(headings) == 3 && headings[0] == "漏洞描述" && headings[1] == "验证命令" && headings[2] == "修复建议"
}

func section(lines []string, title string) (string, bool) {
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "#### "+title {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "#### ") {
			end = i
			break
		}
	}
	value := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	return value, true
}

func commandBlock(lines []string, tool string) (string, bool) {
	for i, line := range lines {
		if strings.TrimSpace(line) != "##### "+tool {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) != "```bash" {
				continue
			}
			for k := j + 1; k < len(lines); k++ {
				if strings.TrimSpace(lines[k]) == "```" {
					return strings.TrimSpace(strings.Join(lines[j+1:k], "\n")), true
				}
			}
		}
	}
	return "", false
}

func validNuclei(command string) bool {
	args, err := config.SplitArgs(command)
	return err == nil && len(args) == 5 && args[0] == "nuclei" && args[1] == "-t" && args[3] == "-u" && (args[4] == "{{url}}" || args[4] == "{{host}}:{{port}}")
}

func parseSeverity(value string) Severity {
	return map[string]Severity{"严重": SeverityCritical, "高": SeverityHigh, "中": SeverityMedium, "低": SeverityLow}[value]
}

func diagnostic(line int, id, reason string) *Diagnostic {
	return &Diagnostic{Status: StatusDegraded, EntryID: id, Line: line, Reason: reason}
}
func unavailable(reason string) *Catalog {
	return &Catalog{status: StatusUnavailable, diagnostics: []Diagnostic{{Status: StatusUnavailable, Reason: fmt.Sprint(reason)}}, byID: map[string]Entry{}}
}
