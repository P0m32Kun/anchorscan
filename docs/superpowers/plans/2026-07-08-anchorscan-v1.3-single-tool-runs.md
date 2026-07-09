# AnchorScan V1.3 Single Tool Runs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CLI and Web Console single-tool runs for rustscan, nmap, httpx, and nuclei while reusing existing reports and SQLite persistence.

**Architecture:** Add one app-level `RunTool` entry point that switches on the requested tool and writes existing `scan_runs`, `scan_events`, `fingerprints`, and `findings` records. Reuse current `tools` helpers where possible; add only the missing nmap alive and nuclei template helpers. Web uses the existing single-active-run manager with one new `StartTool` method.

**Tech Stack:** Go, stdlib `testing`, `net/http/httptest`, `modernc.org/sqlite`, existing `internal/tools`, `internal/store`, `internal/report`, and embedded HTML templates.

## Global Constraints

- Keep existing `anchorscan scan` pipeline behavior unchanged.
- Do not bundle Metasploit.
- Do not add a generic workflow DAG engine, plugin system, vulnerability encyclopedia, or exploit-based verification.
- Reuse existing SQLite tables at first: `scan_runs`, `scan_events`, `fingerprints`, and `findings`.
- Store manual run type in `scan_runs.profile` as `tool:<name>`.
- Use TDD: every production change starts with a failing test and the failing output is verified before implementation.
- Use the fewest files and dependencies possible; add no new dependency.

---

## File Structure

- Create `internal/app/tool_run.go`: `ToolRunOptions`, `RunTool`, validation, event emission, report writing, per-tool handlers.
- Create `internal/app/tool_run_test.go`: app-level tests with a fake `tools.Runner` and real temp SQLite store.
- Create `internal/app/manual_review.go`: BlueKeep manual-review rule over fingerprints.
- Create `internal/app/manual_review_test.go`: focused tests for RDP on 3389.
- Modify `internal/tools/nmap.go`: add nmap alive helper and parser.
- Create `internal/tools/nmap_alive_test.go`: alive parser/argument tests.
- Modify `internal/tools/nuclei.go`: add template-mode runner.
- Create `internal/tools/nuclei_test.go`: template argument test.
- Modify `internal/app/scan.go`: run manual-review checks for pipeline fingerprints.
- Modify `internal/app/manager.go`: add `StartTool` using same single-active-run guard.
- Modify `cmd/anchorscan/main.go`: add `tool` command and help.
- Modify or create `cmd/anchorscan/main_test.go`: CLI validation and one success path.
- Modify `internal/web/server.go`: add `/tools/new` and `/tools` routes.
- Create `internal/web/templates/tool_new.html`: manual tool form.
- Modify `internal/web/templates/base.html`: add nav link.
- Modify `internal/web/server_test.go`: Web form route and submit test.
- Modify `README.md`: short usage section.

---

### Task 1: Tool-layer gaps for nmap alive and nuclei template mode

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/internal/tools/nmap.go`
- Create: `/Users/kun/DEV/new-Anchor/internal/tools/nmap_alive_test.go`
- Modify: `/Users/kun/DEV/new-Anchor/internal/tools/nuclei.go`
- Create: `/Users/kun/DEV/new-Anchor/internal/tools/nuclei_test.go`

**Interfaces:**
- Produces: `tools.CheckAlive(ctx context.Context, runner tools.Runner, binaryPath string, target string, extraArgs []string) (bool, error)`
- Produces: `tools.RunNucleiTemplate(ctx context.Context, runner tools.Runner, binaryPath string, target string, template string, extraArgs []string) ([]byte, error)`
- Consumed by: Task 2 `app.RunTool`

- [ ] **Step 1: Write failing tests for nmap alive**

Create `/Users/kun/DEV/new-Anchor/internal/tools/nmap_alive_test.go`:

```go
package tools

import (
    "context"
    "reflect"
    "testing"
)

type aliveRunner struct {
    binary string
    args   []string
    out    []byte
}

func (r *aliveRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
    r.binary = binary
    r.args = append([]string(nil), args...)
    return r.out, nil
}

