# Nuclei Finding Endpoint Attribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure every Nuclei finding is stored under its actual endpoint, remove broad `default-login` selection from URL-targeted rules, and correct the five misattributed Redis rows in `run-20260712-145702.423172000`.

**Architecture:** `internal/tools` preserves and resolves the structured endpoint carried by Nuclei JSONL. A single `internal/app` conversion function creates `report.Finding` values for both full scans and standalone Nuclei runs, while the existing report grouping and HTML rendering remain unchanged. Historical repair stays an explicit, backed-up SQLite operation outside product runtime code.

**Tech Stack:** Go 1.x standard library, SQLite, YAML configuration, existing AnchorScan test suite

## Global Constraints

- Do not modify `internal/report/html.go` or `internal/report/model.go`.
- Do not add a third-party dependency or a permanent migration/data-cleaning subsystem.
- Prefer structured `ip` and `port`; resolve the host from `host`/`matched-at`, resolve the port from `host`/`url`/`matched-at`, and only then fall back to the current fingerprint.
- Invalid or absent endpoint metadata must not discard an otherwise valid finding.
- Remove `default-login` only from `target: "url"` rules; leave `target: "hostport"` rules unchanged.
- Historical repair must update exactly five rows, preserve duplicates, and create a recoverable database backup first.
- Before the first production edit, run the project `pre-edit-safety-gate` checks for `ParseNucleiJSONL`, `scanTarget`, and `runNucleiTool`.
- Follow TDD: observe each focused test fail before implementing its production change.

---

### Task 1: Preserve and resolve Nuclei result endpoints

**Files:**
- Modify: `internal/tools/nuclei.go:13-95`
- Test: `internal/tools/nuclei_test.go`

**Interfaces:**
- Consumes: Nuclei JSONL fields `host`, `ip`, `port`, `url`, and `matched-at`.
- Produces: `func (f NucleiFinding) Endpoint(fallbackHost string, fallbackPort int) (string, int)`.

- [ ] **Step 1: Add failing parser and endpoint-resolution tests**

Append these tests to `internal/tools/nuclei_test.go`:

```go
func TestParseNucleiJSONLKeepsEndpointFields(t *testing.T) {
	input := []byte(`{"template-id":"redis-default-logins","info":{"name":"Redis - Default Logins","severity":"high"},"host":"172.22.0.1","ip":"172.22.0.1","port":"6379","url":"172.22.0.1:6379","matched-at":"172.22.0.1:6379"}` + "\n")

	findings, err := ParseNucleiJSONL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %#v", findings)
	}
	got := findings[0]
	if got.Host != "172.22.0.1" || got.IP != "172.22.0.1" || got.Port != "6379" || got.URL != "172.22.0.1:6379" {
		t.Fatalf("endpoint fields = %#v", got)
	}
}

func TestNucleiFindingEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		finding      NucleiFinding
		fallbackHost string
		fallbackPort int
		wantHost     string
		wantPort     int
	}{
		{
			name: "structured redis endpoint",
			finding: NucleiFinding{IP: "172.22.0.1", Port: "6379", MatchedAt: "172.22.0.1:6379"},
			fallbackHost: "172.22.0.1", fallbackPort: 8080,
			wantHost: "172.22.0.1", wantPort: 6379,
		},
		{
			name: "https standard port",
			finding: NucleiFinding{MatchedAt: "https://example.test/manager"},
			fallbackHost: "fallback.test", fallbackPort: 8080,
			wantHost: "example.test", wantPort: 443,
		},
		{
			name: "bracketed ipv6",
			finding: NucleiFinding{Host: "[2001:db8::10]:8443"},
			fallbackHost: "2001:db8::20", fallbackPort: 8080,
			wantHost: "2001:db8::10", wantPort: 8443,
		},
		{
			name: "invalid port falls back",
			finding: NucleiFinding{IP: "192.0.2.10", Port: "70000"},
			fallbackHost: "192.0.2.20", fallbackPort: 8080,
			wantHost: "192.0.2.10", wantPort: 8080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := tt.finding.Endpoint(tt.fallbackHost, tt.fallbackPort)
			if host != tt.wantHost || port != tt.wantPort {
				t.Fatalf("Endpoint() = %s:%d, want %s:%d", host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}
```

- [ ] **Step 2: Run the focused tests and confirm the red state**

Run:

