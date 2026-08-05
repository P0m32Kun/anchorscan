package knowledgebase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const handbook = `<!-- anchorscan-catalog
version: 1
-->

### SMB 签名未启用（中危）

<!-- anchorscan-entry
id: smb-signing
aliases: [SMB signing]
match:
  nuclei: [smb-signing]
  nse: []
  manual-review: []
  cve: [CVE-2024-0001]
-->

#### 漏洞描述

描述。

#### 验证命令

##### Nuclei

` + "```bash" + `
nuclei -t network/smb.yaml -u {{host}}:{{port}}
` + "```" + `

#### 修复建议

启用签名。
`

func TestLoadJSONV2PreservesSafetyAndAuditFields(t *testing.T) {
	configPath, catalogPath := writeCatalog(t, fixture(t, "catalog-v2.json"))
	catalog := Load(configPath, filepath.Base(catalogPath))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusReady || !ok {
		t.Fatalf("Status() = %q, entry = %#v, ok = %t", catalog.Status(), entry, ok)
	}
	if entry.ReviewStatus != ReviewStatusNeedsReview || entry.Safety.Mode != SafetyOptional || entry.Safety.Cleanup != "停止认证尝试" || len(entry.Sources) != 1 || entry.Generated.By != "agent:test" {
		t.Fatalf("Entry() lost catalog v2 safety or audit data: %#v", entry)
	}
	if entry.Verify == nil || entry.Verify.Template != "network/smb.yaml" {
		t.Fatalf("Entry() lost verify data: %#v", entry.Verify)
	}
	if entry.Commands.Nuclei != "nuclei -t network/smb.yaml -u {{host}}:{{port}}" {
		t.Fatalf("Entry() command = %q", entry.Commands.Nuclei)
	}
}

func TestLoadJSONRejectsTrailingDocument(t *testing.T) {
	for _, trailing := range []string{"\n{}", "\n]", "\n}"} {
		catalog := LoadJSON([]byte(fixture(t, "catalog-v2.json") + trailing))
		if catalog.Status() != StatusUnavailable {
			t.Fatalf("Status() = %q, want %q for trailing %q", catalog.Status(), StatusUnavailable, trailing)
		}
	}
}

func TestLoadJSONFixtureSupportsCodeAndNoVerify(t *testing.T) {
	configPath, catalogPath := writeCatalog(t, fixture(t, "catalog-v2.json"))
	catalog := Load(configPath, filepath.Base(catalogPath))
	code, codeOK := catalog.Entry("nuclei-code")
	noVerify, noVerifyOK := catalog.Entry("no-verify")
	manual, manualOK := catalog.Entry("test-file-delete")
	if catalog.Status() != StatusReady || !codeOK || code.Verify == nil || !code.Verify.Code || !strings.Contains(code.Commands.Nuclei, "nuclei -code") || !noVerifyOK || noVerify.Verify != nil || noVerify.Commands != (Commands{}) || !manualOK || manual.Safety.Cleanup == "" {
		t.Fatalf("catalog v2 fixture was not preserved: status=%q code=%#v no-verify=%#v manual=%#v", catalog.Status(), code, noVerify, manual)
	}
}

func TestLoadJSONRealCatalogV2AcceptsAllEntriesAndCodeCommands(t *testing.T) {
	catalog := LoadJSON([]byte(fixture(t, "catalog-v2-real.json")))
	entries := catalog.Search("")
	if catalog.Status() != StatusReady || len(catalog.Diagnostics()) != 0 || len(entries) != 188 {
		t.Fatalf("real catalog status=%q diagnostics=%#v entries=%d", catalog.Status(), catalog.Diagnostics(), len(entries))
	}

	commands := 0
	for _, entry := range entries {
		if entry.Commands != (Commands{}) {
			commands++
		}
	}
	if commands != 145 {
		t.Fatalf("real catalog command count=%d, want 145", commands)
	}

	want := map[string]string{
		"cve-2026-24061": "nuclei -code -t code/cves/2026/CVE-2026-24061.yaml -u {{host}}:{{port}}",
		"cve-2017-7529":  "nuclei -code -t RBKD-templates/http/vulnerabilities/cve-2017-7529.yaml -u {{url}}",
	}
	for id, command := range want {
		entry, ok := catalog.Entry(id)
		if !ok || entry.Verify == nil || !entry.Verify.Code || entry.Commands.Nuclei != command {
			t.Fatalf("real code entry %q = %#v, found=%t", id, entry, ok)
		}
	}
}