func TestCheckAliveBuildsNmapPingAndParsesUpHost(t *testing.T) {
    runner := &aliveRunner{out: []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`)}

    alive, err := CheckAlive(context.Background(), runner, "nmap", "192.0.2.10", []string{"--min-rate", "50"})
    if err != nil {
        t.Fatal(err)
    }
    if !alive {
        t.Fatal("expected host to be alive")
    }

    want := []string{"-sn", "192.0.2.10", "-oX", "-", "--min-rate", "50"}
    if !reflect.DeepEqual(runner.args, want) {
        t.Fatalf("args = %#v, want %#v", runner.args, want)
    }
}

func TestCheckAliveReturnsFalseForDownHost(t *testing.T) {
    runner := &aliveRunner{out: []byte(`<nmaprun><host><status state="down"/></host></nmaprun>`)}

    alive, err := CheckAlive(context.Background(), runner, "nmap", "192.0.2.10", nil)
    if err != nil {
        t.Fatal(err)
    }
    if alive {
        t.Fatal("expected host to be down")
    }
}
```

- [ ] **Step 2: Verify nmap alive tests fail**

Run:

```bash
go test ./internal/tools -run 'TestCheckAlive' -count=1
```

Expected: FAIL with `undefined: CheckAlive`.

- [ ] **Step 3: Implement nmap alive helper**

Add to `/Users/kun/DEV/new-Anchor/internal/tools/nmap.go`:

```go
type aliveXML struct {
    Hosts []struct {
        Status struct {
            State string `xml:"state,attr"`
        } `xml:"status"`
    } `xml:"host"`
}

func CheckAlive(ctx context.Context, runner Runner, binaryPath string, target string, extraArgs []string) (bool, error) {
    args := []string{"-sn", target, "-oX", "-"}
    args = append(args, extraArgs...)

    out, err := runner.Run(ctx, binaryPath, args)
    if err != nil {
        return false, err
    }

    var parsed aliveXML
    if err := xml.Unmarshal(out, &parsed); err != nil {
        return false, err
    }
    for _, host := range parsed.Hosts {
        if host.Status.State == "up" {
            return true, nil
        }
    }
    return false, nil
}
```

Update imports in `nmap.go` to include `encoding/xml`.

- [ ] **Step 4: Verify nmap alive tests pass**

Run:

```bash
go test ./internal/tools -run 'TestCheckAlive' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing test for nuclei template mode**

Create `/Users/kun/DEV/new-Anchor/internal/tools/nuclei_test.go`:

```go
package tools

import (
    "context"
    "reflect"
    "testing"
)

type nucleiRunner struct {
    args []string
}

func (r *nucleiRunner) Run(_ context.Context, _ string, args []string) ([]byte, error) {
    r.args = append([]string(nil), args...)
    return []byte(`{"template-id":"x","info":{"name":"x","severity":"info"},"matched-at":"http://example.test"}` + "\n"), nil
}

func TestRunNucleiTemplateUsesTemplateFlag(t *testing.T) {
    runner := &nucleiRunner{}

    if _, err := RunNucleiTemplate(context.Background(), runner, "nuclei", "http://example.test", "cves/2021/test.yaml", []string{"-rate-limit", "5"}); err != nil {
        t.Fatal(err)
    }

    want := []string{"-target", "http://example.test", "-t", "cves/2021/test.yaml", "-jsonl", "-rate-limit", "5"}
    if !reflect.DeepEqual(runner.args, want) {
        t.Fatalf("args = %#v, want %#v", runner.args, want)
    }
}
```

- [ ] **Step 6: Verify nuclei template test fails**

Run:

```bash
go test ./internal/tools -run 'TestRunNucleiTemplate' -count=1
```

Expected: FAIL with `undefined: RunNucleiTemplate`.

- [ ] **Step 7: Implement nuclei template helper**

Add to `/Users/kun/DEV/new-Anchor/internal/tools/nuclei.go`:

```go
func RunNucleiTemplate(ctx context.Context, runner Runner, binaryPath string, target string, template string, extraArgs []string) ([]byte, error) {
    args := []string{"-target", target, "-t", template, "-jsonl"}
    args = append(args, extraArgs...)
    return runner.Run(ctx, binaryPath, args)
}
```

- [ ] **Step 8: Verify tool tests pass**

Run:

```bash
go test ./internal/tools -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 1**

```bash
git add internal/tools/nmap.go internal/tools/nmap_alive_test.go internal/tools/nuclei.go internal/tools/nuclei_test.go
git commit -m "feat: add single-tool helper commands"
```

---

### Task 2: App-level `RunTool` persistence

**Files:**
- Create: `/Users/kun/DEV/new-Anchor/internal/app/tool_run.go`
- Create: `/Users/kun/DEV/new-Anchor/internal/app/tool_run_test.go`

**Interfaces:**
- Consumes: `tools.DiscoverPorts`, `tools.Fingerprint`, `tools.CheckAlive`, `tools.EnrichWeb`, `tools.RunNuclei`, `tools.RunNucleiTemplate`, `tools.ParseNucleiJSONL`
- Produces: `app.ToolRunOptions`
- Produces: `app.RunTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) error`
- Consumed by: CLI and Web tasks

- [ ] **Step 1: Write failing app tests**

Create `/Users/kun/DEV/new-Anchor/internal/app/tool_run_test.go`:

```go
package app

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/P0m32Kun/anchorscan/internal/store"
)

type toolRunnerFunc func(binary string, args []string) ([]byte, error)

func (f toolRunnerFunc) Run(_ context.Context, binary string, args []string) ([]byte, error) {
    return f(binary, args)
}

func newToolRunStore(t *testing.T) *store.Store {
    t.Helper()
    st, err := store.Open(filepath.Join(t.TempDir(), "scans.sqlite"))
    if err != nil {
        t.Fatal(err)
    }
    return st
}

func TestRunToolRustscanSavesOpenPorts(t *testing.T) {
    st := newToolRunStore(t)
    jsonPath := filepath.Join(t.TempDir(), "report.json")
    runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
        return []byte("[80,443]"), nil
    })

    err := RunTool(context.Background(), runner, st, ToolRunOptions{
        RunID: "run-rustscan", Tool: "rustscan", Target: "192.0.2.10", Ports: "80,443", Tools: ToolPaths{Rustscan: "rustscan"}, JSONReportPath: jsonPath,
    })
    if err != nil {
        t.Fatal(err)
    }

    fps, err := st.ListFingerprints("run-rustscan")
    if err != nil {
        t.Fatal(err)
    }
    if len(fps) != 2 || fps[0].Port != 80 || fps[1].Port != 443 {
        t.Fatalf("fingerprints = %#v", fps)
    }
    if _, err := os.Stat(jsonPath); err != nil {
        t.Fatal(err)
    }
}

func TestRunToolNmapServiceSavesFingerprintsAndManualReview(t *testing.T) {
    st := newToolRunStore(t)
    xml := `<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="3389"><state state="open"/><service name="ms-wbt-server" product="Microsoft Terminal Services" version=""/></port></ports></host></nmaprun>`
    runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) { return []byte(xml), nil })

    err := RunTool(context.Background(), runner, st, ToolRunOptions{
        RunID: "run-nmap", Tool: "nmap", Mode: "service", Target: "192.0.2.10", Ports: "3389", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
    })
    if err != nil {
        t.Fatal(err)
    }

    findings, err := st.ListFindings("run-nmap")
    if err != nil {
        t.Fatal(err)
    }
    if len(findings) != 1 || findings[0].Source != "manual-review" || findings[0].ID != "manual-review:CVE-2019-0708" {
        t.Fatalf("findings = %#v", findings)
    }
}

func TestRunToolNmapAliveSavesInfoFinding(t *testing.T) {
    st := newToolRunStore(t)
    runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
        return []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`), nil
    })

    err := RunTool(context.Background(), runner, st, ToolRunOptions{
        RunID: "run-alive", Tool: "nmap", Mode: "alive", Target: "192.0.2.10", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
    })
    if err != nil {
        t.Fatal(err)
    }

    findings, err := st.ListFindings("run-alive")
    if err != nil {
        t.Fatal(err)
    }
    if len(findings) != 1 || findings[0].ID != "host-alive" || !strings.Contains(findings[0].Summary, "alive") {
        t.Fatalf("findings = %#v", findings)
    }
}

