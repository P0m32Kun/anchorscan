package knowledgebase

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/config"
)

type catalogV2 struct {
	Version    int               `json:"version"`
	Source     string            `json:"source"`
	EntryCount int               `json:"entry_count"`
	Entries    []json.RawMessage `json:"entries"`
}

type catalogV2Entry struct {
	ID        string              `json:"id"`
	Title     string              `json:"title"`
	Severity  string              `json:"severity"`
	Status    string              `json:"status"`
	Safety    *catalogV2Safety    `json:"safety"`
	Match     map[string][]string `json:"match"`
	Sections  map[string]string   `json:"sections"`
	Verify    json.RawMessage     `json:"verify"`
	Command   json.RawMessage     `json:"command"`
	Sources   []string            `json:"sources"`
	Generated Generated           `json:"generated"`
}

type catalogV2Safety struct {
	Mode    string    `json:"mode"`
	Effects *[]string `json:"effects"`
	Cleanup *string   `json:"cleanup"`
}

type catalogV2Verify struct {
	Tool     string   `json:"tool"`
	Template string   `json:"template"`
	Target   string   `json:"target"`
	Code     bool     `json:"code"`
	Script   string   `json:"script"`
	Args     string   `json:"args"`
	Flags    []string `json:"flags"`
	Module   string   `json:"module"`
	Action   string   `json:"action"`
}

// LoadJSON reads the published catalog v2 protocol. Callers normally use Load.
func LoadJSON(source []byte) *Catalog {
	if hasDuplicateJSONKeys(source) {
		return unavailable("catalog JSON 无效")
	}
	var decoded catalogV2
	decoder := json.NewDecoder(bytes.NewReader(source))
	if err := decoder.Decode(&decoded); err != nil {
		return unavailable("catalog JSON 无效")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return unavailable("catalog JSON 无效")
	}
	if decoded.Version != 2 || decoded.Source != "handbook-v3" || decoded.EntryCount != len(decoded.Entries) {
		return unavailable("catalog v2 顶层协议无效")
	}

	entries := make([]Entry, 0, len(decoded.Entries))
	diagnostics := make([]Diagnostic, 0)
	ids := make(map[string]bool, len(decoded.Entries))
	for _, raw := range decoded.Entries {
		entry, keep, diagnostic := parseJSONEntry(raw)
		if entry.ID != "" {
			if ids[entry.ID] {
				return unavailable("重复条目 ID: " + entry.ID)
			}
			ids[entry.ID] = true
		}
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		}
		if keep {
			entries = append(entries, entry)
		}
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

var errInvalidJSON = errors.New("invalid JSON")

func hasDuplicateJSONKeys(source []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(source))
	duplicate, err := scanJSONValue(decoder)
	return err != nil || duplicate
}

func scanJSONValue(decoder *json.Decoder) (bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false, nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			key, ok := token.(string)
			if err != nil || !ok {
				return false, errInvalidJSON
			}
			if keys[key] {
				return true, nil
			}
			keys[key] = true
			if duplicate, err := scanJSONValue(decoder); err != nil || duplicate {
				return duplicate, err
			}
		}
	case '[':
		for decoder.More() {
			if duplicate, err := scanJSONValue(decoder); err != nil || duplicate {
				return duplicate, err
			}
		}
	default:
		return false, nil
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') && end != json.Delim(']') {
		return false, errInvalidJSON
	}
	return false, nil
}

func parseJSONEntry(raw json.RawMessage) (Entry, bool, *Diagnostic) {
	var value catalogV2Entry
	if err := json.Unmarshal(raw, &value); err != nil {
		return Entry{}, false, diagnostic(0, "", "catalog 条目无效")
	}
	entry := Entry{
		ID:           value.ID,
		Name:         value.Title,
		Severity:     parseSeverity(value.Severity),
		ReviewStatus: ReviewStatus(value.Status),
		Sources:      append([]string(nil), value.Sources...),
		Generated:    value.Generated,
	}
	if !validJSONDisplayEntry(value, entry) {
		return entry, false, diagnostic(0, entry.ID, "catalog 条目展示字段无效")
	}
	entry.Match = MatchKeys{
		NucleiIDs: append([]string(nil), value.Match["nuclei"]...),
		NSEIDs:    append([]string(nil), value.Match["nse"]...),
		Names:     append([]string(nil), value.Match["manual-review"]...),
		CVEs:      append([]string(nil), value.Match["cve"]...),
	}
	entry.Description = value.Sections["漏洞描述"]
	entry.Remediation = value.Sections["修复建议"]
	entry.Safety = Safety{Mode: SafetyMode(value.Safety.Mode), Effects: append([]string(nil), (*value.Safety.Effects)...)}
	if value.Safety.Cleanup != nil {
		entry.Safety.Cleanup = *value.Safety.Cleanup
	}
	verify, hasVerify, validVerify := decodeJSONVerify(value.Verify)
	command, hasCommand, validCommand := decodeJSONCommand(value.Command)
	if !hasVerify && !hasCommand {
		return entry, true, nil
	}
	if !hasVerify || !hasCommand || !validVerify || !validCommand || !validJSONCommand(command, verify) {
		return entry, true, diagnostic(0, entry.ID, "catalog 命令或 verify 无效")
	}
	entry.Verify = &Verify{Tool: verify.Tool, Template: verify.Template, Target: verify.Target, Code: verify.Code, Script: verify.Script, Args: verify.Args, Flags: append([]string(nil), verify.Flags...), Module: verify.Module, Action: verify.Action}
	switch verify.Tool {
	case "nuclei":
		entry.Commands.Nuclei = command
	case "nmap":
		entry.Commands.NmapNSE = command
	case "msf":
		entry.Commands.Metasploit = command
	}
	return entry, true, nil
}