```bash
go test ./internal/tools -run 'Test(ParseNucleiJSONLKeepsEndpointFields|NucleiFindingEndpoint)$'
```

Expected: compilation fails because `NucleiFinding` has no `Host`, `IP`, `Port`, `URL`, or `Endpoint` yet.

- [ ] **Step 3: Extend `NucleiFinding` and parse the structured fields**

Add imports `net/netip`, `net/url`, and `strconv` to `internal/tools/nuclei.go`. Extend the type and the temporary JSON row:

```go
type NucleiFinding struct {
	TemplateID       string
	Name             string
	Severity         string
	Host             string
	IP               string
	Port             string
	URL              string
	MatchedAt        string
	MatcherName      string
	ExtractedResults []string
	CurlCommand      string
	Raw              string
}
```

```go
var row struct {
	TemplateID       string   `json:"template-id"`
	MatcherName      string   `json:"matcher-name"`
	ExtractedResults []string `json:"extracted-results"`
	ExtractorResults []string `json:"extractor-results"`
	CurlCommand      string   `json:"curl-command"`
	Host             string   `json:"host"`
	IP               string   `json:"ip"`
	Port             string   `json:"port"`
	URL              string   `json:"url"`
	Info             struct {
		Name     string `json:"name"`
		Severity string `json:"severity"`
	} `json:"info"`
	MatchedAt string `json:"matched-at"`
}
```

Copy the four new fields when constructing `NucleiFinding`:

```go
Host:      row.Host,
IP:        row.IP,
Port:      row.Port,
URL:       row.URL,
MatchedAt: row.MatchedAt,
```

- [ ] **Step 4: Implement minimal standard-library endpoint resolution**

Add the following below `ParseNucleiJSONL` in `internal/tools/nuclei.go`:

```go
func (f NucleiFinding) Endpoint(fallbackHost string, fallbackPort int) (string, int) {
	host := strings.TrimSpace(f.IP)
	if host == "" {
		for _, value := range []string{f.Host, f.MatchedAt} {
			host, _ = parseNucleiEndpoint(value)
			if host != "" {
				break
			}
		}
	}
	if host == "" {
		host = fallbackHost
	}

	port := parseNucleiPort(f.Port)
	if port == 0 {
		for _, value := range []string{f.Host, f.URL, f.MatchedAt} {
			_, port = parseNucleiEndpoint(value)
			if port != 0 {
				break
			}
		}
	}
	if port == 0 {
		port = fallbackPort
	}
	return host, port
}

func parseNucleiEndpoint(value string) (string, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0
	}
	if addr, err := netip.ParseAddr(strings.Trim(value, "[]")); err == nil {
		return addr.String(), 0
	}

	candidate := value
	if !strings.Contains(candidate, "://") {
		candidate = "//" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Hostname() == "" {
		return "", 0
	}
	port := parseNucleiPort(parsed.Port())
	if port == 0 {
		switch parsed.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		}
	}
	return parsed.Hostname(), port
}

func parseNucleiPort(value string) int {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}
```

- [ ] **Step 5: Format and verify the green state**

Run:

```bash
gofmt -w internal/tools/nuclei.go internal/tools/nuclei_test.go
go test ./internal/tools
```

Expected: `internal/tools` passes, including URL defaults, IPv6, invalid-port fallback, and structured Redis fields.

- [ ] **Step 6: Commit Task 1**

```bash
git add internal/tools/nuclei.go internal/tools/nuclei_test.go
git commit -m "fix: preserve nuclei finding endpoints"
```

---

### Task 2: Use one conversion path in full scans and standalone tool runs

**Files:**
- Modify: `internal/app/scan.go:312-323`
- Modify: `internal/app/tool_run.go:268-308`
- Test: `internal/app/scan_test.go`
- Test: `internal/app/tool_run_test.go:140-160`

**Interfaces:**
- Consumes: `NucleiFinding.Endpoint(fallbackHost string, fallbackPort int) (string, int)` from Task 1.
- Produces: `func findingFromNuclei(result tools.NucleiFinding, fallback fingerprint.ServiceFingerprint, fingerprints []fingerprint.ServiceFingerprint) report.Finding`.

- [ ] **Step 1: Add a failing shared-conversion test**

Add to `internal/app/scan_test.go`:

```go
func TestFindingFromNucleiUsesResultEndpoint(t *testing.T) {
	fallback := fingerprint.ServiceFingerprint{IP: "172.22.0.1", Port: 8080, Protocol: "tcp"}
	fingerprints := []fingerprint.ServiceFingerprint{
		fallback,
		{IP: "172.22.0.1", Port: 6379, Protocol: "tcp"},
	}
	result := tools.NucleiFinding{
		TemplateID: "redis-default-logins",
		Name:       "Redis - Default Logins",
		Severity:   "high",
		IP:         "172.22.0.1",
		Port:       "6379",
		MatchedAt:  "172.22.0.1:6379",
	}

	got := findingFromNuclei(result, fallback, fingerprints)
	if got.IP != "172.22.0.1" || got.Port != 6379 || got.Protocol != "tcp" || got.Target != "172.22.0.1:6379" {
		t.Fatalf("finding = %#v", got)
	}
}

func TestFindingFromNucleiFallsBackToCurrentFingerprint(t *testing.T) {
	fallback := fingerprint.ServiceFingerprint{IP: "192.0.2.10", Port: 8080, Protocol: "tcp"}
	got := findingFromNuclei(tools.NucleiFinding{TemplateID: "x"}, fallback, nil)
	if got.IP != fallback.IP || got.Port != fallback.Port || got.Protocol != fallback.Protocol {
		t.Fatalf("finding = %#v", got)
	}
}
```

- [ ] **Step 2: Make the standalone-tool regression test demand port 6379**

In `TestRunToolNucleiSavesFindings` in `internal/app/tool_run_test.go`, change the fake JSONL and final assertion to:

```go
return []byte(`{"template-id":"redis-default-logins","info":{"name":"Redis - Default Logins","severity":"high"},"host":"192.0.2.10","ip":"192.0.2.10","port":"6379","matched-at":"192.0.2.10:6379"}` + "\n"), nil
```

```go
if len(findings) != 1 || findings[0].Source != "nuclei" || findings[0].ID != "redis-default-logins" || findings[0].IP != "192.0.2.10" || findings[0].Port != 6379 || findings[0].Target != "192.0.2.10:6379" {
	t.Fatalf("findings = %#v", findings)
}
```

- [ ] **Step 3: Run both focused tests and confirm the red state**

Run:

```bash
go test ./internal/app -run 'Test(FindingFromNuclei|RunToolNucleiSavesFindings)'
```

Expected: compilation fails because `findingFromNuclei` does not exist.

- [ ] **Step 4: Add the shared conversion function**

Add below `formatNucleiEvidence` in `internal/app/scan.go`:

```go
func findingFromNuclei(result tools.NucleiFinding, fallback fingerprint.ServiceFingerprint, fingerprints []fingerprint.ServiceFingerprint) report.Finding {
	ip, port := result.Endpoint(fallback.IP, fallback.Port)
	protocol := fallback.Protocol
	for _, fp := range fingerprints {
		if fp.IP == ip && fp.Port == port {
			protocol = fp.Protocol
			break
		}
	}
	return report.Finding{
		IP:       ip,
		Port:     port,
		Protocol: protocol,
		Source:   "nuclei",
		ID:       result.TemplateID,
		Severity: result.Severity,
		Summary:  result.Name,
		Target:   result.MatchedAt,
		Output:   formatNucleiEvidence(result),
	}
}
```

- [ ] **Step 5: Replace both duplicated constructors**

In `scanTarget`, replace the current `report.Finding{...}` block with:

```go
finding := findingFromNuclei(result, fp, fingerprints)
```

In `runNucleiTool`, replace its current `report.Finding{...}` block with:

```go
finding := findingFromNuclei(result, fp, nil)
```

Leave each existing `SaveFinding` and append block unchanged.

- [ ] **Step 6: Format and verify the green state**

Run:

```bash
gofmt -w internal/app/scan.go internal/app/scan_test.go internal/app/tool_run.go internal/app/tool_run_test.go
go test ./internal/app
```

Expected: both shared-conversion tests and the persisted standalone-tool test pass; existing scan tests remain green.

- [ ] **Step 7: Commit Task 2**

```bash
git add internal/app/scan.go internal/app/scan_test.go internal/app/tool_run.go internal/app/tool_run_test.go
git commit -m "fix: attribute nuclei findings to matched endpoints"
```

---

### Task 3: Remove broad default-login selection from URL rules