func TestRunToolHttpxSavesWebFingerprint(t *testing.T) {
    st := newToolRunStore(t)
    runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
        return []byte(`{"url":"http://192.0.2.10:8080","status-code":200,"title":"Lab","tech":["nginx"]}` + "\n"), nil
    })

    err := RunTool(context.Background(), runner, st, ToolRunOptions{
        RunID: "run-httpx", Tool: "httpx", URL: "http://192.0.2.10:8080", Tools: ToolPaths{Httpx: "httpx"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
    })
    if err != nil {
        t.Fatal(err)
    }

    fps, err := st.ListFingerprints("run-httpx")
    if err != nil {
        t.Fatal(err)
    }
    if len(fps) != 1 || !fps[0].IsWeb || fps[0].Port != 8080 || fps[0].URL != "http://192.0.2.10:8080" {
        t.Fatalf("fingerprints = %#v", fps)
    }
}

func TestRunToolNucleiSavesFindings(t *testing.T) {
    st := newToolRunStore(t)
    runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
        return []byte(`{"template-id":"cve-test","info":{"name":"Example CVE","severity":"high"},"matched-at":"http://192.0.2.10:8080"}` + "\n"), nil
    })

    err := RunTool(context.Background(), runner, st, ToolRunOptions{
        RunID: "run-nuclei", Tool: "nuclei", URL: "http://192.0.2.10:8080", Tags: []string{"tomcat"}, Tools: ToolPaths{Nuclei: "nuclei"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
    })
    if err != nil {
        t.Fatal(err)
    }

    findings, err := st.ListFindings("run-nuclei")
    if err != nil {
        t.Fatal(err)
    }
    if len(findings) != 1 || findings[0].Source != "nuclei" || findings[0].ID != "cve-test" || findings[0].Severity != "high" {
        t.Fatalf("findings = %#v", findings)
    }
}
```

- [ ] **Step 2: Verify app tests fail**

Run:

```bash
go test ./internal/app -run 'TestRunTool' -count=1
```

Expected: FAIL with `undefined: RunTool` and `undefined: ToolRunOptions`.

- [ ] **Step 3: Implement minimal `RunTool`**

Create `/Users/kun/DEV/new-Anchor/internal/app/tool_run.go`:

```go
package app

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net"
    "net/url"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/P0m32Kun/anchorscan/internal/fingerprint"
    "github.com/P0m32Kun/anchorscan/internal/report"
    "github.com/P0m32Kun/anchorscan/internal/store"
    "github.com/P0m32Kun/anchorscan/internal/tools"
)

type ToolRunOptions struct {
    RunID          string
    ProjectID      string
    Tool           string
    Mode           string
    Target         string
    Ports          string
    URL            string
    Tags           []string
    Template       string
    Tools          ToolPaths
    ExtraArgs      ToolExtraArgs
    JSONReportPath string
    Logf           func(format string, args ...any)
}

func RunTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) (runErr error) {
    opts.Tool = strings.TrimSpace(opts.Tool)
    if opts.Mode == "" {
        opts.Mode = "service"
    }
    if opts.RunID == "" {
        return errors.New("tool run requires run id")
    }
    if scanStore == nil {
        return errors.New("tool run requires store")
    }
    if opts.JSONReportPath == "" {
        return errors.New("tool run requires json report path")
    }

    snapshot, _ := json.Marshal(opts)
    _ = scanStore.SaveScanRun(store.ScanRun{
        RunID: opts.RunID, ProjectID: opts.ProjectID, Target: coalesceTool(opts.Target, opts.URL), Ports: opts.Ports,
        Profile: "tool:" + opts.Tool, Status: "running", StartedAt: time.Now(), ConfigSnapshot: string(snapshot),
    })
    defer func() {
        status := "completed"
        message := ""
        if runErr != nil {
            status = "failed"
            message = runErr.Error()
            if errors.Is(runErr, context.Canceled) {
                status = "canceled"
            }
        }
        _ = scanStore.UpdateScanRunStatus(opts.RunID, status, message, time.Now())
    }()

    var fps []fingerprint.ServiceFingerprint
    var findings []report.Finding
    switch opts.Tool {
    case "rustscan":
        fps, runErr = runRustscanTool(ctx, runner, scanStore, opts)
    case "nmap":
        fps, findings, runErr = runNmapTool(ctx, runner, scanStore, opts)
    case "httpx":
        fps, findings, runErr = runHTTPXTool(ctx, runner, scanStore, opts)
    case "nuclei":
        findings, runErr = runNucleiTool(ctx, runner, scanStore, opts)
    default:
        runErr = fmt.Errorf("unknown tool: %s", opts.Tool)
    }
    if runErr != nil {
        return runErr
    }
    emitTool(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
    return report.WriteJSON(opts.JSONReportPath, report.Build(fps, findings))
}
```

Continue in the same file with helper functions from the test requirements:

```go
func runRustscanTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]fingerprint.ServiceFingerprint, error) {
    if strings.TrimSpace(opts.Target) == "" || strings.TrimSpace(opts.Ports) == "" {
        return nil, errors.New("rustscan requires target and ports")
    }
    emitTool(opts, scanStore, "info", "rustscan", "rustscan %s ports=%s", opts.Target, opts.Ports)
    ports, err := tools.DiscoverPorts(ctx, runner, opts.Tools.Rustscan, opts.Target, opts.Ports, opts.ExtraArgs.Rustscan)
    if err != nil {
        return nil, normalizeToolError(ctx, err)
    }
    out := make([]fingerprint.ServiceFingerprint, 0, len(ports))
    for _, port := range ports {
        fp := fingerprint.ServiceFingerprint{IP: opts.Target, Port: port, Protocol: "tcp"}
        if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
            return nil, err
        }
        out = append(out, fp)
    }
    return out, nil
}

func runNmapTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
    if strings.TrimSpace(opts.Target) == "" {
        return nil, nil, errors.New("nmap requires target")
    }
    if opts.Mode == "alive" {
        alive, err := tools.CheckAlive(ctx, runner, opts.Tools.Nmap, opts.Target, opts.ExtraArgs.Nmap)
        if err != nil {
            return nil, nil, normalizeToolError(ctx, err)
        }
        summary := "Host did not respond"
        if alive {
            summary = "Host is alive"
        }
        finding := report.Finding{IP: opts.Target, Source: "nmap", ID: "host-alive", Severity: "info", Summary: summary, Target: opts.Target, Output: summary}
        return nil, []report.Finding{finding}, scanStore.SaveFinding(opts.RunID, finding)
    }
    if strings.TrimSpace(opts.Ports) == "" {
        return nil, nil, errors.New("nmap service mode requires ports")
    }
    ports, err := parsePortCSV(opts.Ports)
    if err != nil {
        return nil, nil, err
    }
    fps, err := tools.Fingerprint(ctx, runner, opts.Tools.Nmap, opts.Target, ports, opts.ExtraArgs.Nmap)
    if err != nil {
        return nil, nil, normalizeToolError(ctx, err)
    }
    var findings []report.Finding
    for _, fp := range fps {
        if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
            return nil, nil, err
        }
        for _, finding := range ManualReviewFindings(fp) {
            if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
                return nil, nil, err
            }
            findings = append(findings, finding)
        }
    }
    return fps, findings, nil
}
```

Add the remaining helpers:

```go
func runHTTPXTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
    if strings.TrimSpace(opts.URL) == "" {
        return nil, nil, errors.New("httpx requires url")
    }
    fp, err := fingerprintFromURL(opts.URL)
    if err != nil {
        return nil, nil, err
    }
    result, err := tools.EnrichWeb(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
    if err != nil {
        return nil, nil, normalizeToolError(ctx, err)
    }
    if result.URL != "" {
        fp.URL = result.URL
    }
    fp.IsWeb = true
    fp.Product = strings.Join(result.Tech, ", ")
    fp.Normalized = fp.Product
    if err := scanStore.SaveFingerprint(opts.RunID, fp); err != nil {
        return nil, nil, err
    }
    finding := report.Finding{IP: fp.IP, Port: fp.Port, Source: "httpx", ID: "web-fingerprint", Severity: "info", Summary: result.Title, Target: fp.URL, Output: fmt.Sprintf("status=%d tech=%s title=%s", result.StatusCode, strings.Join(result.Tech, ","), result.Title)}
    return []fingerprint.ServiceFingerprint{fp}, []report.Finding{finding}, scanStore.SaveFinding(opts.RunID, finding)
}

func runNucleiTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) ([]report.Finding, error) {
    if strings.TrimSpace(opts.URL) == "" {
        return nil, errors.New("nuclei requires url")
    }
    var out []byte
    var err error
    if strings.TrimSpace(opts.Template) != "" {
        out, err = tools.RunNucleiTemplate(ctx, runner, opts.Tools.Nuclei, opts.URL, opts.Template, opts.ExtraArgs.Nuclei)
    } else {
        if len(opts.Tags) == 0 {
            return nil, errors.New("nuclei requires tags or template")
        }
        out, err = tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, opts.URL, opts.Tags, opts.ExtraArgs.Nuclei)
    }
    if err != nil {
        return nil, normalizeToolError(ctx, err)
    }
    parsed, err := tools.ParseNucleiJSONL(out)
    if err != nil {
        return nil, err
    }
    fp, _ := fingerprintFromURL(opts.URL)
    findings := make([]report.Finding, 0, len(parsed))
    for _, result := range parsed {
        finding := report.Finding{IP: fp.IP, Port: fp.Port, Source: "nuclei", ID: result.TemplateID, Severity: result.Severity, Summary: result.Name, Target: result.MatchedAt, Output: formatNucleiEvidence(result)}
        if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
            return nil, err
        }
        findings = append(findings, finding)
    }
    return findings, nil
}
```

And small local helpers:

```go
func fingerprintFromURL(raw string) (fingerprint.ServiceFingerprint, error) {
    parsed, err := url.Parse(raw)
    if err != nil {
        return fingerprint.ServiceFingerprint{}, err
    }
    host := parsed.Hostname()
    port := parsed.Port()
    if port == "" && parsed.Scheme == "https" {
        port = "443"
    }
    if port == "" {
        port = "80"
    }
    portNum, err := strconv.Atoi(port)
    if err != nil {
        return fingerprint.ServiceFingerprint{}, err
    }
    return fingerprint.Classify(fingerprint.ServiceFingerprint{IP: host, Port: portNum, Protocol: "tcp", Service: "http", IsWeb: true, URL: raw}), nil
}

func parsePortCSV(value string) ([]int, error) {
    parts := strings.Split(value, ",")
    ports := make([]int, 0, len(parts))
    for _, part := range parts {
        port, err := strconv.Atoi(strings.TrimSpace(part))
        if err != nil || port < 1 || port > 65535 {
            return nil, fmt.Errorf("invalid port: %s", part)
        }
        ports = append(ports, port)
    }
    return ports, nil
}

func coalesceTool(values ...string) string {
    for _, value := range values {
        if strings.TrimSpace(value) != "" {
            return value
        }
    }
    return ""
}

func emitTool(opts ToolRunOptions, scanStore *store.Store, level string, stage string, format string, args ...any) {
    message := fmt.Sprintf(format, args...)
    if opts.Logf != nil {
        opts.Logf("%s", message)
    }
    _ = scanStore.AppendScanEvent(store.ScanEvent{RunID: opts.RunID, Time: time.Now(), Level: level, Stage: stage, Message: message})
}

