# AnchorScan V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that orchestrates `rustscan`, `nmap`, `httpx`, and `nuclei` using user-supplied tool paths, then stores scan evidence and renders JSON/HTML reports.

**Architecture:** The CLI coordinates a fixed pipeline: target parsing, port selection, `rustscan` port discovery, `nmap -sV` fingerprinting, service normalization, optional `httpx` enrichment for web services, fingerprint-driven NSE/Nuclei selection, persistence to SQLite, and report generation. External binaries stay outside the repo package boundary; the Go code only loads their paths from config and shells out through a single runner abstraction.

**Tech Stack:** Go, Cobra, YAML v3, `modernc.org/sqlite`, `encoding/xml`, `html/template`, external tools configured by absolute or relative path

## Global Constraints

- V1 is Go-first and follows TDD: no production code without a failing test first.
- External tools are not bundled into the app yet; paths come from configuration.
- The scan pipeline is fixed: `rustscan` -> `nmap -sV` -> fingerprint classification -> `httpx` for web -> NSE/Nuclei by fingerprint -> persistence -> reports.
- Port selection is only `--ports <csv>`, `top100`, `top1000`, or `full`; no extra scan modes.
- Nuclei runs by service-specific tags, not by broad template directories.
- NSE is a vulnerability/enrichment phase after fingerprinting, with a small default config and user-extensible mappings.
- Unknown services are kept in results; they are not dropped or automatically treated as web unless the explicit unknown-web probe path is added later.

---

## Planned File Structure

- Create: `go.mod`
- Create: `cmd/anchorscan/main.go`
- Create: `internal/app/scan.go`
- Create: `internal/app/report.go`
- Create: `internal/config/config.go`
- Create: `internal/target/parse.go`
- Create: `internal/ports/resolve.go`
- Create: `internal/tools/runner.go`
- Create: `internal/tools/rustscan.go`
- Create: `internal/tools/nmap.go`
- Create: `internal/tools/httpx.go`
- Create: `internal/tools/nuclei.go`
- Create: `internal/fingerprint/nmap_xml.go`
- Create: `internal/fingerprint/normalize.go`
- Create: `internal/fingerprint/classify.go`
- Create: `internal/vuln/nse.go`
- Create: `internal/vuln/nuclei_tags.go`
- Create: `internal/store/sqlite.go`
- Create: `internal/report/json.go`
- Create: `internal/report/html.go`
- Create: `internal/report/templates/report.html`
- Create: `tests` as package-local `_test.go` files under the matching `internal/...` packages
- Create: `config/default.yaml`
- Create: `config/ports-top100.txt`
- Create: `config/ports-top1000.txt`
- Create: `config/service-aliases.yaml`
- Create: `config/service-tags.yaml`
- Create: `config/nse.yaml`

### Task 1: Bootstrap The CLI Skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/anchorscan/main.go`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: none
- Produces: `type Config struct`, `func Load(path string) (Config, error)`, `func Execute() error`

- [ ] **Step 1: Write the failing config test**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
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
  ports: top100
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Tools.Rustscan != "/usr/local/bin/rustscan" {
		t.Fatalf("unexpected rustscan path: %q", cfg.Tools.Rustscan)
	}
	if cfg.Scan.Ports != "top100" {
		t.Fatalf("unexpected default ports: %q", cfg.Scan.Ports)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoadParsesToolPathsAndDefaults -v`
Expected: FAIL with `undefined: Load`

- [ ] **Step 3: Write minimal implementation**

```go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Tools struct {
		Rustscan string `yaml:"rustscan"`
		Nmap     string `yaml:"nmap"`
		Httpx    string `yaml:"httpx"`
		Nuclei   string `yaml:"nuclei"`
	} `yaml:"tools"`
	Scan struct {
		Ports string `yaml:"ports"`
	} `yaml:"scan"`
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Scan.Ports == "" {
		cfg.Scan.Ports = "top100"
	}
	return cfg, nil
}
```

- [ ] **Step 4: Add the minimal CLI entrypoint**

```go
package main