func TestLoadJSONRejectsInvalidNucleiCode(t *testing.T) {
	fixtureContent := fixture(t, "catalog-v2.json")
	codeVerify := `"verify": {"tool": "nuclei", "template": "code/cves/2026/CVE-2026-24061.yaml", "target": "host:port", "code": true}`
	codeCommand := `"command": "nuclei -code -t code/cves/2026/CVE-2026-24061.yaml -u {{host}}:{{port}}"`
	cases := map[string]string{
		"repeated": strings.Replace(fixtureContent, codeCommand, `"command": "nuclei -code -code -t code/cves/2026/CVE-2026-24061.yaml -u {{host}}:{{port}}"`, 1),
		"misplaced": strings.Replace(
			strings.Replace(fixtureContent, codeVerify, `"verify": {"tool": "nuclei", "template": "-code", "target": "host:port"}`, 1),
			codeCommand, `"command": "nuclei -t -code -u {{host}}:{{port}}"`, 1),
		"verify requires code":  strings.Replace(fixtureContent, codeCommand, `"command": "nuclei -t code/cves/2026/CVE-2026-24061.yaml -u {{host}}:{{port}}"`, 1),
		"command requires code": strings.Replace(fixtureContent, codeVerify, `"verify": {"tool": "nuclei", "template": "code/cves/2026/CVE-2026-24061.yaml", "target": "host:port"}`, 1),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			catalog := LoadJSON([]byte(content))
			entry, ok := catalog.Entry("nuclei-code")
			if catalog.Status() != StatusDegraded || !ok || entry.Commands != (Commands{}) || entry.Verify != nil {
				t.Fatalf("status=%q entry=%#v found=%t", catalog.Status(), entry, ok)
			}
		})
	}
}

func TestLoadJSONSkipsInvalidSafetyAndClearsInvalidCommand(t *testing.T) {
	fixtureContent := fixture(t, "catalog-v2.json")
	invalidSafety := strings.Replace(fixtureContent, `"safety": {"mode": "optional", "effects": ["authentication-attempt"], "cleanup": "停止认证尝试"}`, `"safety": null`, 1)
	catalog := LoadJSON([]byte(invalidSafety))
	if catalog.Status() != StatusDegraded {
		t.Fatalf("invalid safety status = %q, want %q", catalog.Status(), StatusDegraded)
	}
	if _, ok := catalog.Entry("smb-signing"); ok {
		t.Fatal("invalid safety entry was retained")
	}

	invalidCommand := strings.Replace(fixtureContent, "nuclei -t network/smb.yaml", "nuclei --invalid network/smb.yaml", 1)
	catalog = LoadJSON([]byte(invalidCommand))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusDegraded || !ok || entry.Commands != (Commands{}) {
		t.Fatalf("invalid command status=%q entry=%#v ok=%t", catalog.Status(), entry, ok)
	}
}

func TestLoadJSONRejectsSafeSafetyWithoutEffects(t *testing.T) {
	content := strings.Replace(fixture(t, "catalog-v2.json"), `"safety": {"mode": "safe", "effects": []}`, `"safety": {"mode": "safe"}`, 1)
	catalog := LoadJSON([]byte(content))
	if catalog.Status() != StatusDegraded {
		t.Fatalf("Status() = %q, want %q", catalog.Status(), StatusDegraded)
	}
	if _, ok := catalog.Entry("nuclei-code"); ok {
		t.Fatal("safe entry without effects was retained")
	}
}

func TestLoadJSONRejectsManualGatedSafetyWithoutEffects(t *testing.T) {
	content := strings.Replace(fixture(t, "catalog-v2.json"), `"safety": {"mode": "manual-gated", "effects": ["file-read"]}`, `"safety": {"mode": "manual-gated", "effects": []}`, 1)
	catalog := LoadJSON([]byte(content))
	if catalog.Status() != StatusDegraded {
		t.Fatalf("Status() = %q, want %q", catalog.Status(), StatusDegraded)
	}
	if _, ok := catalog.Entry("no-verify"); ok {
		t.Fatal("manual-gated entry without effects was retained")
	}
}