var _ = filepath.Join
var _ = net.JoinHostPort
```

Remove the two `var _` lines if imports do not require them after implementation.

- [ ] **Step 4: Verify app tests pass**

Run:

```bash
go test ./internal/app -run 'TestRunTool' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/app/tool_run.go internal/app/tool_run_test.go
git commit -m "feat: persist single tool runs"
```

---

### Task 3: Manual-review BlueKeep rule in single-tool and pipeline paths

**Files:**
- Create: `/Users/kun/DEV/new-Anchor/internal/app/manual_review.go`
- Create: `/Users/kun/DEV/new-Anchor/internal/app/manual_review_test.go`
- Modify: `/Users/kun/DEV/new-Anchor/internal/app/scan.go`

**Interfaces:**
- Produces: `app.ManualReviewFindings(fp fingerprint.ServiceFingerprint) []report.Finding`
- Consumed by: `RunTool` nmap service mode and existing `scanTarget`

- [ ] **Step 1: Write failing BlueKeep tests**

Create `/Users/kun/DEV/new-Anchor/internal/app/manual_review_test.go`:

```go
package app

import (
    "strings"
    "testing"

    "github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestManualReviewFindingsAddsBlueKeepForRDP3389(t *testing.T) {
    findings := ManualReviewFindings(fingerprint.ServiceFingerprint{IP: "192.0.2.10", Port: 3389, Service: "ms-wbt-server", Product: "Microsoft Terminal Services"})

    if len(findings) != 1 {
        t.Fatalf("findings = %#v", findings)
    }
    got := findings[0]
    if got.Source != "manual-review" || got.ID != "manual-review:CVE-2019-0708" || got.Severity != "critical" {
        t.Fatalf("finding = %#v", got)
    }
    if !strings.Contains(got.Output, "external validation") {
        t.Fatalf("output = %q", got.Output)
    }
}

func TestManualReviewFindingsSkipsNonRDP(t *testing.T) {
    findings := ManualReviewFindings(fingerprint.ServiceFingerprint{IP: "192.0.2.10", Port: 22, Service: "ssh"})
    if len(findings) != 0 {
        t.Fatalf("findings = %#v", findings)
    }
}
```

- [ ] **Step 2: Verify BlueKeep tests fail**

Run:

```bash
go test ./internal/app -run 'TestManualReviewFindings' -count=1
```

Expected: FAIL with `undefined: ManualReviewFindings`.

- [ ] **Step 3: Implement manual-review rule**

Create `/Users/kun/DEV/new-Anchor/internal/app/manual_review.go`:

```go
package app

import (
    "fmt"
    "strings"

    "github.com/P0m32Kun/anchorscan/internal/fingerprint"
    "github.com/P0m32Kun/anchorscan/internal/report"
)

func ManualReviewFindings(fp fingerprint.ServiceFingerprint) []report.Finding {
    service := strings.ToLower(fp.Service + " " + fp.Normalized + " " + fp.Product)
    if fp.Port != 3389 || !(strings.Contains(service, "rdp") || strings.Contains(service, "ms-wbt-server")) {
        return nil
    }
    return []report.Finding{{
        IP: fp.IP, Port: fp.Port, Source: "manual-review", ID: "manual-review:CVE-2019-0708", Severity: "critical",
        Summary: "RDP service requires BlueKeep verification", Target: fmt.Sprintf("%s:%d", fp.IP, fp.Port),
        Output: fmt.Sprintf("RDP-like service detected on %s:%d (%s %s). BlueKeep requires external validation; AnchorScan does not bundle Metasploit or run exploit verification.", fp.IP, fp.Port, fp.Service, fp.Product),
    }}
}
```

- [ ] **Step 4: Verify BlueKeep tests pass**

Run:

```bash
go test ./internal/app -run 'TestManualReviewFindings' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing pipeline integration test**

Add to `/Users/kun/DEV/new-Anchor/internal/app/scan_test.go`:

```go
func TestRunScanAddsManualReviewForRDP(t *testing.T) {
    st, err := store.Open(filepath.Join(t.TempDir(), "scans.sqlite"))
    if err != nil {
        t.Fatal(err)
    }
    runner := fakeRunner{outputs: [][]byte{
        []byte("[3389]"),
        []byte(`<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="3389"><state state="open"/><service name="ms-wbt-server" product="Microsoft Terminal Services"/></port></ports></host></nmaprun>`),
    }}
    jsonPath := filepath.Join(t.TempDir(), "report.json")

    err = RunScan(context.Background(), &runner, st, ScanOptions{
        RunID: "run-bluekeep", Targets: []string{"192.0.2.10"}, Ports: "3389", Tools: ToolPaths{Rustscan: "rustscan", Nmap: "nmap"}, JSONReportPath: jsonPath,
    })
    if err != nil {
        t.Fatal(err)
    }
    findings, err := st.ListFindings("run-bluekeep")
    if err != nil {
        t.Fatal(err)
    }
    if len(findings) != 1 || findings[0].ID != "manual-review:CVE-2019-0708" {
        t.Fatalf("findings = %#v", findings)
    }
}
```

Adjust the fake runner construction to match the existing `scan_test.go` helper names if they differ.

- [ ] **Step 6: Verify pipeline test fails**

Run:

```bash
go test ./internal/app -run 'TestRunScanAddsManualReviewForRDP' -count=1
```

Expected: FAIL because no manual-review finding is saved by the pipeline yet.

- [ ] **Step 7: Wire manual-review into pipeline**

In `/Users/kun/DEV/new-Anchor/internal/app/scan.go`, immediately after saving each fingerprint and appending it to `allFingerprints`, add:

```go
for _, finding := range ManualReviewFindings(fp) {
    if err := scanStore.SaveFinding(opts.RunID, finding); err != nil {
        return nil, nil, err
    }
    allFindings = append(allFindings, finding)
}
```

Place this before NSE/nuclei matching so manual-review findings are independent of external scripts.

- [ ] **Step 8: Verify app tests pass**

Run:

```bash
go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit Task 3**

```bash
git add internal/app/manual_review.go internal/app/manual_review_test.go internal/app/scan.go internal/app/scan_test.go
git commit -m "feat: flag BlueKeep manual review"
```

---

### Task 4: CLI `anchorscan tool <name>`

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/cmd/anchorscan/main.go`
- Modify or create: `/Users/kun/DEV/new-Anchor/cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: `app.RunTool`, `app.ToolRunOptions`
- Produces: CLI command `tool rustscan|nmap|httpx|nuclei`

- [ ] **Step 1: Write failing CLI validation tests**

Add to `/Users/kun/DEV/new-Anchor/cmd/anchorscan/main_test.go`:

```go
func TestRunToolRequiresSubcommand(t *testing.T) {
    err := run([]string{"tool"}, io.Discard, io.Discard, cliDeps{})
    if err == nil || !strings.Contains(err.Error(), "usage: anchorscan tool") {
        t.Fatalf("err = %v", err)
    }
}