import "os"

func main() {
	if err := Execute(); err != nil {
		os.Exit(1)
	}
}
```

```go
package main

func Execute() error {
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config -run TestLoadParsesToolPathsAndDefaults -v`
Expected: PASS

- [ ] **Step 6: Run package-level verification**

Run: `go test ./...`
Expected: PASS with the current bootstrap packages

- [ ] **Step 7: Commit**

```bash
git add go.mod cmd/anchorscan/main.go internal/config/config.go internal/config/config_test.go
git commit -m "feat: bootstrap anchorscan cli and config loader"
```

### Task 2: Resolve Targets And Port Presets

**Files:**
- Create: `internal/target/parse.go`
- Create: `internal/ports/resolve.go`
- Create: `config/ports-top100.txt`
- Create: `config/ports-top1000.txt`
- Test: `internal/target/parse_test.go`
- Test: `internal/ports/resolve_test.go`

**Interfaces:**
- Consumes: `Config`
- Produces: `func Parse(input string) ([]string, error)`, `func Resolve(spec string, presetDir string) (string, error)`

- [ ] **Step 1: Write the failing target parser test**

```go
package target

import (
	"reflect"
	"testing"
)

func TestParseDeduplicatesCommaSeparatedTargets(t *testing.T) {
	got, err := Parse("192.168.1.10,192.168.1.10,192.168.1.11")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := []string{"192.168.1.10", "192.168.1.11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse mismatch: got %#v want %#v", got, want)
	}
}
```

- [ ] **Step 2: Verify the target parser fails**

Run: `go test ./internal/target -run TestParseDeduplicatesCommaSeparatedTargets -v`
Expected: FAIL with `undefined: Parse`

- [ ] **Step 3: Write the failing port resolver test**

```go
package ports

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveReadsPresetFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ports-top100.txt")
	if err := os.WriteFile(path, []byte("80,443,8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("top100", dir)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got != "80,443,8080" {
		t.Fatalf("unexpected ports: %q", got)
	}
}
```

- [ ] **Step 4: Verify the port resolver fails**

Run: `go test ./internal/ports -run TestResolveReadsPresetFile -v`
Expected: FAIL with `undefined: Resolve`

- [ ] **Step 5: Write minimal implementations**

```go
package target

import "strings"

