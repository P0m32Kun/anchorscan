package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

func TestLoadParsesToolPathsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
tools:
  rustscan: /usr/local/bin/rustscan
  nmap: /usr/local/bin/nmap
  httpx: /usr/local/bin/httpx
  nuclei: /usr/local/bin/nuclei
scan:
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var paths ToolPaths = cfg.Tools
	if paths.Rustscan != "/usr/local/bin/rustscan" || paths.Nmap != "/usr/local/bin/nmap" {
		t.Fatalf("unexpected tool paths: %#v", paths)
	}
	if cfg.Tools.Rustscan != "/usr/local/bin/rustscan" {
		t.Fatalf("unexpected rustscan path: %q", cfg.Tools.Rustscan)
	}
	if cfg.Scan.Ports != "top1000" {
		t.Fatalf("unexpected default ports: %q", cfg.Scan.Ports)
	}
}

func TestLoadValidatesToolTimeouts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("timeouts:\n  rustscan: 2s\n  nmap: 0\n  httpx: -1s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted a negative timeout")
	}
	if err := os.WriteFile(path, []byte("timeouts:\n  rustscan: 2s\n  nmap: 0\n  httpx: invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted an invalid timeout")
	}
	if err := os.WriteFile(path, []byte("timeouts:\n  rustscan: 2s\n  nmap: 0\n  httpx: 0\n  nse: 150ms\n  nuclei: 1m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	timeouts, err := cfg.Timeouts.Durations()
	if err != nil || timeouts.Rustscan.String() != "2s" || timeouts.NSE.String() != "150ms" || timeouts.Nmap != 0 {
		t.Fatalf("timeouts = %#v, %v", timeouts, err)
	}
}

func TestLoadTagRulesParsesSnakeCaseFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service-tags.yaml")
	content := []byte(`
- name: tomcat
  service: ["http"]
  product: ["apache tomcat"]
  tech: ["tomcat"]
  nuclei_tags: ["tomcat", "apache-tomcat"]
  target: "url"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadTagRules(path)
	if err != nil {
		t.Fatalf("LoadTagRules returned error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("unexpected rules: %#v", rules)
	}
	if got := rules[0].NucleiTags; len(got) != 2 || got[0] != "tomcat" || rules[0].Target != "url" {
		t.Fatalf("unexpected parsed rule: %#v", rules[0])
	}
}

func TestLoadParsesProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
tools:
  rustscan: /opt/rustscan
  nmap: /opt/nmap
  httpx: /opt/httpx
  nuclei: /opt/nuclei
scan:
  ports: top1000
  profile: slow
profiles:
  slow:
    host_workers: 1
    rustscan_args: ["--batch-size", "100"]
    nmap_args: ["-T2", "--max-retries", "3"]
    httpx_args: ["-rate-limit", "20"]
    nuclei_args: ["-rate-limit", "10"]
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Scan.Profile != "slow" {
		t.Fatalf("profile mismatch: got %q", cfg.Scan.Profile)
	}
	profile := cfg.Profiles["slow"]
	if profile.HostWorkers != 1 {
		t.Fatalf("host workers mismatch: got %d", profile.HostWorkers)
	}
	if !reflect.DeepEqual(profile.Nmap, []string{"-T2", "--max-retries", "3"}) {
		t.Fatalf("nmap args mismatch: %#v", profile.Nmap)
	}
}

func TestLoadDefaultsProfileToNormal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\nscan:\n  ports: top1000\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Scan.Profile != "normal" {
		t.Fatalf("profile mismatch: got %q want normal", cfg.Scan.Profile)
	}
}

func TestLoadRulesForConfigFallsBackToRootConfig(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "nse.yaml"), []byte("http:\n  - http-title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "service-tags.yaml"), []byte("- name: http\n  service: [http]\n  nuclei_tags: [http]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nseRules, err := LoadNSERulesForConfig(filepath.Join(root, "custom", "default.yaml"))
	if err != nil {
		t.Fatalf("LoadNSERulesForConfig returned error: %v", err)
	}
	if got := nseRules["http"]; len(got) != 1 || got[0] != "http-title" {
		t.Fatalf("unexpected nse rules: %#v", nseRules)
	}

	tagRules, err := LoadTagRulesForConfig(filepath.Join(root, "custom", "default.yaml"))
	if err != nil {
		t.Fatalf("LoadTagRulesForConfig returned error: %v", err)
	}
	if len(tagRules) != 1 || tagRules[0].Name != "http" {
		t.Fatalf("unexpected tag rules: %#v", tagRules)
	}
}

func TestDefaultRuleFilesProvideDualEngineCoverage(t *testing.T) {
	nseRules, err := LoadNSERules(filepath.Join("..", "..", "config", "nse.yaml"))
	if err != nil {
		t.Fatalf("LoadNSERules returned error: %v", err)
	}
	// 所有有 NSE 覆盖的服务都应配置脚本（键为归一化后的服务名）。
	for _, service := range []string{
		"ssh", "ftp", "telnet", "smtp", "ldap", "domain", "snmp",
		"rdp", "vnc", "redis", "mysql", "postgresql", "ms-sql",
		"mongodb", "memcached", "amqp", "mqtt", "smb", "nfs",
		"rpc", "rsync", "docker",
	} {
		if len(nseRules[service]) == 0 {
			t.Fatalf("expected NSE rules for %s: %#v", service, nseRules)
		}
	}
	// 仅 nuclei 覆盖、无 NSE 脚本的服务不应出现在 nse.yaml。
	for _, service := range []string{"elasticsearch", "kafka", "kubernetes", "winrm"} {
		if len(nseRules[service]) != 0 {
			t.Fatalf("did not expect NSE rules for nuclei-only service %s: %#v", service, nseRules)
		}
	}

	tagRules, err := LoadTagRules(filepath.Join("..", "..", "config", "service-tags.yaml"))
	if err != nil {
		t.Fatalf("LoadTagRules returned error: %v", err)
	}
	ruleByName := map[string]vuln.TagRule{}
	for _, rule := range tagRules {
		ruleByName[rule.Name] = rule
	}
	// 所有服务（含非 Web）都应有 nuclei tag 规则，实现双引擎覆盖矩阵。
	for _, service := range []string{
		"ssh", "ftp", "telnet", "smtp", "ldap", "dns", "snmp",
		"rdp", "vnc", "winrm", "redis", "mysql", "postgres",
		"mssql", "mongodb", "memcached", "elasticsearch",
		"rabbitmq", "kafka", "mqtt", "smb", "nfs", "rpc",
		"rsync", "docker", "kubernetes",
		"tomcat", "nginx", "apache", "iis", "wordpress", "joomla",
	} {
		if _, ok := ruleByName[service]; !ok {
			t.Fatalf("expected nuclei tag rule for %s: %#v", service, tagRules)
		}
	}
	// 弱口令检测契约：nuclei_tags 不携带全局 default-login 标签。每个服务的
	// default-login 模板本身都带服务名 tag（如 redis-default-logins 带 redis），
	// 所以 -tags <service> 已能覆盖该服务的弱口令检测；追加全局 default-login
	// 会加载 360 个无关爆破模板，既慢又易触发账户锁定。SSH 用 exclude_tags 单独排除。
	for _, rule := range tagRules {
		if len(rule.NucleiTags) == 0 {
			t.Fatalf("rule %s has no nuclei_tags: %#v", rule.Name, rule)
		}
		for _, tag := range rule.NucleiTags {
			if tag == "default-login" {
				t.Fatalf("rule %s must not include global default-login tag: %#v", rule.Name, rule)
			}
		}
		if rule.Target != "url" && rule.Target != "hostport" {
			t.Fatalf("rule %s has unexpected target %q (want url or hostport)", rule.Name, rule.Target)
		}
	}
}