func TestRunToolNucleiRejectsMissingTagsAndTemplate(t *testing.T) {
    dir := t.TempDir()
    configPath := writeCLIConfig(t, dir)
    err := run([]string{"tool", "nuclei", "--config", configPath, "--db", filepath.Join(dir, "scans.sqlite"), "--url", "http://example.test"}, io.Discard, io.Discard, cliDeps{now: fixedNow})
    if err == nil || !strings.Contains(err.Error(), "nuclei requires tags or template") {
        t.Fatalf("err = %v", err)
    }
}
```

If `writeCLIConfig` or `fixedNow` do not exist, add local test helpers in the same file:

```go
func fixedNow() time.Time { return time.Date(2026, 7, 8, 1, 2, 3, 0, time.UTC) }

func writeCLIConfig(t *testing.T, dir string) string {
    t.Helper()
    path := filepath.Join(dir, "config.yaml")
    data := []byte("tools:\n  rustscan: rustscan\n  nmap: nmap\n  httpx: httpx\n  nuclei: nuclei\nscan:\n  ports: 80\n  profile: normal\n")
    if err := os.WriteFile(path, data, 0o600); err != nil {
        t.Fatal(err)
    }
    return path
}
```

- [ ] **Step 2: Verify CLI validation tests fail**

Run:

```bash
go test ./cmd/anchorscan -run 'TestRunTool' -count=1
```

Expected: FAIL because `tool` is unknown or validation is missing.

- [ ] **Step 3: Implement CLI command parsing**

In `/Users/kun/DEV/new-Anchor/cmd/anchorscan/main.go`, add a switch case:

```go
case "tool":
    return runTool(args[1:], stdout, stderr, deps)
```

Add `runTool`:

```go
func runTool(args []string, stdout io.Writer, stderr io.Writer, deps cliDeps) error {
    if len(args) == 0 || isHelpRequest(args[0]) {
        printToolHelp(stdout)
        if len(args) == 0 {
            return errors.New("usage: anchorscan tool <rustscan|nmap|httpx|nuclei>")
        }
        return nil
    }
    toolName := args[0]
    fs := flag.NewFlagSet("tool "+toolName, flag.ContinueOnError)
    fs.SetOutput(io.Discard)
    configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
    dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
    jsonPath := fs.String("json", "", "path to JSON report output")
    projectID := fs.String("project", "", "project id")
    targetValue := fs.String("target", "", "target host")
    urlValue := fs.String("url", "", "target URL")
    portsValue := fs.String("ports", "", "port csv or range for rustscan")
    modeValue := fs.String("mode", "service", "nmap mode: service or alive")
    tagsValue := fs.String("tags", "", "nuclei tags csv")
    templateValue := fs.String("template", "", "nuclei template path")
    extraArgsValue := fs.String("args", "", "extra tool args")
    if err := fs.Parse(args[1:]); err != nil {
        return err
    }

    cfg, err := config.Load(*configPath)
    if err != nil {
        return err
    }
    if *jsonPath == "" {
        *jsonPath = filepath.Join("reports", "tool-"+toolName+"-"+deps.now().Format("20060102-150405")+".json")
    }
    if err := ensureParentDir(*dbPath); err != nil {
        return err
    }
    if err := ensureParentDir(*jsonPath); err != nil {
        return err
    }
    scanStore, err := deps.openStore(*dbPath)
    if err != nil {
        return err
    }
    runID := "tool-" + toolName + "-" + deps.now().Format("20060102-150405")
    opts := app.ToolRunOptions{
        RunID: runID, ProjectID: *projectID, Tool: toolName, Mode: *modeValue, Target: *targetValue, Ports: *portsValue, URL: *urlValue,
        Tags: splitCSV(*tagsValue), Template: *templateValue,
        Tools: app.ToolPaths{Rustscan: cfg.Tools.Rustscan, Nmap: cfg.Tools.Nmap, Httpx: cfg.Tools.Httpx, Nuclei: cfg.Tools.Nuclei},
        JSONReportPath: *jsonPath,
        Logf: func(format string, args ...any) { logScan(stderr, format, args...) },
    }
    applyToolExtraArgs(&opts, toolName, *extraArgsValue)
    if err := app.RunTool(context.Background(), deps.newRunner(), scanStore, opts); err != nil {
        return err
    }
    _, _ = fmt.Fprintf(stdout, "run_id=%s\njson=%s\n", runID, *jsonPath)
    return nil
}
```

Add helpers:

```go
func splitCSV(value string) []string {
    var out []string
    for _, part := range strings.Split(value, ",") {
        if item := strings.TrimSpace(part); item != "" {
            out = append(out, item)
        }
    }
    return out
}

func applyToolExtraArgs(opts *app.ToolRunOptions, toolName string, value string) {
    args := config.SplitArgs(value)
    switch toolName {
    case "rustscan":
        opts.ExtraArgs.Rustscan = args
    case "nmap":
        opts.ExtraArgs.Nmap = args
    case "httpx":
        opts.ExtraArgs.Httpx = args
    case "nuclei":
        opts.ExtraArgs.Nuclei = args
    }
}
```

If `config.SplitArgs` is not exported, reuse the existing exported config resolution path or add a small local `strings.Fields` helper for CLI `--args`; do not add shell parsing complexity.

- [ ] **Step 4: Add help text**

Add `printToolHelp(stdout)` near existing help functions:

```go
func printToolHelp(w io.Writer) {
    _, _ = fmt.Fprintln(w, "usage: anchorscan tool <rustscan|nmap|httpx|nuclei> [options]")
    _, _ = fmt.Fprintln(w, "examples:")
    _, _ = fmt.Fprintln(w, "  anchorscan tool rustscan --target 192.168.1.10 --ports 80,443")
    _, _ = fmt.Fprintln(w, "  anchorscan tool nmap --target 192.168.1.10 --ports 80,443")
    _, _ = fmt.Fprintln(w, "  anchorscan tool nmap --target 192.168.1.10 --mode alive")
    _, _ = fmt.Fprintln(w, "  anchorscan tool httpx --url http://192.168.1.10:8080")
    _, _ = fmt.Fprintln(w, "  anchorscan tool nuclei --url http://192.168.1.10:8080 --tags tomcat")
}
```

Update root help to mention `tool`.

- [ ] **Step 5: Verify CLI tests pass**

Run:

```bash
go test ./cmd/anchorscan -run 'TestRunTool' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run CLI package tests**