func decodeJSONVerify(raw json.RawMessage) (*catalogV2Verify, bool, bool) {
	if len(raw) == 0 {
		return nil, false, true
	}
	var verify catalogV2Verify
	if err := json.Unmarshal(raw, &verify); err != nil || string(raw) == "null" {
		return nil, true, false
	}
	return &verify, true, true
}

func decodeJSONCommand(raw json.RawMessage) (string, bool, bool) {
	if len(raw) == 0 {
		return "", false, true
	}
	var command string
	if err := json.Unmarshal(raw, &command); err != nil || string(raw) == "null" {
		return "", true, false
	}
	return command, true, true
}

func validJSONDisplayEntry(value catalogV2Entry, entry Entry) bool {
	if strings.TrimSpace(entry.ID) == "" || strings.TrimSpace(entry.Name) == "" || entry.Severity == "" || !validReviewStatus(entry.ReviewStatus) || value.Safety == nil || !validSafety(value.Safety) {
		return false
	}
	for _, key := range []string{"nuclei", "nse", "manual-review", "cve"} {
		if _, ok := value.Match[key]; !ok {
			return false
		}
	}
	return strings.TrimSpace(value.Sections["漏洞描述"]) != "" && strings.TrimSpace(value.Sections["修复建议"]) != ""
}

func validReviewStatus(status ReviewStatus) bool {
	return status == ReviewStatusStable || status == ReviewStatusNeedsReview
}

func validSafety(safety *catalogV2Safety) bool {
	if safety == nil || safety.Effects == nil || (safety.Mode != string(SafetySafe) && safety.Mode != string(SafetyOptional) && safety.Mode != string(SafetyManualGated)) || hasDuplicate(*safety.Effects) {
		return false
	}
	effects := *safety.Effects
	hasFileEffect := false
	for _, effect := range effects {
		switch effect {
		case "authentication-attempt", "file-read", "test-file-create", "test-file-delete", "controlled-command", "oast":
		default:
			return false
		}
		hasFileEffect = hasFileEffect || effect == "test-file-create" || effect == "test-file-delete"
	}
	if safety.Mode == string(SafetySafe) && len(effects) != 0 {
		return false
	}
	if safety.Mode == string(SafetyOptional) && (len(effects) != 1 || effects[0] != "authentication-attempt") {
		return false
	}
	if safety.Mode == string(SafetyManualGated) && len(effects) == 0 {
		return false
	}
	return !hasFileEffect || safety.Cleanup != nil && strings.TrimSpace(*safety.Cleanup) != ""
}

func hasDuplicate(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func validJSONCommand(command string, verify *catalogV2Verify) bool {
	switch verify.Tool {
	case "nuclei":
		return validNucleiJSON(command, verify)
	case "nmap":
		return !verify.Code && validNmapJSON(command, verify)
	case "msf":
		return !verify.Code && validMSFJSON(command, verify)
	default:
		return false
	}
}

func validNmapJSON(command string, verify *catalogV2Verify) bool {
	args, err := config.SplitArgs(command)
	if err != nil || verify.Script == "" || hasDuplicate(verify.Flags) || len(verify.Flags) > 1 || len(args) < 5 || args[0] != "nmap" {
		return false
	}
	index := 1
	for _, flag := range verify.Flags {
		if flag != "-sU" || index >= len(args) || args[index] != flag {
			return false
		}
		index++
	}
	if len(args) < index+5 || args[index] != "-p" || args[index+1] != "{{port}}" || args[index+2] != "--script" || args[index+3] != verify.Script {
		return false
	}
	index += 4
	if verify.Args != "" {
		if len(args) < index+3 || args[index] != "--script-args" || args[index+1] != verify.Args {
			return false
		}
		index += 2
	}
	return len(args) == index+1 && args[index] == "{{host}}"
}

func validMSFJSON(command string, verify *catalogV2Verify) bool {
	if verify.Module == "" || (verify.Action != "run" && verify.Action != "check") || !validMSF(command) {
		return false
	}
	lines := strings.FieldsFunc(command, func(r rune) bool { return r == '\n' || r == '\r' })
	return strings.TrimSpace(strings.TrimPrefix(lines[0], "use ")) == verify.Module && strings.TrimSpace(lines[3]) == verify.Action
}

func validNucleiJSON(command string, verify *catalogV2Verify) bool {
	args, err := config.SplitArgs(command)
	if err != nil || len(args) < 5 || args[0] != "nuclei" || verify.Template == "" {
		return false
	}
	index := 1
	hasCode := args[index] == "-code"
	if hasCode != verify.Code {
		return false
	}
	if hasCode {
		index++
	}
	for _, arg := range args[index:] {
		if arg == "-code" {
			return false
		}
	}
	target := "{{host}}:{{port}}"
	if verify.Target == "url" {
		target = "{{url}}"
	} else if verify.Target != "host:port" {
		return false
	}
	return len(args) == index+4 && args[index] == "-t" && args[index+1] == verify.Template && args[index+2] == "-u" && args[index+3] == target
}