func TestLoadJSONKeepsDisplayEntryForMalformedCommandOrVerify(t *testing.T) {
	fixtureContent := fixture(t, "catalog-v2.json")
	for _, content := range []string{
		strings.Replace(fixtureContent, `"command": "nuclei -t network/smb.yaml -u {{host}}:{{port}}"`, `"command": 123`, 1),
		strings.Replace(fixtureContent, `"verify": {"tool": "nuclei", "template": "network/smb.yaml", "target": "host:port"}`, `"verify": []`, 1),
	} {
		catalog := LoadJSON([]byte(content))
		entry, ok := catalog.Entry("smb-signing")
		if catalog.Status() != StatusDegraded || !ok || entry.Commands != (Commands{}) || entry.Verify != nil {
			t.Fatalf("malformed command/verify status=%q entry=%#v ok=%t", catalog.Status(), entry, ok)
		}
	}
}

func TestLoadJSONAcceptsCanonicalNmapCommand(t *testing.T) {
	content := fixture(t, "catalog-v2.json")
	content = strings.Replace(content, `"verify": {"tool": "nuclei", "template": "network/smb.yaml", "target": "host:port"}`, `"verify": {"tool": "nmap", "script": "smb-signing", "flags": ["-sU"]}`, 1)
	content = strings.Replace(content, `"command": "nuclei -t network/smb.yaml -u {{host}}:{{port}}"`, `"command": "nmap -sU -p {{port}} --script smb-signing {{host}}"`, 1)
	catalog := LoadJSON([]byte(content))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusReady || !ok || entry.Commands.NmapNSE == "" || entry.Verify == nil || entry.Verify.Script != "smb-signing" {
		t.Fatalf("canonical Nmap command status=%q entry=%#v ok=%t", catalog.Status(), entry, ok)
	}
}

func TestLoadJSONRejectsNmapCommandWithoutScript(t *testing.T) {
	content := fixture(t, "catalog-v2.json")
	content = strings.Replace(content, `"verify": {"tool": "nuclei", "template": "network/smb.yaml", "target": "host:port"}`, `"verify": {"tool": "nmap", "script": "smb-signing"}`, 1)
	content = strings.Replace(content, `"command": "nuclei -t network/smb.yaml -u {{host}}:{{port}}"`, `"command": "nmap -sU -p {{port}} {{host}}"`, 1)
	catalog := LoadJSON([]byte(content))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusDegraded || !ok || entry.Commands != (Commands{}) {
		t.Fatalf("invalid Nmap command status=%q entry=%#v ok=%t", catalog.Status(), entry, ok)
	}
}

func TestLoadJSONRejectsNmapAndMSFCode(t *testing.T) {
	fixtureContent := fixture(t, "catalog-v2.json")
	verify := `"verify": {"tool": "nuclei", "template": "network/smb.yaml", "target": "host:port"}`
	command := `"command": "nuclei -t network/smb.yaml -u {{host}}:{{port}}"`
	cases := map[string]string{
		"nmap": strings.Replace(
			strings.Replace(fixtureContent, verify, `"verify": {"tool": "nmap", "script": "smb-signing", "code": true}`, 1),
			command, `"command": "nmap -p {{port}} --script smb-signing {{host}}"`, 1),
		"msf": strings.Replace(
			strings.Replace(fixtureContent, verify, `"verify": {"tool": "msf", "module": "auxiliary/scanner/ssh/ssh_version", "action": "run", "code": true}`, 1),
			command, `"command": "use auxiliary/scanner/ssh/ssh_version\nset RHOSTS {{host}}\nset RPORT {{port}}\nrun"`, 1),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			catalog := LoadJSON([]byte(content))
			entry, ok := catalog.Entry("smb-signing")
			if catalog.Status() != StatusDegraded || !ok || entry.Commands != (Commands{}) || entry.Verify != nil {
				t.Fatalf("status=%q entry=%#v found=%t", catalog.Status(), entry, ok)
			}
		})
	}
}

func TestLoadJSONClearsCommandThatDoesNotMatchVerify(t *testing.T) {
	content := strings.Replace(fixture(t, "catalog-v2.json"), `"command": "nuclei -t network/smb.yaml -u {{host}}:{{port}}"`, `"command": "nuclei -t network/other.yaml -u {{host}}:{{port}}"`, 1)
	catalog := LoadJSON([]byte(content))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusDegraded || !ok || entry.Commands != (Commands{}) || entry.Verify != nil {
		t.Fatalf("verify mismatch status=%q entry=%#v ok=%t", catalog.Status(), entry, ok)
	}
}

func TestLoadLegacyFixtureAllowsExtensionsAndFailsClosed(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, fixture(t, "handbook-v1.md"))
	catalog := Load(configPath, filepath.Base(handbookPath))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusReady || !ok || entry.ReviewStatus != ReviewStatusLegacyUnknown || entry.Safety.Mode != SafetyLegacyUnknown {
		t.Fatalf("legacy entry status=%q entry=%#v ok=%t", catalog.Status(), entry, ok)
	}
}