Run:

```bash
go test ./cmd/anchorscan -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
git add cmd/anchorscan/main.go cmd/anchorscan/main_test.go
git commit -m "feat: add single tool CLI command"
```

---

### Task 5: Web Console single-tool page

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/internal/app/manager.go`
- Modify: `/Users/kun/DEV/new-Anchor/internal/app/manager_test.go`
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/server.go`
- Create: `/Users/kun/DEV/new-Anchor/internal/web/templates/tool_new.html`
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/templates/base.html`
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/server_test.go`

**Interfaces:**
- Consumes: `app.RunTool`, `app.ToolRunOptions`
- Produces: Web routes `GET /tools/new` and `POST /tools`

- [ ] **Step 1: Write failing manager test**

Add to `/Users/kun/DEV/new-Anchor/internal/app/manager_test.go`:

```go
func TestManagerStartToolRejectsSecondActiveRun(t *testing.T) {
    st, err := store.Open(filepath.Join(t.TempDir(), "scans.sqlite"))
    if err != nil {
        t.Fatal(err)
    }
    runner := blockingRunner{block: make(chan struct{})}
    manager := NewManager(&runner, st)

    _, err = manager.StartTool(context.Background(), ToolRunOptions{RunID: "tool-1", Tool: "nmap", Mode: "alive", Target: "192.0.2.10", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "one.json")})
    if err != nil {
        t.Fatal(err)
    }
    if _, err := manager.StartTool(context.Background(), ToolRunOptions{RunID: "tool-2"}); err == nil {
        t.Fatal("expected active run error")
    }
    manager.Cancel("tool-1")
    close(runner.block)
}
```

Reuse or adapt existing `blockingRunner` from the file. If none exists, add the smallest fake runner that blocks until the channel closes.

- [ ] **Step 2: Verify manager test fails**

Run:

```bash
go test ./internal/app -run 'TestManagerStartTool' -count=1
```

Expected: FAIL with `manager.StartTool undefined`.

- [ ] **Step 3: Implement `StartTool`**

Add to `/Users/kun/DEV/new-Anchor/internal/app/manager.go`:

```go
func (m *Manager) StartTool(ctx context.Context, opts ToolRunOptions) (string, error) {
    m.mu.Lock()
    if m.activeID != "" {
        m.mu.Unlock()
        return "", errors.New("scan already running")
    }
    runCtx, cancel := context.WithCancel(ctx)
    m.activeID = opts.RunID
    m.cancel = cancel
    m.mu.Unlock()

    go func() {
        _ = RunTool(runCtx, m.runner, m.store, opts)
        m.mu.Lock()
        m.activeID = ""
        m.cancel = nil
        m.mu.Unlock()
    }()
    return opts.RunID, nil
}
```

- [ ] **Step 4: Verify manager test passes**

Run:

```bash
go test ./internal/app -run 'TestManagerStartTool' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing Web route tests**

Add to `/Users/kun/DEV/new-Anchor/internal/web/server_test.go`:

```go
func TestToolNewPageRenders(t *testing.T) {
    handler := newTestServer(t)
    req := httptest.NewRequest(http.MethodGet, "/tools/new", nil)
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), "rustscan") || !strings.Contains(rec.Body.String(), "nuclei") {
        t.Fatalf("body = %s", rec.Body.String())
    }
}

func TestToolCreateStartsRun(t *testing.T) {
    handler := newTestServer(t)
    form := url.Values{}
    form.Set("tool", "nmap")
    form.Set("mode", "alive")
    form.Set("target", "192.0.2.10")

    req := httptest.NewRequest(http.MethodPost, "/tools", strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusSeeOther {
        t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
    }
    if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/runs/tool-nmap-") {
        t.Fatalf("Location = %q", loc)
    }
}
```

Adapt `newTestServer` to the actual helper in `server_test.go`. If the current fake runner has fixed outputs, make it return nmap alive XML for the nmap alive route.

- [ ] **Step 6: Verify Web route tests fail**

Run:

```bash
go test ./internal/web -run 'TestTool' -count=1
```

Expected: FAIL because routes/template are missing.

- [ ] **Step 7: Add routes**

In `/Users/kun/DEV/new-Anchor/internal/web/server.go`, register routes in `NewServer`:

```go
mux.HandleFunc("/tools/new", s.toolNew)
mux.HandleFunc("/tools", s.toolCreate)
```

Add handlers:

```go
func (s *server) toolNew(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    projects, err := s.store.ListProjects()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    render(w, "templates/tool_new.html", map[string]any{"Projects": projects})
}

func (s *server) toolCreate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    if err := r.ParseForm(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    cfg, err := config.Load(s.opts.ConfigPath)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    toolName := strings.TrimSpace(r.FormValue("tool"))
    runID := newID("tool-"+toolName, s.opts.Now())
    projectID := r.FormValue("project_id")
    jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)
    if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    opts := app.ToolRunOptions{
        RunID: runID, ProjectID: projectID, Tool: toolName, Mode: r.FormValue("mode"), Target: strings.TrimSpace(r.FormValue("target")), Ports: strings.TrimSpace(r.FormValue("ports")), URL: strings.TrimSpace(r.FormValue("url")),
        Tags: splitCSV(r.FormValue("tags")), Template: strings.TrimSpace(r.FormValue("template")),
        Tools: app.ToolPaths{Rustscan: cfg.Tools.Rustscan, Nmap: cfg.Tools.Nmap, Httpx: cfg.Tools.Httpx, Nuclei: cfg.Tools.Nuclei},
        JSONReportPath: jsonPath,
    }
    applyWebToolArgs(&opts, r.FormValue("extra_args"))
    if _, err := s.manager.StartTool(context.Background(), opts); err != nil {
        http.Error(w, err.Error(), http.StatusConflict)
        return
    }
    http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
}
```

Add local helpers if not already available:

```go
func splitCSV(value string) []string {
    var out []string
    for _, part := range strings.Split(value, ",") {
        if item := strings.TrimSpace(part); item != "" {
            out = append(out, item)
        }
    }
    return out
}