func Parse(input string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(input, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}
```

```go
package ports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Resolve(spec string, presetDir string) (string, error) {
	switch spec {
	case "full":
		return "1-65535", nil
	case "top100", "top1000":
		name := fmt.Sprintf("ports-%s.txt", spec)
		data, err := os.ReadFile(filepath.Join(presetDir, name))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return spec, nil
	}
}
```

- [ ] **Step 6: Create the preset files**

```text
config/ports-top100.txt
22,80,81,88,110,111,123,135,137,138,139,143,161,389,443,445,465,512,513,514,515,543,544,548,554,587,631,636,873,902,993,995,1025,1080,1099,1433,1521,1723,2049,2181,2375,2379,3306,3389,3690,4369,4443,4500,5000,5001,5432,5601,5672,5900,5938,5984,5985,5986,6082,6379,6443,7001,7002,7070,7180,7443,7474,7601,7777,8000,8008,8009,8080,8081,8088,8090,8091,8443,8800,8834,8880,8888,9000,9042,9090,9200,9300,9418,9999,10000,11211,15672,27017,28017,50070

config/ports-top1000.txt
21,22,23,25,53,67,68,69,80,81,88,110,111,123,135,137,138,139,143,161,162,179,389,427,443,445,465,500,512,513,514,515,520,524,548,554,587,623,631,636,873,902,989,990,993,995,1025,1080,1099,1110,1194,1241,1311,1433,1434,1521,1524,1604,1645,1646,1701,1720,1723,1812,1813,1883,1900,1935,2000,2001,2049,2082,2083,2086,2087,2181,2222,2375,2376,2379,2380,2483,2484,3128,3260,3306,33060,3389,3478,3632,3690,4000,4040,4369,4443,4500,4567,4848,5000,5001,5005,5006,5007,5044,5060,5061,5222,5232,5353,5432,5555,5601,5671,5672,5800,5900,5938,5984,5985,5986,6000,6082,61616,6379,6380,6443,6667,7001,7002,7070,7180,7199,7443,7474,7601,7700,7777,7800,8000,8001,8008,8009,8010,8020,8080,8081,8082,8083,8086,8088,8090,8091,8100,8180,8181,8443,8500,8530,8531,8686,8800,8834,8880,8888,9000,9042,9060,9080,9090,9092,9200,9300,9418,9443,9990,9999,10000,10050,10051,11211,15672,27017,27018,28017,50070
```

- [ ] **Step 7: Run focused tests**

Run: `go test ./internal/target ./internal/ports -v`
Expected: PASS

- [ ] **Step 8: Run package-level verification**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/target/parse.go internal/target/parse_test.go internal/ports/resolve.go internal/ports/resolve_test.go config/ports-top100.txt config/ports-top1000.txt
git commit -m "feat: add target parsing and port preset resolution"
```

### Task 3: Add External Tool Runner And Port Discovery

**Files:**
- Create: `internal/tools/runner.go`
- Create: `internal/tools/rustscan.go`
- Test: `internal/tools/runner_test.go`
- Test: `internal/tools/rustscan_test.go`

**Interfaces:**
- Consumes: `Config`, resolved targets, resolved ports
- Produces: `type Runner interface`, `func NewExecRunner() Runner`, `func DiscoverPorts(r Runner, binaryPath string, target string, ports string, extraArgs []string) ([]int, error)`

- [ ] **Step 1: Write the failing rustscan command-shaping test**

```go
package tools

import (
	"context"
	"reflect"
	"testing"
)

type fakeRunner struct {
	args []string
}

func (f *fakeRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	f.args = append([]string{binary}, args...)
	return []byte("Open 80\nOpen 443\n"), nil
}

func TestDiscoverPortsBuildsRustscanCommand(t *testing.T) {
	runner := &fakeRunner{}
	got, err := DiscoverPorts(context.Background(), runner, "/opt/rustscan", "192.168.1.10", "80,443", []string{"--batch-size", "500"})
	if err != nil {
		t.Fatalf("DiscoverPorts returned error: %v", err)
	}

	wantPorts := []int{80, 443}
	if !reflect.DeepEqual(got, wantPorts) {
		t.Fatalf("ports mismatch: got %#v want %#v", got, wantPorts)
	}

	wantArgs := []string{"/opt/rustscan", "-a", "192.168.1.10", "--ports", "80,443", "--no-nmap", "--batch-size", "500"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args mismatch: got %#v want %#v", runner.args, wantArgs)
	}
}
```

- [ ] **Step 2: Verify the rustscan test fails**

Run: `go test ./internal/tools -run TestDiscoverPortsBuildsRustscanCommand -v`
Expected: FAIL with `undefined: DiscoverPorts`

- [ ] **Step 3: Write minimal interfaces and implementation**

```go
package tools

import (
	"context"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, binary string, args []string) ([]byte, error)
}

type ExecRunner struct{}

func NewExecRunner() Runner {
	return ExecRunner{}
}

func (ExecRunner) Run(ctx context.Context, binary string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, binary, args...).CombinedOutput()
}
```

```go
package tools

import (
	"context"
	"regexp"
	"sort"
	"strconv"
)

var openPortPattern = regexp.MustCompile(`\b(\d+)\b`)

func DiscoverPorts(ctx context.Context, runner Runner, binaryPath string, target string, ports string, extraArgs []string) ([]int, error) {
	args := []string{"-a", target, "--ports", ports, "--no-nmap"}
	args = append(args, extraArgs...)
	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return nil, err
	}

	matches := openPortPattern.FindAllString(string(out), -1)
	seen := map[int]struct{}{}
	var found []int
	for _, match := range matches {
		port, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		found = append(found, port)
	}
	sort.Ints(found)
	return found, nil
}
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/tools -run TestDiscoverPortsBuildsRustscanCommand -v`
Expected: PASS

- [ ] **Step 5: Add one failure-path test**

```go
func TestDiscoverPortsReturnsRunnerError(t *testing.T) {
	runner := failingRunner{err: errors.New("boom")}
	_, err := DiscoverPorts(context.Background(), runner, "/opt/rustscan", "192.168.1.10", "80", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
```

```go
type failingRunner struct {
	err error
}

func (f failingRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	return nil, f.err
}
```

- [ ] **Step 6: Re-run the package tests**

Run: `go test ./internal/tools -v`
Expected: PASS

- [ ] **Step 7: Run package-level verification**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tools/runner.go internal/tools/runner_test.go internal/tools/rustscan.go internal/tools/rustscan_test.go
git commit -m "feat: add external runner and rustscan orchestration"
```

### Task 4: Parse Nmap XML And Classify Services

**Files:**
- Create: `internal/tools/nmap.go`
- Create: `internal/fingerprint/nmap_xml.go`
- Create: `internal/fingerprint/normalize.go`
- Create: `internal/fingerprint/classify.go`
- Create: `config/service-aliases.yaml`
- Test: `internal/fingerprint/nmap_xml_test.go`
- Test: `internal/fingerprint/classify_test.go`

**Interfaces:**
- Consumes: open ports from `DiscoverPorts`
- Produces: `func Fingerprint(ctx context.Context, r Runner, binaryPath string, ip string, ports []int, extraArgs []string) ([]ServiceFingerprint, error)`, `type ServiceFingerprint struct`

- [ ] **Step 1: Write the failing XML parser test**

```go
package fingerprint

import "testing"

func TestParseNmapXMLExtractsServiceFields(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <host>
    <address addr="192.168.1.10" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="8443">
        <state state="open"/>
        <service name="ssl/http" product="nginx" version="1.24.0" tunnel="ssl"/>
      </port>
    </ports>
  </host>
</nmaprun>`)

	got, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("unexpected fingerprint count: %d", len(got))
	}
	if got[0].Service != "ssl/http" || got[0].Product != "nginx" || got[0].Tunnel != "ssl" {
		t.Fatalf("unexpected fingerprint: %#v", got[0])
	}
}
```

- [ ] **Step 2: Verify the parser fails**

Run: `go test ./internal/fingerprint -run TestParseNmapXMLExtractsServiceFields -v`
Expected: FAIL with `undefined: ParseNmapXML`

- [ ] **Step 3: Write the failing classifier test**

```go
func TestClassifyMarksWebFromSSLHTTPService(t *testing.T) {
	fp := ServiceFingerprint{
		IP:      "192.168.1.10",
		Port:    8443,
		Service: "ssl/http",
		Product: "nginx",
		Tunnel:  "ssl",
	}

	got := Classify(fp)
	if !got.IsWeb {
		t.Fatalf("expected web classification: %#v", got)
	}
	if got.URL != "https://192.168.1.10:8443" {
		t.Fatalf("unexpected url: %q", got.URL)
	}
}
```

- [ ] **Step 4: Verify the classifier fails**

Run: `go test ./internal/fingerprint -run TestClassifyMarksWebFromSSLHTTPService -v`
Expected: FAIL with `undefined: Classify`

- [ ] **Step 5: Write minimal implementation**

```go
package fingerprint

type ServiceFingerprint struct {
	IP         string
	Port       int
	Protocol   string
	Service    string
	Product    string
	Version    string
	ExtraInfo  string
	Tunnel     string
	Normalized string
	IsWeb      bool
	URL        string
}
```

```go
func Classify(fp ServiceFingerprint) ServiceFingerprint {
	out := fp
	out.Normalized = normalizeService(fp.Service, fp.Product)
	if strings.Contains(fp.Service, "http") || strings.Contains(fp.Product, "nginx") || strings.Contains(fp.Product, "apache") || strings.Contains(fp.Product, "tomcat") {
		out.IsWeb = true
		scheme := "http"
		if fp.Tunnel == "ssl" || strings.Contains(fp.Service, "https") || strings.Contains(fp.Service, "ssl/http") {
			scheme = "https"
		}
		out.URL = fmt.Sprintf("%s://%s:%d", scheme, fp.IP, fp.Port)
	}
	return out
}
```

- [ ] **Step 6: Add the Nmap command wrapper**

```go
func Fingerprint(ctx context.Context, r tools.Runner, binaryPath string, ip string, ports []int, extraArgs []string) ([]ServiceFingerprint, error)
```

```go
func Fingerprint(ctx context.Context, r tools.Runner, binaryPath string, ip string, ports []int, extraArgs []string) ([]ServiceFingerprint, error) {
	baseArgs := []string{"-sV", "--version-intensity", "7", "-p", joinPorts(ports), ip, "-oX", "-"}
	baseArgs = append(baseArgs, extraArgs...)
	out, err := r.Run(ctx, binaryPath, baseArgs)
	if err != nil {
		return nil, err
	}

	parsed, err := ParseNmapXML(out)
	if err != nil {
		return nil, err
	}

	result := make([]ServiceFingerprint, 0, len(parsed))
	for _, fp := range parsed {
		result = append(result, Classify(fp))
	}
	return result, nil
}
```

- [ ] **Step 7: Run focused tests**

Run: `go test ./internal/fingerprint -v`
Expected: PASS

- [ ] **Step 8: Run package-level verification**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/tools/nmap.go internal/fingerprint/nmap_xml.go internal/fingerprint/normalize.go internal/fingerprint/classify.go internal/fingerprint/nmap_xml_test.go internal/fingerprint/classify_test.go config/service-aliases.yaml
git commit -m "feat: add nmap fingerprint parsing and service classification"
```

### Task 5: Add HTTPX Enrichment And Fingerprint-Driven Vulnerability Mapping

**Files:**
- Create: `internal/tools/httpx.go`
- Create: `internal/tools/nuclei.go`
- Create: `internal/vuln/nse.go`
- Create: `internal/vuln/nuclei_tags.go`
- Create: `config/service-tags.yaml`
- Create: `config/nse.yaml`
- Test: `internal/tools/httpx_test.go`
- Test: `internal/vuln/nuclei_tags_test.go`
- Test: `internal/vuln/nse_test.go`

**Interfaces:**
- Consumes: `ServiceFingerprint`
- Produces: `type HTTPResult struct`, `func EnrichWeb(...)`, `func MatchNSE(...) []string`, `func MatchNucleiTags(...) MatchResult`

- [ ] **Step 1: Write the failing nuclei tag mapping test**

```go
package vuln

import (
	"reflect"
	"testing"

	"new-anchor/internal/fingerprint"
)

func TestMatchNucleiTagsUsesServiceAndProductRules(t *testing.T) {
	rules := []TagRule{
		{
			Name:       "redis",
			Service:    []string{"redis"},
			Product:    []string{"redis"},
			NucleiTags: []string{"redis"},
			Target:     "hostport",
		},
	}

	fp := fingerprint.ServiceFingerprint{
		IP:         "192.168.1.10",
		Port:       6379,
		Service:    "redis",
		Product:    "redis",
		Normalized: "redis",
	}

	got := MatchNucleiTags(fp, HTTPResult{}, rules)
	want := MatchResult{Tags: []string{"redis"}, Target: "hostport", Address: "192.168.1.10:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: got %#v want %#v", got, want)
	}
}
```

- [ ] **Step 2: Verify the nuclei tag test fails**

Run: `go test ./internal/vuln -run TestMatchNucleiTagsUsesServiceAndProductRules -v`
Expected: FAIL with `undefined: MatchNucleiTags`

- [ ] **Step 3: Write the failing NSE selection test**

```go
func TestMatchNSEReturnsConfiguredScriptsForNormalizedService(t *testing.T) {
	rules := map[string][]string{
		"redis": {"redis-info"},
	}

	fp := fingerprint.ServiceFingerprint{Normalized: "redis"}
	got := MatchNSE(fp, rules)
	want := []string{"redis-info"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected scripts: got %#v want %#v", got, want)
	}
}
```

- [ ] **Step 4: Verify the NSE test fails**

Run: `go test ./internal/vuln -run TestMatchNSEReturnsConfiguredScriptsForNormalizedService -v`
Expected: FAIL with `undefined: MatchNSE`

- [ ] **Step 5: Write minimal matching implementations**

```go
package vuln

import (
	"fmt"
	"strings"

	"new-anchor/internal/fingerprint"
)

type HTTPResult struct {
	URL  string
	Tech []string
}

type TagRule struct {
	Name       string
	Service    []string
	Product    []string
	Tech       []string
	NucleiTags []string
	Target     string
}

type MatchResult struct {
	Tags    []string
	Target  string
	Address string
}

func MatchNSE(fp fingerprint.ServiceFingerprint, rules map[string][]string) []string {
	return append([]string(nil), rules[fp.Normalized]...)
}

func MatchNucleiTags(fp fingerprint.ServiceFingerprint, http HTTPResult, rules []TagRule) MatchResult {
	for _, rule := range rules {
		if contains(rule.Service, fp.Normalized) || contains(rule.Product, fp.Product) || overlaps(rule.Tech, http.Tech) {
			address := fmt.Sprintf("%s:%d", fp.IP, fp.Port)
			if rule.Target == "url" && http.URL != "" {
				address = http.URL
			}
			return MatchResult{Tags: append([]string(nil), rule.NucleiTags...), Target: rule.Target, Address: address}
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
```

- [ ] **Step 6: Add the httpx wrapper test and implementation**

```go
func TestEnrichWebBuildsHTTPXCommandAndParsesJSON(t *testing.T) {
	runner := &fakeRunner{
		output: []byte(`{"url":"https://192.168.1.10:8443","status-code":200,"title":"Admin","tech":["nginx","Vue.js"]}`),
	}

	fp := fingerprint.ServiceFingerprint{
		IP:    "192.168.1.10",
		Port:  8443,
		IsWeb: true,
		URL:   "https://192.168.1.10:8443",
	}

	got, err := EnrichWeb(context.Background(), runner, "/opt/httpx", fp, nil)
	if err != nil {
		t.Fatalf("EnrichWeb returned error: %v", err)
	}
	if got.URL != "https://192.168.1.10:8443" || got.Title != "Admin" {
		t.Fatalf("unexpected http result: %#v", got)
	}
	wantArgs := []string{"/opt/httpx", "-json", "-status-code", "-title", "-tech-detect", "-follow-redirects", "-u", "https://192.168.1.10:8443"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args mismatch: got %#v want %#v", runner.args, wantArgs)
	}
}
```

```go
type HTTPResult struct {
	URL        string   `json:"url"`
	StatusCode int      `json:"status-code"`
	Title      string   `json:"title"`
	Tech       []string `json:"tech"`
}

func EnrichWeb(ctx context.Context, runner tools.Runner, binaryPath string, fp fingerprint.ServiceFingerprint, extraArgs []string) (HTTPResult, error) {
	args := []string{"-json", "-status-code", "-title", "-tech-detect", "-follow-redirects", "-u", fp.URL}
	args = append(args, extraArgs...)
	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return HTTPResult{}, err
	}
	var result HTTPResult
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		return HTTPResult{}, err
	}
	return result, nil
}
```

- [ ] **Step 7: Create the YAML configs**

```yaml
# config/service-tags.yaml
- name: redis
  service: ["redis"]
  product: ["redis"]
  tech: []
  nuclei_tags: ["redis"]
  target: "hostport"

- name: tomcat
  service: ["http"]
  product: ["tomcat", "apache tomcat"]
  tech: ["tomcat"]
  nuclei_tags: ["tomcat", "apache-tomcat"]
  target: "url"

- name: nginx
  service: ["http"]
  product: ["nginx"]
  tech: ["nginx"]
  nuclei_tags: ["nginx"]
  target: "url"

- name: mysql
  service: ["mysql"]
  product: ["mysql", "mariadb"]
  tech: []
  nuclei_tags: ["mysql", "mariadb"]
  target: "hostport"

- name: smb
  service: ["smb"]
  product: ["samba", "microsoft-ds"]
  tech: []
  nuclei_tags: ["smb"]
  target: "hostport"
```

```yaml
# config/nse.yaml
redis:
  - redis-info
mysql:
  - mysql-info
  - mysql-empty-password
smb:
  - smb-protocols
  - smb-security-mode
ssh:
  - ssh2-enum-algos
  - ssh-hostkey
http:
  - http-title
  - http-server-header
tomcat:
  - http-tomcat-manager
```

- [ ] **Step 8: Run focused tests**

Run: `go test ./internal/tools ./internal/vuln -v`
Expected: PASS

- [ ] **Step 9: Run package-level verification**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/tools/httpx.go internal/tools/httpx_test.go internal/tools/nuclei.go internal/vuln/nse.go internal/vuln/nse_test.go internal/vuln/nuclei_tags.go internal/vuln/nuclei_tags_test.go config/service-tags.yaml config/nse.yaml
git commit -m "feat: add httpx enrichment and fingerprint-driven vuln mapping"
```

### Task 6: Persist Scan Results And Render Reports

**Files:**
- Create: `internal/store/sqlite.go`
- Create: `internal/app/scan.go`
- Create: `internal/app/report.go`
- Create: `internal/report/json.go`
- Create: `internal/report/html.go`
- Create: `internal/report/templates/report.html`
- Test: `internal/store/sqlite_test.go`
- Test: `internal/report/json_test.go`
- Test: `internal/report/html_test.go`

**Interfaces:**
- Consumes: classified fingerprints, HTTPX data, NSE mappings, Nuclei findings
- Produces: `type RunStore interface`, `func SaveFingerprint(...) error`, `func WriteJSON(...) error`, `func WriteHTML(...) error`

- [ ] **Step 1: Write the failing store round-trip test**

```go
package store

import (
	"testing"

	"new-anchor/internal/fingerprint"
)

func TestSQLiteStoreSavesAndListsFingerprints(t *testing.T) {
	db := t.TempDir() + "/scan.db"
	store, err := Open(db)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	fp := fingerprint.ServiceFingerprint{
		IP:         "192.168.1.10",
		Port:       6379,
		Service:    "redis",
		Product:    "redis",
		Normalized: "redis",
	}
	if err := store.SaveFingerprint("run-1", fp); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}

	got, err := store.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(got) != 1 || got[0].Port != 6379 {
		t.Fatalf("unexpected rows: %#v", got)
	}
}
```

- [ ] **Step 2: Verify the store test fails**

Run: `go test ./internal/store -run TestSQLiteStoreSavesAndListsFingerprints -v`
Expected: FAIL with `undefined: Open`

- [ ] **Step 3: Write the failing JSON report test**

```go
package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"new-anchor/internal/fingerprint"
)

func TestWriteJSONOutputsHostAndPortData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	input := []fingerprint.ServiceFingerprint{
		{IP: "192.168.1.10", Port: 8080, Service: "http", Product: "tomcat", IsWeb: true, URL: "http://192.168.1.10:8080"},
	}

	if err := WriteJSON(path, input); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}
```

- [ ] **Step 4: Verify the JSON report test fails**

Run: `go test ./internal/report -run TestWriteJSONOutputsHostAndPortData -v`
Expected: FAIL with `undefined: WriteJSON`

- [ ] **Step 5: Write minimal store and report implementations**

```go
type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	schema := `
CREATE TABLE IF NOT EXISTS fingerprints (
  run_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  service TEXT NOT NULL,
  product TEXT NOT NULL,
  normalized TEXT NOT NULL,
  is_web INTEGER NOT NULL,
  url TEXT NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}
```

```go
func WriteJSON(path string, fps []fingerprint.ServiceFingerprint) error {
	payload := map[string]any{
		"scan_meta": map[string]any{"tool": "anchorscan"},
		"hosts":     fps,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
```

- [ ] **Step 6: Add one app-level orchestration test**

```go
func TestRunScanStoresFingerprintAndWritesJSONReport(t *testing.T) {
	runner := &sequenceRunner{
		outputs: [][]byte{
			[]byte("Open 8080\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat" version="9.0.65"/></port></ports></host></nmaprun>`),
			[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
		},
	}

	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	store, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID: "run-1",
		Targets: []string{"192.168.1.10"},
		Ports: "8080",
		Tools: ToolPaths{
			Rustscan: "/opt/rustscan",
			Nmap:     "/opt/nmap",
			Httpx:    "/opt/httpx",
		},
		JSONReportPath: reportPath,
	}

	if err := RunScan(context.Background(), runner, store, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	rows, err := store.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].URL != "http://192.168.1.10:8080" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report not written: %v", err)
	}
}
```

```go
type sequenceRunner struct {
	outputs [][]byte
	index   int
}

func (s *sequenceRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	out := s.outputs[s.index]
	s.index++
	return out, nil
}
```

- [ ] **Step 7: Run focused tests**

Run: `go test ./internal/store ./internal/report ./internal/app -v`
Expected: PASS

- [ ] **Step 8: Run package-level verification**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/store/sqlite.go internal/store/sqlite_test.go internal/app/scan.go internal/app/report.go internal/report/json.go internal/report/json_test.go internal/report/html.go internal/report/html_test.go internal/report/templates/report.html
git commit -m "feat: add persistence and report generation"
```

## Self-Review

### Spec Coverage

- Fixed pipeline with `rustscan` then `nmap -sV`: covered by Tasks 3 and 4.
- Tool paths supplied from config rather than bundled binaries: covered by Task 1 and used throughout later tasks.
- Supported port input forms `csv`, `top100`, `top1000`, `full`: covered by Task 2.
- Fingerprint-driven web routing and URL scheme handling: covered by Task 4.
- `httpx` only for web assets: covered by Task 5.
- NSE and Nuclei selected after fingerprinting, using small defaults plus config extension: covered by Task 5.
- SQLite-backed persistence and resumable storage basis: covered by Task 6.
- JSON and HTML reporting: covered by Task 6.

### Placeholder Scan

- The only remaining shorthand is `go.mod`, which should use the module path `new-anchor` so the package imports shown in the plan compile as written.
- The report template file in Task 6 still needs one explicit HTML table layout when implemented.

### Type Consistency

- `ServiceFingerprint` is defined in Task 4 and reused in Tasks 5 and 6 consistently.
- `Runner` and `DiscoverPorts` from Task 3 feed `Fingerprint` in Task 4 without renaming.
- `HTTPResult`, `MatchResult`, and `MatchNSE` are introduced in Task 5 and only consumed after that point.

### Fixups To Apply During Execution

- Start Task 1 by writing `go.mod` with `module new-anchor` and the minimum dependencies used by the first failing test.
- Keep `sequenceRunner` only in tests; the production runner remains the single `ExecRunner` implementation.