func TestCatalogV2FixtureMatchesMarkdownProjection(t *testing.T) {
	jsonEntry, jsonOK := LoadJSON([]byte(fixture(t, "catalog-v2.json"))).Entry("smb-signing")
	configPath, handbookPath := writeHandbook(t, fixture(t, "handbook-v2.md"))
	markdownEntry, markdownOK := Load(configPath, filepath.Base(handbookPath)).Entry("smb-signing")
	if !jsonOK || !markdownOK || jsonEntry.Name != markdownEntry.Name || jsonEntry.Severity != markdownEntry.Severity || jsonEntry.Description != markdownEntry.Description || jsonEntry.Remediation != markdownEntry.Remediation || jsonEntry.Commands.Nuclei != markdownEntry.Commands.Nuclei || !slicesEqual(jsonEntry.Match.NucleiIDs, markdownEntry.Match.NucleiIDs) || !slicesEqual(jsonEntry.Match.CVEs, markdownEntry.Match.CVEs) {
		t.Fatalf("JSON and Markdown projections differ: json=%#v markdown=%#v", jsonEntry, markdownEntry)
	}
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestLoadJSONRejectsDuplicateObjectKeys(t *testing.T) {
	content := strings.Replace(fixture(t, "catalog-v2.json"), `"source": "handbook-v3",`, `"source": "handbook-v3", "source": "handbook-v3",`, 1)
	if got := LoadJSON([]byte(content)).Status(); got != StatusUnavailable {
		t.Fatalf("Status() = %q, want %q", got, StatusUnavailable)
	}
}

func TestLoadJSONRejectsInvalidCatalogProtocol(t *testing.T) {
	fixtureContent := fixture(t, "catalog-v2.json")
	cases := []string{
		"{",
		strings.Replace(fixtureContent, `"handbook-v3"`, `"handbook-v2"`, 1),
		strings.Replace(fixtureContent, `"entry_count": 4`, `"entry_count": 5`, 1),
		strings.Replace(fixtureContent, `"id": "nuclei-code"`, `"id": "smb-signing"`, 1),
		`{"version": 2, "source": "handbook-v3", "entry_count": 1, "entries": [{}]}`,
	}
	for _, content := range cases {
		if got := LoadJSON([]byte(content)).Status(); got != StatusUnavailable {
			t.Errorf("Status() = %q, want %q for %s", got, StatusUnavailable, content[:min(len(content), 30)])
		}
	}
}

func TestLoadParsesThreeSections(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, handbook)
	catalog := Load(configPath, filepath.Base(handbookPath))
	if catalog.Status() != StatusReady {
		t.Fatalf("Status() = %q, diagnostics = %#v", catalog.Status(), catalog.Diagnostics())
	}
	entry, ok := catalog.Entry("smb-signing")
	if !ok || entry.Description != "描述。" || entry.Remediation != "启用签名。" || entry.Commands.Nuclei == "" || entry.ReviewStatus != ReviewStatusLegacyUnknown || entry.Safety.Mode != SafetyLegacyUnknown {
		t.Fatalf("Entry() = %#v, %t", entry, ok)
	}
}

func TestLoadDropsTrailingEntrySeparator(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, handbook+"\n---\n")
	entry, ok := Load(configPath, filepath.Base(handbookPath)).Entry("smb-signing")
	if !ok || entry.Remediation != "启用签名。" {
		t.Fatalf("Entry() = %#v, %t", entry, ok)
	}
}

func TestLoadPreservesSeparatorInDescription(t *testing.T) {
	withSeparator := strings.Replace(handbook, "描述。\n\n#### 验证命令", "描述。\n\n---\n\n#### 验证命令", 1)
	configPath, handbookPath := writeHandbook(t, withSeparator)
	entry, ok := Load(configPath, filepath.Base(handbookPath)).Entry("smb-signing")
	if !ok || entry.Description != "描述。\n\n---" {
		t.Fatalf("Entry() = %#v, %t", entry, ok)
	}
}

func TestLoadAcceptsEmptyVerificationCommands(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, strings.Replace(handbook, "##### Nuclei\n\n```bash\nnuclei -t network/smb.yaml -u {{host}}:{{port}}\n```\n\n", "", 1))
	if got := Load(configPath, filepath.Base(handbookPath)).Status(); got != StatusReady {
		t.Fatalf("Status() = %q, want %q", got, StatusReady)
	}
}