**Files:**
- Modify: `config/service-tags.yaml:153-191`
- Test: `internal/config/config_test.go:159-218`

**Interfaces:**
- Consumes: existing `LoadTagRules` output.
- Produces: URL-targeted product rules containing only product/web-specific tags.

- [ ] **Step 1: Add the failing URL-rule contract**

Inside the `for _, rule := range tagRules` loop in `TestDefaultRuleFilesProvideDualEngineCoverage`, replace the `rule.Target == "url"` branch with:

```go
if rule.Target == "url" {
	for _, tag := range rule.NucleiTags {
		if tag == "default-login" {
			t.Fatalf("url rule %s must not use broad default-login tag: %#v", rule.Name, rule)
		}
	}
	continue
}
```

Replace the preceding contract comment with:

```go
// 双引擎覆盖矩阵契约：
// (1) URL 规则只运行产品/Web 专用 tag，非 Web 规则保留 default-login 弱口令 tag；
// (2) 非 Web 规则 target 必须为 hostport，Web 规则 target 必须为 url。
```

- [ ] **Step 2: Run the focused config test and confirm the red state**

Run:

```bash
go test ./internal/config -run TestDefaultRuleFilesProvideDualEngineCoverage
```

Expected: FAIL on the first URL rule that still contains `default-login` (currently `tomcat`).

- [ ] **Step 3: Remove `default-login` from every URL-targeted rule**

Set the affected `nuclei_tags` values in `config/service-tags.yaml` exactly as follows:

```yaml
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
- name: apache
  service: ["http"]
  product: ["apache", "apache httpd", "apache http server"]
  tech: ["apache"]
  nuclei_tags: ["apache"]
  target: "url"
- name: iis
  service: ["http"]
  product: ["iis", "microsoft iis"]
  tech: ["iis", "microsoft-iis"]
  nuclei_tags: ["iis"]
  target: "url"
- name: wordpress
  service: ["http"]
  tech: ["wordpress"]
  nuclei_tags: ["wordpress"]
  target: "url"
- name: joomla
  service: ["http"]
  tech: ["joomla"]
  nuclei_tags: ["joomla"]
  target: "url"
# 通用 HTTP 兜底：识别为 http 但未命中具体产品时，跑通用 web 模板
- name: http-generic
  service: ["http"]
  nuclei_tags: ["http", "exposure", "misconfig"]
  target: "url"
```

- [ ] **Step 4: Verify configuration tests**

Run:

```bash
go test ./internal/config
```

Expected: all configuration tests pass and every URL rule remains non-empty.

- [ ] **Step 5: Commit Task 3**

```bash
git add config/service-tags.yaml internal/config/config_test.go
git commit -m "fix: scope web nuclei tags to detected products"
```

---

### Task 4: Verify production behavior and release impact

**Files:**
- Modify: `internal/version/version.go:7`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: Tasks 1-3.
- Produces: verified patch release metadata for version `1.7.1`.

- [ ] **Step 1: Run focused and full verification**

Run:

```bash
go test ./internal/tools ./internal/app ./internal/config ./internal/report
go test ./...
git diff --name-only HEAD~3..HEAD -- internal/report
```

Expected: all packages pass. No report source file changes are present in `git diff --name-only HEAD~3..HEAD`.

- [ ] **Step 2: Update patch version metadata**

Change `internal/version/version.go` to:

```go
const Version = "1.7.1"
```

Insert this entry above `1.7.0` in `CHANGELOG.md`:

```markdown
## [1.7.1] - 2026-07-12

### Fixed
- Nuclei findings now use their structured matched host and port instead of being forced onto the fingerprint that launched the scan.
- URL-targeted product rules no longer include the broad `default-login` tag, preventing unrelated network login templates from running during Web scans.
- Full scans and standalone Nuclei runs share the same endpoint-attribution path.
```

- [ ] **Step 3: Verify version output and full tests**

Run:

```bash
go test ./...
go run ./cmd/anchorscan --version
```

Expected: all tests pass and the CLI prints version `1.7.1`.

- [ ] **Step 4: Commit Task 4**

```bash
git add internal/version/version.go CHANGELOG.md
git commit -m "chore: release v1.7.1"
```

---

### Task 5: Correct and re-export the specified historical run

**Files:**
- Runtime data: `data/scans.sqlite`
- Backup: `data/scans.sqlite.before-nuclei-endpoint-fix-20260712`
- Replace after backup: `/Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.html`
- Create: `/Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.json`

