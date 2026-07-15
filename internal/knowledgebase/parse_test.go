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

func TestLoadParsesThreeSections(t *testing.T) {
	configPath, handbookPath := writeHandbook(t, handbook)
	catalog := Load(configPath, filepath.Base(handbookPath))
	if catalog.Status() != StatusReady {
		t.Fatalf("Status() = %q, diagnostics = %#v", catalog.Status(), catalog.Diagnostics())
	}
	entry, ok := catalog.Entry("smb-signing")
	if !ok || entry.Description != "描述。" || entry.Remediation != "启用签名。" || entry.Commands.Nuclei == "" {
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