func TestLoadReportsUnavailableForMissingCatalogVersion(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, "### SMB 签名未启用（中危）\n")
	if got := Load(configPath, filepath.Base(handbookPath)).Status(); got != StatusUnavailable {
		t.Fatalf("Status() = %q, want %q", got, StatusUnavailable)
	}
}

func TestLoadDegradesInvalidOptionalCommand(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, handbook+"\n##### Nmap NSE\n\n```bash\nnmap -oX out.xml -p {{port}} {{host}}\n```\n")
	catalog := Load(configPath, filepath.Base(handbookPath))
	entry, ok := catalog.Entry("smb-signing")
	if catalog.Status() != StatusDegraded || !ok || entry.Commands.Nuclei == "" {
		t.Fatalf("status = %q, entry = %#v, ok = %t", catalog.Status(), entry, ok)
	}
}

func TestLoadRejectsTextBetweenTitleAndMetadata(t *testing.T) {
	invalid := strings.Replace(handbook, "<!-- anchorscan-entry", "这不是元数据。\n\n<!-- anchorscan-entry", 1)
	configPath, handbookPath := writeHandbook(t, invalid)
	if got := Load(configPath, filepath.Base(handbookPath)).Status(); got != StatusUnavailable {
		t.Fatalf("Status() = %q, want %q", got, StatusUnavailable)
	}
}

func TestLoadRejectsOutOfOrderSections(t *testing.T) {
	invalid := strings.Replace(handbook, "#### 漏洞描述\n\n描述。\n\n#### 验证命令", "#### 验证命令\n\n#### 漏洞描述\n\n描述。", 1)
	configPath, handbookPath := writeHandbook(t, invalid)
	if got := Load(configPath, filepath.Base(handbookPath)).Status(); got != StatusUnavailable {
		t.Fatalf("Status() = %q, want %q", got, StatusUnavailable)
	}
}

func TestLoadKeepsValidNmapNSECommand(t *testing.T) {
	withNmap := strings.Replace(handbook, "##### Nuclei\n\n```bash\nnuclei -t network/smb.yaml -u {{host}}:{{port}}\n```", "##### Nmap NSE\n\n```bash\nnmap --script smb2-security-mode -p {{port}} {{host}}\n```", 1)
	configPath, handbookPath := writeHandbook(t, withNmap)
	entry, ok := Load(configPath, filepath.Base(handbookPath)).Entry("smb-signing")
	if !ok || entry.Commands.NmapNSE != "nmap --script smb2-security-mode -p {{port}} {{host}}" {
		t.Fatalf("Entry() = %#v, %t", entry, ok)
	}
}

func TestLoadKeepsValidMSFCommand(t *testing.T) {
	withMSF := strings.Replace(handbook, "##### Nuclei\n\n```bash\nnuclei -t network/smb.yaml -u {{host}}:{{port}}\n```", "##### MSF\n\n```text\nuse auxiliary/scanner/ssh/ssh_version\nset RHOSTS {{host}}\nset RPORT {{port}}\nrun\n```", 1)
	configPath, handbookPath := writeHandbook(t, withMSF)
	entry, ok := Load(configPath, filepath.Base(handbookPath)).Entry("smb-signing")
	if !ok || !strings.Contains(entry.Commands.Metasploit, "set RPORT {{port}}") {
		t.Fatalf("Entry() = %#v, %t", entry, ok)
	}
}

func TestLoadRejectsPlaceholderOutsideNucleiTarget(t *testing.T) {
	invalid := strings.Replace(handbook, "network/smb.yaml", "network/{{host}}.yaml", 1)
	configPath, handbookPath := writeHandbook(t, invalid)
	if got := Load(configPath, filepath.Base(handbookPath)).Status(); got != StatusDegraded {
		t.Fatalf("Status() = %q, want %q", got, StatusDegraded)
	}
}

func TestLoadRejectsYAMLAnchor(t *testing.T) {
	invalid := strings.Replace(handbook, "aliases: [SMB signing]", "aliases: &aliases [SMB signing]", 1)
	configPath, handbookPath := writeHandbook(t, invalid)
	if got := Load(configPath, filepath.Base(handbookPath)).Status(); got != StatusUnavailable {
		t.Fatalf("Status() = %q, want %q", got, StatusUnavailable)
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func writeCatalog(t *testing.T, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	catalogPath := filepath.Join(dir, "catalog.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, catalogPath
}

func writeHandbook(t *testing.T, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	handbookPath := filepath.Join(dir, "handbook.md")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handbookPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, handbookPath
}