**Interfaces:**
- Consumes: existing `anchorscan report` command and the verified run row in `data/scans.sqlite`.
- Produces: five corrected database rows plus regenerated JSON and HTML reports.

- [ ] **Step 1: Verify the exact precondition before any write**

Run:

```bash
sqlite3 -readonly data/scans.sqlite "SELECT COUNT(*) FROM findings WHERE run_id='run-20260712-145702.423172000' AND ip='172.22.0.1' AND port=8080 AND protocol='tcp' AND source='nuclei' AND finding_id='redis-default-logins' AND target='172.22.0.1:6379';"
```

Expected: exactly `5`. Stop without changing data for any other result.

- [ ] **Step 2: Create recoverable database and HTML backups**

Run:

```bash
sqlite3 data/scans.sqlite ".backup 'data/scans.sqlite.before-nuclei-endpoint-fix-20260712'"
cp -p /Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.html /Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.html.before-endpoint-fix
```

Expected: both backup files exist and are non-empty.

- [ ] **Step 3: Perform the guarded transaction**

Run:

```bash
sqlite3 -bail data/scans.sqlite <<'SQL'
BEGIN IMMEDIATE;
CREATE TEMP TABLE endpoint_fix_assertion (affected INTEGER CHECK (affected = 5));
UPDATE findings
SET ip = '172.22.0.1', port = 6379, protocol = 'tcp'
WHERE run_id = 'run-20260712-145702.423172000'
  AND ip = '172.22.0.1'
  AND port = 8080
  AND protocol = 'tcp'
  AND source = 'nuclei'
  AND finding_id = 'redis-default-logins'
  AND target = '172.22.0.1:6379';
INSERT INTO endpoint_fix_assertion VALUES (changes());
COMMIT;
SQL
```

Expected: exit code 0. If the row count is not five, the CHECK fails and the connection closes without committing.

- [ ] **Step 4: Verify database state before exporting**

Run:

```bash
sqlite3 -readonly -header -column data/scans.sqlite "SELECT ip, port, protocol, source, finding_id, target, COUNT(*) AS rows FROM findings WHERE run_id='run-20260712-145702.423172000' AND finding_id IN ('redis-default-logins','tomcat-default-login') GROUP BY ip, port, protocol, source, finding_id, target ORDER BY ip, port, finding_id;"
```

Expected:

- `redis-default-logins` appears at `172.22.0.1:6379/tcp` with `rows=5`.
- No Redis row remains at port 8080.
- Tomcat rows remain unchanged at port 8080.

- [ ] **Step 5: Re-export JSON and HTML through the existing CLI**

Run:

```bash
go run ./cmd/anchorscan report \
  --db data/scans.sqlite \
  --run-id run-20260712-145702.423172000 \
  --json /Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.json \
  --html /Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.html
```

Expected: exit code 0 and `run_id=run-20260712-145702.423172000`.

- [ ] **Step 6: Verify the regenerated artifacts**

Run:

```bash
rg -n -C 8 'data-port="6379"|redis-default-logins|data-port="8080"|tomcat-default-login' /Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.html
rg -n -C 5 'redis-default-logins|"port": 6379|tomcat-default-login|"port": 8080' /Users/kun/Downloads/anchorscan-run-20260712-145702.423172000.json
```

Expected: Redis evidence is inside the 6379 record, Tomcat evidence remains inside the 8080 record, and Redis is absent from the 8080 findings list.

- [ ] **Step 7: Run final repository safety checks**

Run:

```bash
go test ./...
git status --short
```

Expected: all tests pass. Runtime database/report files are not staged; only pre-existing unrelated worktree changes, if any, remain.

## Final Verification Checklist

- [ ] `ParseNucleiJSONL` preserves Nuclei endpoint fields.
- [ ] Full scans and standalone Nuclei runs both use `findingFromNuclei`.
- [ ] Endpoint metadata failures fall back without dropping findings.
- [ ] All URL-targeted rules omit `default-login`; hostport rules are unchanged.
- [ ] No report grouping or HTML template source was modified.
- [ ] The five historical Redis rows are at port 6379 and the original DB/HTML backups exist.
- [ ] Regenerated JSON and HTML agree on Redis 6379 and Tomcat 8080.
- [ ] Version is `1.7.1`, changelog is updated, and `go test ./...` passes.