func applyWebToolArgs(opts *app.ToolRunOptions, value string) {
    args := strings.Fields(value)
    switch opts.Tool {
    case "rustscan":
        opts.ExtraArgs.Rustscan = args
    case "nmap":
        opts.ExtraArgs.Nmap = args
    case "httpx":
        opts.ExtraArgs.Httpx = args
    case "nuclei":
        opts.ExtraArgs.Nuclei = args
    }
}
```

- [ ] **Step 8: Add template**

Create `/Users/kun/DEV/new-Anchor/internal/web/templates/tool_new.html`:

```html
{{define "content"}}
<section class="panel">
  <div class="section-title">
    <div>
      <p class="eyebrow">Manual tool</p>
      <h1>Single tool run</h1>
    </div>
  </div>
  <form method="post" action="/tools" class="form-grid">
    <label>Project
      <select name="project_id">
        <option value="">No project</option>
        {{range .Projects}}<option value="{{.ID}}">{{.Name}}</option>{{end}}
      </select>
    </label>
    <label>Tool
      <select name="tool">
        <option value="rustscan">rustscan</option>
        <option value="nmap">nmap</option>
        <option value="httpx">httpx</option>
        <option value="nuclei">nuclei</option>
      </select>
    </label>
    <label>Nmap mode
      <select name="mode">
        <option value="service">service fingerprint</option>
        <option value="alive">host alive</option>
      </select>
    </label>
    <label>Target host <input name="target" value=""></label>
    <label>URL <input name="url" value=""></label>
    <label>Ports <input name="ports" value=""></label>
    <label>Nuclei tags <input name="tags" value=""></label>
    <label>Nuclei template <input name="template" value=""></label>
    <label>Extra args <input name="extra_args" value=""></label>
    <button type="submit">Start tool run</button>
  </form>
</section>
{{end}}
```

In `/Users/kun/DEV/new-Anchor/internal/web/templates/base.html`, add one nav link near existing scan/run links:

```html
<a href="/tools/new">Single tool</a>
```

- [ ] **Step 9: Verify Web tests pass**

Run:

```bash
go test ./internal/web -run 'TestTool' -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit Task 5**

```bash
git add internal/app/manager.go internal/app/manager_test.go internal/web/server.go internal/web/server_test.go internal/web/templates/tool_new.html internal/web/templates/base.html
git commit -m "feat: add single tool web page"
```

---

### Task 6: Documentation and full verification

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/README.md`
- Modify if needed: `/Users/kun/DEV/new-Anchor/docs/project-status.md`

**Interfaces:**
- Consumes: completed CLI and Web behavior from Tasks 1-5
- Produces: user-facing usage docs

- [ ] **Step 1: Write documentation change**

Add to `/Users/kun/DEV/new-Anchor/README.md` after the Quick Start or Web Console section:

```markdown
## Single Tool Runs

Use `anchorscan tool` when you want to run one scanner without the full pipeline:

```bash
anchorscan tool rustscan --config config/default.yaml --target 127.0.0.1 --ports 80,443
anchorscan tool nmap --config config/default.yaml --target 127.0.0.1 --ports 80,443
anchorscan tool nmap --config config/default.yaml --target 127.0.0.1 --mode alive
anchorscan tool httpx --config config/default.yaml --url http://127.0.0.1:8080
anchorscan tool nuclei --config config/default.yaml --url http://127.0.0.1:8080 --tags tech
```

Single tool runs are stored in SQLite and appear in the same run history and report pages as pipeline scans.

Metasploit-backed checks are not bundled. When AnchorScan sees services that require heavy external verification, such as RDP on 3389 for BlueKeep follow-up, it records a `manual-review` finding instead of claiming exploit confirmation.
```

Update `/Users/kun/DEV/new-Anchor/docs/project-status.md` only if the feature is complete on the branch. Add a short V1.3 baseline bullet; do not log implementation details.

- [ ] **Step 2: Run targeted package tests**

Run:

```bash
go test ./internal/tools ./internal/app ./cmd/anchorscan ./internal/web -count=1
```

Expected: PASS.

- [ ] **Step 3: Run all tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Manual CLI smoke with fake-safe inputs**

Run help only if local tools are unavailable:

```bash
go run ./cmd/anchorscan tool --help
```

Expected: output includes `anchorscan tool <rustscan|nmap|httpx|nuclei>`.

If local tools are configured, run one real safe command:

```bash
go run ./cmd/anchorscan tool nmap --config config/default.yaml --target 127.0.0.1 --mode alive --db data/scans.sqlite --json reports/tool-alive.json
```

Expected: stdout prints `run_id=` and `json=reports/tool-alive.json`.

- [ ] **Step 5: Commit Task 6**

```bash
git add README.md docs/project-status.md
git commit -m "docs: document single tool runs"
```

---

## Self-Review Checklist

- Spec coverage:
  - CLI single tool runs: Task 4.
  - Web Console single tool page: Task 5.
  - Shared app runner and persistence: Task 2.
  - Existing reports and JSON output: Task 2.
  - nmap alive mode: Tasks 1 and 2.
  - nuclei tags/template mode: Tasks 1 and 2.
  - BlueKeep manual-review behavior: Task 3.
  - No Metasploit bundling or exploit checks: Global constraints and Task 6 docs.
- Red-flag scan: no unfinished implementation steps are allowed before execution.
- Type consistency:
  - `ToolRunOptions` includes `Tools ToolPaths` and `ExtraArgs ToolExtraArgs`, matching existing app types.
  - `RunTool` signature is consumed consistently by CLI and Web.
  - `StartTool` mirrors existing `Manager.Start` locking behavior.
