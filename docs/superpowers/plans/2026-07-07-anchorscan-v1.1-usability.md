# AnchorScan v1.1 Usability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build v1.1 usability improvements: scan profiles, fine-grained tool args, host worker control, doctor checks, local single-user Web Console, project/run management, progress logs, cancellation, and report viewing.

**Architecture:** Keep the V1 scan pipeline unchanged: `rustscan -> nmap -sV -> fingerprint classification -> httpx / NSE / nuclei -> SQLite -> reports`. Add configuration resolution before scanning, event/run persistence around scanning, and a local `net/http` Web Console that calls the same app-layer scan code. Use SQLite as the only state store.

**Tech Stack:** Go standard library, `html/template`, `net/http`, SQLite through existing `modernc.org/sqlite`, YAML through existing `gopkg.in/yaml.v3`, existing external tools configured by path.

## Global Constraints

- Web Console is local single-user only; default listen address is `127.0.0.1:8088`.
- Do not add login, users, roles, permissions, distributed nodes, remote agents, knowledge base, or remediation features.
- Do not introduce React, Vue, or frontend build tooling.
- Preserve V1 CLI scan/report compatibility.
- Preserve V1 fixed scan pipeline order.
- Use TDD: write a failing test before production changes for each task.
- Keep each commit small and task-scoped.
- Before implementing Task 1, commit or stash the current V1 baseline so v1.1 diffs are reviewable.

---

## File Structure

### Existing Files To Modify

- `config/default.yaml` — add default profile and profile args.
- `cmd/anchorscan/main.go` — add CLI flags and commands: `doctor`, `web`, `cancel`.
- `cmd/anchorscan/main_test.go` — CLI behavior tests.
- `internal/config/config.go` — parse profiles and resolve effective scan settings.
- `internal/config/config_test.go` — config/profile/arg parsing tests.
- `internal/app/scan.go` — consume effective settings, emit persisted events, manage run lifecycle, host workers.
- `internal/app/scan_test.go` — scan behavior tests.
- `internal/store/sqlite.go` — schema migration entry point.
- `internal/store/sqlite_test.go` — store integration tests.
- `README.md` — v1.1 usage docs.
- `docs/testing-lab-checklist.md` — Web/profile lab steps.
- `docs/troubleshooting-lab.md` — v1.1 troubleshooting.

### New Files To Create

- `internal/config/args.go` — shell-like arg splitting for extra tool args.
- `internal/config/profile.go` — effective profile resolution.
- `internal/store/models.go` — `Project`, `ScanRun`, `ScanEvent` structs.
- `internal/store/projects.go` — project CRUD methods.
- `internal/store/runs.go` — scan run and event methods.
- `internal/doctor/doctor.go` — local readiness checks.
- `internal/doctor/doctor_test.go` — doctor tests.
- `internal/app/manager.go` — single running Web scan manager and cancellation.
- `internal/app/manager_test.go` — manager tests.
- `internal/web/server.go` — HTTP server construction and routes.
- `internal/web/server_test.go` — handler tests.
- `internal/web/templates.go` — embedded template parsing.
- `internal/web/templates/*.html` — local Web Console pages.
- `internal/web/static/style.css` — small CSS file.
- `internal/web/static/app.js` — polling JS for run progress.
- `internal/web/reports.go` — report view model and filters.
- `internal/web/reports_test.go` — report filtering tests.
- `internal/config/write.go` — safe config save with timestamped backup.
- `internal/config/write_test.go` — config backup tests.

---

## Interfaces Introduced By This Plan

```go
package config

type ToolArgs struct {
	Rustscan []string `yaml:"rustscan_args"`
	Nmap     []string `yaml:"nmap_args"`
	Httpx    []string `yaml:"httpx_args"`
	Nuclei   []string `yaml:"nuclei_args"`
}

type Profile struct {
	HostWorkers int `yaml:"host_workers"`
	ToolArgs    `yaml:",inline"`
}

type Overrides struct {
	ProfileName  string
	HostWorkers int
	RustscanArgs string
	NmapArgs     string
	HttpxArgs    string
	NucleiArgs   string
}

type EffectiveScan struct {
	ProfileName string
	HostWorkers int
	ToolArgs    ToolArgs
}

func SplitArgs(input string) ([]string, error)
func ResolveScan(cfg Config, overrides Overrides) (EffectiveScan, error)
func SaveWithBackup(path string, cfg Config, now time.Time) (backupPath string, err error)
```

```go
package store

type Project struct {
	ID             string
	Name           string
	Description    string
	DefaultTargets string
	DefaultPorts   string
	DefaultProfile string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ScanRun struct {
	RunID          string
	ProjectID      string
	Target         string
	Ports          string
	Profile        string
	Status         string
	StartedAt      time.Time
	FinishedAt     time.Time
	Error          string
	ConfigSnapshot string
}

type ScanEvent struct {
	ID      int64
	RunID   string
	Time    time.Time
	Level   string
	Stage   string
	Message string
}
```

```go
package doctor

type Check struct {
	Name    string
	OK      bool
	Message string
}

type Options struct {
	ConfigPath string
	DBPath     string
	ReportDir  string
}

func Run(opts Options) []Check
func HasFailures(checks []Check) bool
```

```go
package app

type ToolExtraArgs struct {
	Rustscan []string
	Nmap     []string
	Httpx    []string
	Nuclei   []string
}

// added to existing ScanOptions
// ProfileName string
// HostWorkers int
// ExtraArgs ToolExtraArgs
// ProjectID string
// ConfigSnapshot string

type Manager struct { /* one active scan at a time */ }
func NewManager(runner tools.Runner, scanStore *store.Store) *Manager
func (m *Manager) Start(ctx context.Context, opts ScanOptions) (string, error)
func (m *Manager) Cancel(runID string) error
func (m *Manager) ActiveRunID() string
```

```go
package web

type ServerOptions struct {
	ConfigPath string
	DBPath     string
	Listen     string
	Runner     tools.Runner
	Now        func() time.Time
}

func NewServer(opts ServerOptions) (http.Handler, error)
```

---

### Task 0: Baseline Safety Commit

**Files:**
- Check: all V1 files

**Interfaces:**
- Consumes: current V1 implementation
- Produces: clean Git baseline before v1.1 work

- [ ] **Step 1: Inspect untracked state**

Run:

```bash
git status --short
```

Expected: shows V1 project files. If files are still untracked, continue.

- [ ] **Step 2: Run baseline tests**

Run:

```bash
go test ./...
```

Expected: PASS for all packages.

- [ ] **Step 3: Commit V1 baseline**

Run:

```bash
git add README.md cmd config docker-compose.lab.yml docs go.mod go.sum internal
git commit -m "feat: add anchorscan v1 mvp"
```

Expected: a commit containing V1 code and docs. Do not include generated `data/` or `reports/` files.

---

### Task 1: Parse Scan Profiles And Extra Args

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/profile.go`
- Create: `internal/config/args.go`
- Modify: `internal/config/config_test.go`
- Modify: `config/default.yaml`

**Interfaces:**
- Consumes: existing `config.Load(path string) (Config, error)`
- Produces: `config.ResolveScan(cfg Config, overrides config.Overrides) (config.EffectiveScan, error)` and `config.SplitArgs(input string) ([]string, error)`

- [ ] **Step 1: Write failing profile parsing test**

Add to `internal/config/config_test.go`:

```go
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
  ports: top100
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
```

- [ ] **Step 2: Write failing default profile test**

Add to `internal/config/config_test.go`:

```go
func TestLoadDefaultsProfileToNormal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\nscan:\n  ports: top100\n")
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
```

- [ ] **Step 3: Write failing arg splitter test**

Create `internal/config/args_test.go`:

```go
package config

import (
	"reflect"
	"testing"
)

func TestSplitArgsHonorsQuotes(t *testing.T) {
	got, err := SplitArgs(`-T2 --script "redis-info,mysql-info" --max-retries 3`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}
	want := []string{"-T2", "--script", "redis-info,mysql-info", "--max-retries", "3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %#v want %#v", got, want)
	}
}

func TestSplitArgsRejectsUnclosedQuote(t *testing.T) {
	_, err := SplitArgs(`-T2 "broken`)
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 4: Run tests and verify failure**

Run:

```bash
go test ./internal/config -run 'TestLoadParsesProfiles|TestLoadDefaultsProfileToNormal|TestSplitArgs' -count=1
```

Expected: FAIL because `Profile`, `Scan.Profile`, and `SplitArgs` do not exist.

- [ ] **Step 5: Implement minimal profile types**

Update `internal/config/config.go` with:

```go
type ToolArgs struct {
	Rustscan []string `yaml:"rustscan_args"`
	Nmap     []string `yaml:"nmap_args"`
	Httpx    []string `yaml:"httpx_args"`
	Nuclei   []string `yaml:"nuclei_args"`
}

type Profile struct {
	HostWorkers int `yaml:"host_workers"`
	ToolArgs    `yaml:",inline"`
}

type Config struct {
	Tools struct {
		Rustscan string `yaml:"rustscan"`
		Nmap     string `yaml:"nmap"`
		Httpx    string `yaml:"httpx"`
		Nuclei   string `yaml:"nuclei"`
	} `yaml:"tools"`
	Scan struct {
		Ports   string `yaml:"ports"`
		Profile string `yaml:"profile"`
	} `yaml:"scan"`
	Profiles map[string]Profile `yaml:"profiles"`
}
```

In `Load`, after existing `ports` default:

```go
if cfg.Scan.Profile == "" {
	cfg.Scan.Profile = "normal"
}
if cfg.Profiles == nil {
	cfg.Profiles = map[string]Profile{}
}
```

- [ ] **Step 6: Implement arg splitter**

Create `internal/config/args.go`:

```go
package config

import "fmt"

func SplitArgs(input string) ([]string, error) {
	var args []string
	var current []rune
	var quote rune
	escaped := false

	flush := func() {
		if len(current) > 0 {
			args = append(args, string(current))
			current = nil
		}
	}

	for _, r := range input {
		switch {
		case escaped:
			current = append(current, r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current = append(current, r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current = append(current, r)
		}
	}
	if escaped {
		current = append(current, '\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return args, nil
}
```

- [ ] **Step 7: Write failing ResolveScan test**

Create `internal/config/profile_test.go`:

```go
package config

import (
	"reflect"
	"testing"
)

func TestResolveScanAppliesProfileAndOverrides(t *testing.T) {
	cfg := Config{Profiles: map[string]Profile{
		"slow": {
			HostWorkers: 1,
			ToolArgs: ToolArgs{
				Nmap:   []string{"-T2"},
				Nuclei: []string{"-rate-limit", "10"},
			},
		},
	}}
	cfg.Scan.Profile = "slow"

	got, err := ResolveScan(cfg, Overrides{HostWorkers: 2, NmapArgs: `-T3 --max-retries 2`})
	if err != nil {
		t.Fatalf("ResolveScan returned error: %v", err)
	}
	if got.ProfileName != "slow" || got.HostWorkers != 2 {
		t.Fatalf("unexpected effective scan: %#v", got)
	}
	if !reflect.DeepEqual(got.Nmap, []string{"-T3", "--max-retries", "2"}) {
		t.Fatalf("nmap args mismatch: %#v", got.Nmap)
	}
	if !reflect.DeepEqual(got.Nuclei, []string{"-rate-limit", "10"}) {
		t.Fatalf("nuclei args mismatch: %#v", got.Nuclei)
	}
}

func TestResolveScanRejectsUnknownProfile(t *testing.T) {
	cfg := Config{Profiles: map[string]Profile{"normal": {HostWorkers: 3}}}
	cfg.Scan.Profile = "missing"
	_, err := ResolveScan(cfg, Overrides{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 8: Run ResolveScan test and verify failure**

Run:

```bash
go test ./internal/config -run TestResolveScan -count=1
```

Expected: FAIL because `ResolveScan` and `Overrides` do not exist.

- [ ] **Step 9: Implement ResolveScan**

Create `internal/config/profile.go`:

```go
package config

import "fmt"

type Overrides struct {
	ProfileName  string
	HostWorkers int
	RustscanArgs string
	NmapArgs     string
	HttpxArgs    string
	NucleiArgs   string
}

type EffectiveScan struct {
	ProfileName string
	HostWorkers int
	ToolArgs
}

func ResolveScan(cfg Config, overrides Overrides) (EffectiveScan, error) {
	name := cfg.Scan.Profile
	if overrides.ProfileName != "" {
		name = overrides.ProfileName
	}
	if name == "" {
		name = "normal"
	}

	profile, ok := cfg.Profiles[name]
	if !ok {
		return EffectiveScan{}, fmt.Errorf("unknown scan profile: %s", name)
	}

	out := EffectiveScan{ProfileName: name, HostWorkers: profile.HostWorkers, ToolArgs: profile.ToolArgs}
	if out.HostWorkers <= 0 {
		out.HostWorkers = 1
	}
	if overrides.HostWorkers > 0 {
		out.HostWorkers = overrides.HostWorkers
	}

	var err error
	if overrides.RustscanArgs != "" {
		out.Rustscan, err = SplitArgs(overrides.RustscanArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	if overrides.NmapArgs != "" {
		out.Nmap, err = SplitArgs(overrides.NmapArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	if overrides.HttpxArgs != "" {
		out.Httpx, err = SplitArgs(overrides.HttpxArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	if overrides.NucleiArgs != "" {
		out.Nuclei, err = SplitArgs(overrides.NucleiArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	return out, nil
}
```

- [ ] **Step 10: Update default config**

Update `config/default.yaml`:

```yaml
scan:
  ports: top100
  profile: normal

profiles:
  slow:
    host_workers: 1
    rustscan_args: ["--batch-size", "100", "--timeout", "3000"]
    nmap_args: ["-T2", "--max-retries", "3", "--scan-delay", "100ms"]
    httpx_args: ["-rate-limit", "20", "-threads", "5"]
    nuclei_args: ["-rate-limit", "10", "-c", "5", "-retries", "2"]
  normal:
    host_workers: 3
    rustscan_args: ["--batch-size", "500"]
    nmap_args: ["-T3", "--max-retries", "2"]
    httpx_args: ["-rate-limit", "100", "-threads", "20"]
    nuclei_args: ["-rate-limit", "50", "-c", "20"]
  fast:
    host_workers: 8
    rustscan_args: ["--batch-size", "1000"]
    nmap_args: ["-T4", "--max-retries", "1"]
    httpx_args: ["-rate-limit", "300", "-threads", "50"]
    nuclei_args: ["-rate-limit", "150", "-c", "50"]
```

Keep existing `tools:` block unchanged.

- [ ] **Step 11: Run config tests**

Run:

```bash
go test ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 12: Commit**

```bash
git add internal/config config/default.yaml
git commit -m "feat: add scan profile configuration"
```

---

### Task 2: Add CLI Profile And Tool Args Flags

**Files:**
- Modify: `cmd/anchorscan/main.go`
- Modify: `cmd/anchorscan/main_test.go`
- Modify: `internal/app/scan.go`

**Interfaces:**
- Consumes: `config.ResolveScan(cfg, overrides)`
- Produces: `app.ScanOptions.ProfileName`, `HostWorkers`, `ExtraArgs`

- [ ] **Step 1: Write failing CLI flag test**

Add to `cmd/anchorscan/main_test.go`:

```go
func TestExecuteScanPassesProfileAndToolArgs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "report.json")
	writeFile(t, configPath, `tools:
  rustscan: /opt/rustscan
  nmap: /opt/nmap
  httpx: /opt/httpx
  nuclei: /opt/nuclei
scan:
  ports: 8080
  profile: normal
profiles:
  normal:
    host_workers: 3
    rustscan_args: ["--batch-size", "500"]
    nmap_args: ["-T3"]
    httpx_args: ["-rate-limit", "100"]
    nuclei_args: ["-rate-limit", "50"]
  slow:
    host_workers: 1
    rustscan_args: ["--batch-size", "100"]
    nmap_args: ["-T2"]
    httpx_args: ["-rate-limit", "20"]
    nuclei_args: ["-rate-limit", "10"]
`)

	runner := &recordingRunner{outputs: [][]byte{
		[]byte("Open 8080\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
	}}

	err := run([]string{
		"scan",
		"--config", configPath,
		"--target", "192.168.1.10",
		"--db", dbPath,
		"--json", jsonPath,
		"--profile", "slow",
		"--host-workers", "2",
		"--nmap-args", "-T2 --max-retries 5",
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{newRunner: func() tools.Runner { return runner }})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !runner.hasArg("/opt/nmap", "--max-retries") {
		t.Fatalf("expected nmap override args in %#v", runner.commands)
	}
}
```

Add helper below existing fake runner or replace only when compile errors require it:

```go
type recordingRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
}

func (r *recordingRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	cmd := append([]string{binary}, args...)
	r.commands = append(r.commands, cmd)
	if r.index >= len(r.outputs) {
		return nil, errors.New("unexpected command")
	}
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

func (r *recordingRunner) hasArg(binary string, arg string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, item := range cmd[1:] {
			if item == arg {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./cmd/anchorscan -run TestExecuteScanPassesProfileAndToolArgs -count=1
```

Expected: FAIL because flags are not defined or args are not passed.

- [ ] **Step 3: Extend ScanOptions**

In `internal/app/scan.go`, add:

```go
type ToolExtraArgs struct {
	Rustscan []string
	Nmap     []string
	Httpx    []string
	Nuclei   []string
}
```

Extend `ScanOptions`:

```go
ProfileName    string
HostWorkers    int
ExtraArgs      ToolExtraArgs
ProjectID      string
ConfigSnapshot string
```

No behavior change yet.

- [ ] **Step 4: Add CLI flags and ResolveScan call**

In `runScan`, add flags:

```go
profileFlag := fs.String("profile", "", "scan profile: slow, normal, or fast")
hostWorkersFlag := fs.Int("host-workers", 0, "host-level worker count override")
rustscanArgsFlag := fs.String("rustscan-args", "", "extra rustscan args")
nmapArgsFlag := fs.String("nmap-args", "", "extra nmap args")
httpxArgsFlag := fs.String("httpx-args", "", "extra httpx args")
nucleiArgsFlag := fs.String("nuclei-args", "", "extra nuclei args")
```

After resolving ports, call:

```go
effective, err := config.ResolveScan(cfg, config.Overrides{
	ProfileName:  *profileFlag,
	HostWorkers:  *hostWorkersFlag,
	RustscanArgs: *rustscanArgsFlag,
	NmapArgs:     *nmapArgsFlag,
	HttpxArgs:    *httpxArgsFlag,
	NucleiArgs:   *nucleiArgsFlag,
})
if err != nil {
	return err
}
```

Populate `ScanOptions`:

```go
ProfileName: effective.ProfileName,
HostWorkers: effective.HostWorkers,
ExtraArgs: app.ToolExtraArgs{
	Rustscan: effective.Rustscan,
	Nmap:     effective.Nmap,
	Httpx:    effective.Httpx,
	Nuclei:   effective.Nuclei,
},
```

- [ ] **Step 5: Update help text**

In `printScanHelp`, include:

```text
--profile slow|normal|fast
--host-workers N
--rustscan-args "..."
--nmap-args "..."
--httpx-args "..."
--nuclei-args "..."
```

- [ ] **Step 6: Run CLI tests**

Run:

```bash
go test ./cmd/anchorscan -count=1
```

Expected: may still FAIL until Task 3 passes args into tool wrappers. Continue to Task 3 before full commit if this test needs the tool plumbing.

---

### Task 3: Pass Profile Args To External Tool Wrappers

**Files:**
- Modify: `internal/app/scan.go`
- Modify: `internal/app/scan_test.go`
- Modify: `cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: `ScanOptions.ExtraArgs`
- Produces: each tool wrapper receives the correct extra args

- [ ] **Step 1: Write failing app-level arg plumbing test**

Add to `internal/app/scan_test.go`:

```go
func TestRunScanPassesExtraArgsToTools(t *testing.T) {
	runner := &recordingSequenceRunner{outputs: [][]byte{
		[]byte("192.168.1.10 -> [8080]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat"/></port></ports></host></nmaprun>`),
		[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
		[]byte("{" + `"template-id":"tomcat-default-login","info":{"name":"Tomcat Default Login","severity":"high"},"matched-at":"http://192.168.1.10:8080"` + "}\n"),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:          "run-1",
		Targets:        []string{"192.168.1.10"},
		Ports:          "8080",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap", Httpx: "/opt/httpx", Nuclei: "/opt/nuclei"},
		JSONReportPath: reportPath,
		ExtraArgs: ToolExtraArgs{
			Rustscan: []string{"--batch-size", "500"},
			Nmap:     []string{"-T3"},
			Httpx:    []string{"-rate-limit", "100"},
			Nuclei:   []string{"-rate-limit", "50"},
		},
		TagRules: []TagRule{{Name: "tomcat", Service: []string{"http"}, Product: []string{"tomcat"}, NucleiTags: []string{"tomcat"}, Target: "url"}},
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	for _, check := range []struct{ binary, arg string }{
		{"/opt/rustscan", "--batch-size"},
		{"/opt/nmap", "-T3"},
		{"/opt/httpx", "-rate-limit"},
		{"/opt/nuclei", "-rate-limit"},
	} {
		if !runner.hasArg(check.binary, check.arg) {
			t.Fatalf("expected %s arg %s in %#v", check.binary, check.arg, runner.commands)
		}
	}
}
```

Add helper in `internal/app/scan_test.go`:

```go
type recordingSequenceRunner struct {
	outputs  [][]byte
	commands [][]string
	index    int
}

func (r *recordingSequenceRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

func (r *recordingSequenceRunner) hasArg(binary string, arg string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, got := range cmd[1:] {
			if got == arg {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/app -run TestRunScanPassesExtraArgsToTools -count=1
```

Expected: FAIL because `RunScan` still passes `nil` extra args.

- [ ] **Step 3: Wire extra args through RunScan**

In `internal/app/scan.go`, change tool calls:

```go
ports, err := tools.DiscoverPorts(ctx, runner, opts.Tools.Rustscan, target, opts.Ports, opts.ExtraArgs.Rustscan)
```

```go
fingerprints, err := tools.Fingerprint(ctx, runner, opts.Tools.Nmap, target, ports, opts.ExtraArgs.Nmap)
```

```go
httpResult, err = tools.EnrichWeb(ctx, runner, opts.Tools.Httpx, fp, opts.ExtraArgs.Httpx)
```

```go
nseResults, err := tools.RunNSE(ctx, runner, opts.Tools.Nmap, fp.IP, fp.Port, scripts, opts.ExtraArgs.Nmap)
```

```go
out, err := tools.RunNuclei(ctx, runner, opts.Tools.Nuclei, match.Address, match.Tags, opts.ExtraArgs.Nuclei)
```

- [ ] **Step 4: Run app and CLI tests**

Run:

```bash
go test ./internal/app ./cmd/anchorscan -count=1
```

Expected: PASS, including `TestExecuteScanPassesProfileAndToolArgs` from Task 2.

- [ ] **Step 5: Commit Tasks 2 and 3 together if Task 2 was not committed**

```bash
git add cmd/anchorscan internal/app
git commit -m "feat: pass scan profile args to tools"
```

---

### Task 4: Persist Projects, Scan Runs, And Scan Events

**Files:**
- Create: `internal/store/models.go`
- Create: `internal/store/projects.go`
- Create: `internal/store/runs.go`
- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/sqlite_test.go`

**Interfaces:**
- Consumes: existing `store.Store`
- Produces: project CRUD and run/event persistence methods

- [ ] **Step 1: Write failing project CRUD test**

Add to `internal/store/sqlite_test.go`:

```go
func TestStoreProjectCRUD(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	project := Project{ID: "p1", Name: "Local Lab", Description: "Tomcat Redis", DefaultTargets: "127.0.0.1", DefaultPorts: "8080,6379", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Local Lab" {
		t.Fatalf("unexpected projects: %#v", projects)
	}

	project.Name = "Updated Lab"
	project.UpdatedAt = time.Unix(2, 0)
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("SaveProject update returned error: %v", err)
	}
	got, err := store.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	if got.Name != "Updated Lab" {
		t.Fatalf("project name mismatch: %#v", got)
	}

	if err := store.DeleteProject("p1"); err != nil {
		t.Fatalf("DeleteProject returned error: %v", err)
	}
	projects, err = store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected no projects, got %#v", projects)
	}
}
```

- [ ] **Step 2: Write failing run/event test**

Add to `internal/store/sqlite_test.go`:

```go
func TestStoreScanRunsAndEvents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	run := ScanRun{RunID: "run-1", ProjectID: "p1", Target: "127.0.0.1", Ports: "8080,6379", Profile: "normal", Status: "queued", ConfigSnapshot: "profile: normal", StartedAt: time.Unix(1, 0)}
	if err := store.SaveScanRun(run); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := store.UpdateScanRunStatus("run-1", "running", "", time.Time{}); err != nil {
		t.Fatalf("UpdateScanRunStatus returned error: %v", err)
	}
	if err := store.AppendScanEvent(ScanEvent{RunID: "run-1", Time: time.Unix(2, 0), Level: "info", Stage: "nmap", Message: "nmap still running"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}

	runs, err := store.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "running" {
		t.Fatalf("unexpected runs: %#v", runs)
	}
	events, err := store.ListScanEvents("run-1", 10)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].Stage != "nmap" {
		t.Fatalf("unexpected events: %#v", events)
	}
}
```

- [ ] **Step 3: Run store tests and verify failure**

Run:

```bash
go test ./internal/store -run 'TestStoreProjectCRUD|TestStoreScanRunsAndEvents' -count=1
```

Expected: FAIL because types and methods do not exist.

- [ ] **Step 4: Add models**

Create `internal/store/models.go`:

```go
package store

import "time"

type Project struct {
	ID             string
	Name           string
	Description    string
	DefaultTargets string
	DefaultPorts   string
	DefaultProfile string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ScanRun struct {
	RunID          string
	ProjectID      string
	Target         string
	Ports          string
	Profile        string
	Status         string
	StartedAt      time.Time
	FinishedAt     time.Time
	Error          string
	ConfigSnapshot string
}

type ScanEvent struct {
	ID      int64
	RunID   string
	Time    time.Time
	Level   string
	Stage   string
	Message string
}
```

- [ ] **Step 5: Extend schema**

In `internal/store/sqlite.go` schema string, append:

```sql
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  default_targets TEXT NOT NULL,
  default_ports TEXT NOT NULL,
  default_profile TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS scan_runs (
  run_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  target TEXT NOT NULL,
  ports TEXT NOT NULL,
  profile TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  error TEXT NOT NULL,
  config_snapshot TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS scan_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  time TEXT NOT NULL,
  level TEXT NOT NULL,
  stage TEXT NOT NULL,
  message TEXT NOT NULL
);
```

- [ ] **Step 6: Implement project methods**

Create `internal/store/projects.go` with methods:

```go
func (s *Store) SaveProject(project Project) error
func (s *Store) GetProject(id string) (Project, error)
func (s *Store) ListProjects() ([]Project, error)
func (s *Store) DeleteProject(id string) error
```

Use `INSERT ... ON CONFLICT(id) DO UPDATE SET ...` for save. Store timestamps with `time.RFC3339Nano`.

- [ ] **Step 7: Implement run/event methods**

Create `internal/store/runs.go` with methods:

```go
func (s *Store) SaveScanRun(run ScanRun) error
func (s *Store) GetScanRun(runID string) (ScanRun, error)
func (s *Store) ListScanRuns(limit int) ([]ScanRun, error)
func (s *Store) ListProjectScanRuns(projectID string, limit int) ([]ScanRun, error)
func (s *Store) UpdateScanRunStatus(runID string, status string, message string, finishedAt time.Time) error
func (s *Store) AppendScanEvent(event ScanEvent) error
func (s *Store) ListScanEvents(runID string, limit int) ([]ScanEvent, error)
```

Use `ORDER BY started_at DESC` for run lists and `ORDER BY id ASC` for events.

- [ ] **Step 8: Run store tests**

Run:

```bash
go test ./internal/store -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/store
git commit -m "feat: persist projects and scan runs"
```

---

### Task 5: Record Scan Run Lifecycle And Events

**Files:**
- Modify: `internal/app/scan.go`
- Modify: `internal/app/scan_test.go`

**Interfaces:**
- Consumes: `store.SaveScanRun`, `UpdateScanRunStatus`, `AppendScanEvent`
- Produces: persisted status/events for CLI and Web scans

- [ ] **Step 1: Write failing run lifecycle test**

Add to `internal/app/scan_test.go`:

```go
func TestRunScanPersistsRunLifecycleAndEvents(t *testing.T) {
	runner := &sequenceRunner{outputs: [][]byte{
		[]byte("192.168.1.10 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{RunID: "run-1", ProjectID: "p1", ProfileName: "normal", Targets: []string{"192.168.1.10"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath, ConfigSnapshot: "profile: normal"}
	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	run, err := scanStore.GetScanRun("run-1")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.Status != "completed" || run.Profile != "normal" {
		t.Fatalf("unexpected run: %#v", run)
	}
	events, err := scanStore.ListScanEvents("run-1", 20)
	if err != nil {
		t.Fatalf("ListScanEvents returned error: %v", err)
	}
	if len(events) == 0 || events[0].Message == "" {
		t.Fatalf("expected scan events, got %#v", events)
	}
}
```

- [ ] **Step 2: Write failing canceled status test**

Add to `internal/app/scan_test.go`:

```go
func TestRunScanMarksCanceledWhenContextCanceled(t *testing.T) {
	runner := &cancelRunner{}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := ScanOptions{RunID: "run-1", ProfileName: "normal", Targets: []string{"192.168.1.10"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath}
	err = RunScan(ctx, runner, scanStore, opts)
	if err == nil {
		t.Fatal("expected error")
	}
	run, getErr := scanStore.GetScanRun("run-1")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "canceled" {
		t.Fatalf("status mismatch: %#v", run)
	}
}

type cancelRunner struct{}

func (cancelRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	return nil, ctx.Err()
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
go test ./internal/app -run 'TestRunScanPersistsRunLifecycleAndEvents|TestRunScanMarksCanceledWhenContextCanceled' -count=1
```

Expected: FAIL because run lifecycle is not persisted.

- [ ] **Step 4: Add event helper**

In `internal/app/scan.go`, add helper:

```go
func emit(opts ScanOptions, scanStore *store.Store, level string, stage string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logf(opts, "%s", message)
	if opts.RunID == "" || scanStore == nil {
		return
	}
	_ = scanStore.AppendScanEvent(store.ScanEvent{RunID: opts.RunID, Time: time.Now(), Level: level, Stage: stage, Message: message})
}
```

Add `fmt` import.

- [ ] **Step 5: Save run at start and status at exit**

At start of `RunScan`, before scanning targets:

```go
if opts.ProfileName == "" {
	opts.ProfileName = "normal"
}
if opts.RunID != "" {
	_ = scanStore.SaveScanRun(store.ScanRun{RunID: opts.RunID, ProjectID: opts.ProjectID, Target: strings.Join(opts.Targets, ","), Ports: opts.Ports, Profile: opts.ProfileName, Status: "running", StartedAt: time.Now(), ConfigSnapshot: opts.ConfigSnapshot})
}
```

Add `strings` import.

Wrap body with a named return:

```go
func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) (runErr error) {
	defer func() {
		if opts.RunID == "" || scanStore == nil {
			return
		}
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
```

Add `errors` import.

- [ ] **Step 6: Replace stage logs with emit**

Replace stage logs with examples:

```go
emit(opts, scanStore, "info", "rustscan", "rustscan %s ports=%s", target, opts.Ports)
emit(opts, scanStore, "info", "nmap", "nmap %s ports=%v (service detection may be slow)", target, ports)
emit(opts, scanStore, "info", "httpx", "httpx %s", fp.URL)
emit(opts, scanStore, "info", "nse", "nse %s:%d scripts=%v", fp.IP, fp.Port, scripts)
emit(opts, scanStore, "info", "nuclei", "nuclei %s tags=%v", match.Address, match.Tags)
emit(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
```

Keep CLI output format the same because `emit` calls `logf`.

- [ ] **Step 7: Run app tests**

Run:

```bash
go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/app
git commit -m "feat: record scan lifecycle events"
```

---

### Task 6: Add Host Worker Concurrency And Target Error Isolation

**Files:**
- Modify: `internal/app/scan.go`
- Modify: `internal/app/scan_test.go`

**Interfaces:**
- Consumes: `ScanOptions.HostWorkers`
- Produces: bounded target concurrency and per-target failure events

- [ ] **Step 1: Write failing worker limit test**

Add to `internal/app/scan_test.go`:

```go
func TestRunScanRespectsHostWorkers(t *testing.T) {
	runner := &blockingRunner{output: []byte("127.0.0.1 -> []\n")}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{RunID: "run-1", ProfileName: "slow", HostWorkers: 1, Targets: []string{"10.0.0.1", "10.0.0.2"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath}
	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if runner.maxActive > 1 {
		t.Fatalf("expected max active 1, got %d", runner.maxActive)
	}
}

type blockingRunner struct {
	mu        sync.Mutex
	active    int
	maxActive int
	output    []byte
}

func (r *blockingRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return r.output, nil
}
```

- [ ] **Step 2: Write failing target failure isolation test**

Add:

```go
func TestRunScanContinuesAfterTargetFailure(t *testing.T) {
	runner := &failFirstRunner{outputs: [][]byte{
		[]byte("192.168.1.11 -> [22]\n"),
		[]byte(`<nmaprun><host><address addr="192.168.1.11" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`),
	}}
	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	opts := ScanOptions{RunID: "run-1", ProfileName: "normal", HostWorkers: 1, Targets: []string{"192.168.1.10", "192.168.1.11"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: reportPath}
	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	fps, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 1 || fps[0].IP != "192.168.1.11" {
		t.Fatalf("unexpected fingerprints: %#v", fps)
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
go test ./internal/app -run 'TestRunScanRespectsHostWorkers|TestRunScanContinuesAfterTargetFailure' -count=1
```

Expected: FAIL because scanning is sequential and returns immediately on first target error.

- [ ] **Step 4: Extract single-target scan helper**

In `internal/app/scan.go`, extract current per-target logic to:

```go
func scanTarget(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, target string) ([]fingerprint.ServiceFingerprint, []report.Finding, error)
```

Keep existing tool call order inside this helper.

- [ ] **Step 5: Add worker pool in RunScan**

In `RunScan`, set worker count:

```go
workers := opts.HostWorkers
if workers <= 0 {
	workers = 1
}
if workers > len(opts.Targets) {
	workers = len(opts.Targets)
}
```

Use channels:

```go
targets := make(chan string)
results := make(chan targetResult)
```

Where:

```go
type targetResult struct {
	fingerprints []fingerprint.ServiceFingerprint
	findings     []report.Finding
	err          error
	target       string
}
```

On target error, append an event with stage `target` and level `error`, then continue collecting other results. Return a run-level error only if every target fails or report writing fails.

- [ ] **Step 6: Run app tests**

Run:

```bash
go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app
git commit -m "feat: add host worker scanning"
```

---

### Task 7: Add Doctor Checks And CLI Command

**Files:**
- Create: `internal/doctor/doctor.go`
- Create: `internal/doctor/doctor_test.go`
- Modify: `cmd/anchorscan/main.go`
- Modify: `cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: `config.Load`, `config.LoadNSERules`, `config.LoadTagRules`, `ports.Resolve`
- Produces: `anchorscan doctor --config config/default.yaml --db data/scans.sqlite --reports reports`

- [ ] **Step 1: Write failing doctor package test**

Create `internal/doctor/doctor_test.go`:

```go
package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunReportsMissingTool(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`tools:
  rustscan: /missing/rustscan
  nmap: /missing/nmap
scan:
  ports: top100
  profile: normal
profiles:
  normal:
    host_workers: 1
`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	checks := Run(Options{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), ReportDir: dir})
	if !HasFailures(checks) {
		t.Fatalf("expected failures: %#v", checks)
	}
	if !containsCheck(checks, "rustscan", false) {
		t.Fatalf("expected rustscan failure: %#v", checks)
	}
}

func containsCheck(checks []Check, name string, ok bool) bool {
	for _, check := range checks {
		if check.Name == name && check.OK == ok {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run doctor test and verify failure**

Run:

```bash
go test ./internal/doctor -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement doctor package**

Create `internal/doctor/doctor.go`:

```go
package doctor

import (
	"os"
	"path/filepath"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
)

type Check struct {
	Name    string
	OK      bool
	Message string
}

type Options struct {
	ConfigPath string
	DBPath     string
	ReportDir  string
}

func Run(opts Options) []Check {
	var checks []Check
	cfg, err := config.Load(opts.ConfigPath)
	checks = append(checks, Check{Name: "config", OK: err == nil, Message: message(err, "ok")})
	if err != nil {
		return checks
	}
	for name, path := range map[string]string{"rustscan": cfg.Tools.Rustscan, "nmap": cfg.Tools.Nmap, "httpx": cfg.Tools.Httpx, "nuclei": cfg.Tools.Nuclei} {
		checks = append(checks, executableCheck(name, path))
	}
	_, portErr := ports.Resolve(cfg.Scan.Ports, filepath.Dir(opts.ConfigPath))
	checks = append(checks, Check{Name: "ports", OK: portErr == nil, Message: message(portErr, "ok")})
	checks = append(checks, writableParentCheck("database", opts.DBPath))
	checks = append(checks, writableDirCheck("reports", opts.ReportDir))
	return checks
}

func HasFailures(checks []Check) bool {
	for _, check := range checks {
		if !check.OK {
			return true
		}
	}
	return false
}
```

Implement helpers in the same file:

```go
func executableCheck(name string, path string) Check
func writableParentCheck(name string, path string) Check
func writableDirCheck(name string, path string) Check
func message(err error, ok string) string
```

`executableCheck` returns OK only when `os.Stat(path)` succeeds, file is not a directory, and `mode&0111 != 0`.

- [ ] **Step 4: Write failing CLI doctor test**

Add to `cmd/anchorscan/main_test.go`:

```go
func TestExecuteDoctorPrintsChecks(t *testing.T) {
	dir := t.TempDir()
	toolPath := filepath.Join(dir, "tool")
	writeFile(t, toolPath, "")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")

	var stdout bytes.Buffer
	err := run([]string{"doctor", "--config", configPath, "--db", filepath.Join(dir, "scan.db"), "--reports", dir}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	for _, want := range []string{"config: ok", "rustscan: ok", "nmap: ok", "database: ok", "reports: ok"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in %q", want, stdout.String())
		}
	}
}
```

- [ ] **Step 5: Implement CLI command**

In `run`, add:

```go
case "doctor":
	return runDoctor(args[1:], stdout)
```

Add function:

```go
func runDoctor(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	reportsDir := fs.String("reports", "reports", "report output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checks := doctor.Run(doctor.Options{ConfigPath: *configPath, DBPath: *dbPath, ReportDir: *reportsDir})
	for _, check := range checks {
		status := "fail"
		if check.OK {
			status = "ok"
		}
		_, _ = fmt.Fprintf(stdout, "%s: %s %s\n", check.Name, status, check.Message)
	}
	if doctor.HasFailures(checks) {
		return errors.New("doctor found issues")
	}
	return nil
}
```

Add `internal/doctor` import.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/doctor ./cmd/anchorscan -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/doctor cmd/anchorscan
git commit -m "feat: add doctor checks"
```

---

### Task 8: Add Web Server Skeleton And Home Page

**Files:**
- Create: `internal/web/server.go`
- Create: `internal/web/templates.go`
- Create: `internal/web/templates/base.html`
- Create: `internal/web/templates/home.html`
- Create: `internal/web/static/style.css`
- Create: `internal/web/server_test.go`
- Modify: `cmd/anchorscan/main.go`
- Modify: `cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: `doctor.Run`, `store.ListProjects`, `store.ListScanRuns`
- Produces: `web.NewServer(opts web.ServerOptions) (http.Handler, error)` and `anchorscan web`

- [ ] **Step 1: Write failing home handler test**

Create `internal/web/server_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestHomePageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}

	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "missing.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "AnchorScan") || !strings.Contains(res.Body.String(), "Local Lab") {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/web -run TestHomePageRenders -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement templates**

Create `internal/web/templates.go`:

```go
package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html static/*
var assets embed.FS

func parseTemplates(files ...string) (*template.Template, error) {
	all := append([]string{"templates/base.html"}, files...)
	return template.ParseFS(assets, all...)
}
```

Create `internal/web/templates/base.html`:

```html
{{define "base"}}
<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>AnchorScan</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <header><h1>AnchorScan</h1><nav><a href="/">Home</a> <a href="/projects">Projects</a> <a href="/runs">Runs</a> <a href="/config">Config</a></nav></header>
  <main>{{template "content" .}}</main>
</body>
</html>
{{end}}
```

Create `internal/web/templates/home.html`:

```html
{{define "content"}}
<h2>Home</h2>
<section>
  <h3>Projects</h3>
  <ul>{{range .Projects}}<li>{{.Name}}</li>{{else}}<li>No projects yet.</li>{{end}}</ul>
</section>
<section>
  <h3>Recent Runs</h3>
  <ul>{{range .Runs}}<li>{{.RunID}} - {{.Status}}</li>{{else}}<li>No runs yet.</li>{{end}}</ul>
</section>
{{end}}
```

Create `internal/web/static/style.css`:

```css
body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;margin:0;background:#f7f7f8;color:#222}header{background:#111827;color:white;padding:12px 20px}header a{color:white;margin-right:12px}main{padding:20px}.card,section{background:white;border:1px solid #ddd;border-radius:8px;padding:16px;margin-bottom:16px}table{border-collapse:collapse;width:100%;background:white}td,th{border:1px solid #ddd;padding:8px;text-align:left}.error{color:#b91c1c}.ok{color:#047857}
```

- [ ] **Step 4: Implement server skeleton**

Create `internal/web/server.go`:

```go
package web

import (
	"net/http"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type ServerOptions struct {
	ConfigPath string
	DBPath     string
	Listen     string
	Runner     tools.Runner
	Now        func() time.Time
}

type server struct {
	opts  ServerOptions
	store *store.Store
}

func NewServer(opts ServerOptions) (http.Handler, error) {
	if opts.Listen == "" {
		opts.Listen = "127.0.0.1:8088"
	}
	scanStore, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}
	s := &server{opts: opts, store: scanStore}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServerFS(assets))
	mux.HandleFunc("/", s.home)
	return mux, nil
}
```

Add `time` import. Implement home:

```go
func (s *server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	projects, _ := s.store.ListProjects()
	runs, _ := s.store.ListScanRuns(10)
	tmpl, err := parseTemplates("templates/home.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.ExecuteTemplate(w, "base", map[string]any{"Projects": projects, "Runs": runs})
}
```

- [ ] **Step 5: Write failing CLI web help test**

Add to `cmd/anchorscan/main_test.go`:

```go
func TestExecuteWebHelpShowsListen(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"web", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "--listen") {
		t.Fatalf("expected --listen in %q", stdout.String())
	}
}
```

- [ ] **Step 6: Add `web` CLI command**

In `run`, add:

```go
case "web":
	return runWeb(args[1:], stdout, stderr, deps)
```

Implement:

```go
func runWeb(args []string, stdout io.Writer, stderr io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	listen := fs.String("listen", "127.0.0.1:8088", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	handler, err := web.NewServer(web.ServerOptions{ConfigPath: *configPath, DBPath: *dbPath, Listen: *listen, Runner: deps.newRunner()})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "listening on http://%s\n", *listen)
	return http.ListenAndServe(*listen, handler)
}
```

Add imports `net/http` and `internal/web`.

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/web ./cmd/anchorscan -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/web cmd/anchorscan
git commit -m "feat: add local web console shell"
```

---

### Task 9: Add Web Project Management

**Files:**
- Modify: `internal/web/server.go`
- Create: `internal/web/templates/projects.html`
- Create: `internal/web/templates/project_form.html`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Consumes: `store.SaveProject`, `ListProjects`, `GetProject`, `DeleteProject`
- Produces: Web pages for project list/create/edit/delete

- [ ] **Step 1: Write failing project create test**

Add to `internal/web/server_test.go`:

```go
func TestCreateProjectFromWeb(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	form := strings.NewReader("name=Local+Lab&description=Test&default_targets=127.0.0.1&default_ports=8080%2C6379&default_profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/projects", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d", res.Code)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Local Lab" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/web -run TestCreateProjectFromWeb -count=1
```

Expected: FAIL because `/projects` POST is not implemented.

- [ ] **Step 3: Add templates**

Create `internal/web/templates/projects.html`:

```html
{{define "content"}}
<h2>Projects</h2>
<p><a href="/projects/new">New Project</a></p>
<table><tr><th>Name</th><th>Targets</th><th>Ports</th><th>Profile</th><th>Actions</th></tr>
{{range .Projects}}
<tr><td><a href="/projects/{{.ID}}">{{.Name}}</a></td><td>{{.DefaultTargets}}</td><td>{{.DefaultPorts}}</td><td>{{.DefaultProfile}}</td><td><a href="/projects/{{.ID}}/edit">Edit</a></td></tr>
{{else}}<tr><td colspan="5">No projects.</td></tr>{{end}}
</table>
{{end}}
```

Create `internal/web/templates/project_form.html`:

```html
{{define "content"}}
<h2>{{.Title}}</h2>
<form method="post" action="{{.Action}}">
  <label>Name <input name="name" value="{{.Project.Name}}" required></label><br>
  <label>Description <input name="description" value="{{.Project.Description}}"></label><br>
  <label>Default Targets <input name="default_targets" value="{{.Project.DefaultTargets}}"></label><br>
  <label>Default Ports <input name="default_ports" value="{{.Project.DefaultPorts}}"></label><br>
  <label>Default Profile <select name="default_profile"><option>slow</option><option selected>normal</option><option>fast</option></select></label><br>
  <button type="submit">Save</button>
</form>
{{end}}
```

- [ ] **Step 4: Add project routes**

In `NewServer`, register:

```go
mux.HandleFunc("/projects", s.projects)
mux.HandleFunc("/projects/new", s.projectNew)
mux.HandleFunc("/projects/", s.projectDetail)
```

Implement:

```go
func (s *server) projects(w http.ResponseWriter, r *http.Request)
func (s *server) projectNew(w http.ResponseWriter, r *http.Request)
func (s *server) projectDetail(w http.ResponseWriter, r *http.Request)
```

Use `crypto/rand` or timestamp-based ID. For this local tool, use:

```go
func newID(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%s", prefix, now.Format("20060102-150405.000000000"))
}
```

In POST `/projects`, parse form and save project.

- [ ] **Step 5: Run web tests**

Run:

```bash
go test ./internal/web -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/web
git commit -m "feat: add web project management"
```

---

### Task 10: Add Web Scan Manager And New Scan Form

**Files:**
- Create: `internal/app/manager.go`
- Create: `internal/app/manager_test.go`
- Modify: `internal/web/server.go`
- Create: `internal/web/templates/scan_new.html`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Consumes: `app.RunScan`, `store.Project`, `config.ResolveScan`
- Produces: one active local scan from Web at a time

- [ ] **Step 1: Write failing manager single-active test**

Create `internal/app/manager_test.go`:

```go
package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

func TestManagerAllowsOnlyOneActiveScan(t *testing.T) {
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	manager := NewManager(sleepRunner{}, scanStore)
	opts := ScanOptions{RunID: "run-1", ProfileName: "normal", Targets: []string{"127.0.0.1"}, Ports: "22", Tools: ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json")}
	if _, err := manager.Start(context.Background(), opts); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	_, err = manager.Start(context.Background(), opts)
	if err == nil {
		t.Fatal("expected active scan error")
	}
}

type sleepRunner struct{}

func (sleepRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		return []byte("127.0.0.1 -> []\n"), nil
	}
}

var _ tools.Runner = sleepRunner{}
```

- [ ] **Step 2: Run manager test and verify failure**

Run:

```bash
go test ./internal/app -run TestManagerAllowsOnlyOneActiveScan -count=1
```

Expected: FAIL because `NewManager` does not exist.

- [ ] **Step 3: Implement manager**

Create `internal/app/manager.go`:

```go
package app

import (
	"context"
	"errors"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type Manager struct {
	mu        sync.Mutex
	runner    tools.Runner
	store     *store.Store
	activeID  string
	cancel    context.CancelFunc
}

func NewManager(runner tools.Runner, scanStore *store.Store) *Manager {
	return &Manager{runner: runner, store: scanStore}
}

func (m *Manager) Start(ctx context.Context, opts ScanOptions) (string, error) {
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
		_ = RunScan(runCtx, m.runner, m.store, opts)
		m.mu.Lock()
		m.activeID = ""
		m.cancel = nil
		m.mu.Unlock()
	}()
	return opts.RunID, nil
}

func (m *Manager) Cancel(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeID != runID || m.cancel == nil {
		return errors.New("scan is not running")
	}
	m.cancel()
	return nil
}

func (m *Manager) ActiveRunID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeID
}
```

- [ ] **Step 4: Write failing Web new scan test**

Add to `internal/web/server_test.go`:

```go
func TestNewScanPageRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveProject(store.Project{ID: "p1", Name: "Local Lab", DefaultTargets: "127.0.0.1", DefaultPorts: "8080", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/scan/new", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "Start Scan") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
}
```

- [ ] **Step 5: Add scan form template and route**

Create `internal/web/templates/scan_new.html`:

```html
{{define "content"}}
<h2>Start Scan</h2>
<form method="post" action="/scan">
  <label>Project <select name="project_id">{{range .Projects}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label><br>
  <label>Target <input name="target" value="127.0.0.1" required></label><br>
  <label>Ports <input name="ports" value="8080,6379"></label><br>
  <label>Profile <select name="profile"><option>slow</option><option selected>normal</option><option>fast</option></select></label><br>
  <details><summary>Advanced args</summary>
    <label>Rustscan args <input name="rustscan_args"></label><br>
    <label>Nmap args <input name="nmap_args"></label><br>
    <label>Httpx args <input name="httpx_args"></label><br>
    <label>Nuclei args <input name="nuclei_args"></label><br>
  </details>
  <button type="submit">Start Scan</button>
</form>
{{end}}
```

Register routes:

```go
mux.HandleFunc("/scan/new", s.scanNew)
mux.HandleFunc("/scan", s.scanCreate)
```

- [ ] **Step 6: Implement POST `/scan`**

In `server`, store an app manager:

```go
manager *app.Manager
```

In `NewServer`, create manager with configured runner.

In `scanCreate`, parse form, load config, resolve ports/profile, create run id, build `app.ScanOptions`, call `s.manager.Start`, redirect to `/runs/{runID}`.

Use default JSON path:

```go
filepath.Join("reports", "scan-"+runID+".json")
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/app ./internal/web -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/app internal/web
git commit -m "feat: start scans from web console"
```

---

### Task 11: Add Progress API, Run Page, And Cancellation

**Files:**
- Modify: `internal/web/server.go`
- Create: `internal/web/templates/run.html`
- Create: `internal/web/static/app.js`
- Modify: `internal/web/server_test.go`
- Modify: `cmd/anchorscan/main.go`
- Modify: `cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: `store.GetScanRun`, `store.ListScanEvents`, `app.Manager.Cancel`
- Produces: `/runs/{runID}`, `/api/runs/{runID}/events`, POST `/runs/{runID}/cancel`, `anchorscan cancel --run-id ... --server ...`

- [ ] **Step 1: Write failing events API test**

Add to `internal/web/server_test.go`:

```go
func TestRunEventsAPI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "8080", Profile: "normal", Status: "running", StartedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.AppendScanEvent(store.ScanEvent{RunID: "run-1", Time: time.Unix(2, 0), Level: "info", Stage: "nmap", Message: "still running"}); err != nil {
		t.Fatalf("AppendScanEvent returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/runs/run-1/events", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "still running") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/web -run TestRunEventsAPI -count=1
```

Expected: FAIL because API route does not exist.

- [ ] **Step 3: Add run page template**

Create `internal/web/templates/run.html`:

```html
{{define "content"}}
<h2>Run {{.Run.RunID}}</h2>
<p>Status: <strong id="run-status">{{.Run.Status}}</strong></p>
<p>Target: {{.Run.Target}} | Ports: {{.Run.Ports}} | Profile: {{.Run.Profile}}</p>
<form method="post" action="/runs/{{.Run.RunID}}/cancel"><button type="submit">Cancel Scan</button></form>
<section><h3>Events</h3><pre id="events">Loading...</pre></section>
<script src="/static/app.js"></script>
<script>window.anchorRunID = "{{.Run.RunID}}";</script>
{{end}}
```

Create `internal/web/static/app.js`:

```javascript
async function refreshEvents(){
  if(!window.anchorRunID) return;
  const res = await fetch('/api/runs/' + window.anchorRunID + '/events');
  if(!res.ok) return;
  const events = await res.json();
  const box = document.getElementById('events');
  if(box){ box.textContent = events.map(e => `${e.time} [${e.stage}] ${e.message}`).join('\n'); }
}
setInterval(refreshEvents, 1000);
refreshEvents();
```

- [ ] **Step 4: Implement run/API/cancel routes**

Register:

```go
mux.HandleFunc("/runs/", s.runDetail)
mux.HandleFunc("/api/runs/", s.runAPI)
```

Implement:

```go
func (s *server) runDetail(w http.ResponseWriter, r *http.Request)
func (s *server) runAPI(w http.ResponseWriter, r *http.Request)
```

For `GET /api/runs/{id}/events`, return JSON:

```go
w.Header().Set("Content-Type", "application/json")
_ = json.NewEncoder(w).Encode(events)
```

For `POST /runs/{id}/cancel`, call `s.manager.Cancel(id)`, append a `cancel` event when successful, redirect to run page.

- [ ] **Step 5: Write failing CLI cancel test**

Add to `cmd/anchorscan/main_test.go`:

```go
func TestExecuteCancelPostsToServer(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/runs/run-1/cancel" {
			called = true
			w.WriteHeader(http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := run([]string{"cancel", "--run-id", "run-1", "--server", server.URL}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !called {
		t.Fatal("expected cancel request")
	}
}
```

- [ ] **Step 6: Implement CLI cancel**

In `run`, add:

```go
case "cancel":
	return runCancel(args[1:], stdout)
```

Implement:

```go
func runCancel(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("cancel", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runID := fs.String("run-id", "", "scan run id")
	serverURL := fs.String("server", "http://127.0.0.1:8088", "local web console URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("cancel requires --run-id")
	}
	resp, err := http.Post(strings.TrimRight(*serverURL, "/")+"/runs/"+*runID+"/cancel", "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("cancel failed: %s", resp.Status)
	}
	_, _ = fmt.Fprintf(stdout, "canceled %s\n", *runID)
	return nil
}
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/web ./cmd/anchorscan -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/web cmd/anchorscan
git commit -m "feat: add scan progress and cancellation"
```

---

### Task 12: Add Web Report List, Detail, Filters, And Downloads

**Files:**
- Create: `internal/web/reports.go`
- Create: `internal/web/reports_test.go`
- Modify: `internal/web/server.go`
- Create: `internal/web/templates/runs.html`
- Create: `internal/web/templates/report.html`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Consumes: `store.ListScanRuns`, `ListFingerprints`, `ListFindings`, `report.Build`, `report.WriteJSON`, `report.WriteHTML`
- Produces: `/runs`, `/reports/{runID}`, `/reports/{runID}.json`, `/reports/{runID}.html`

- [ ] **Step 1: Write failing report filter test**

Create `internal/web/reports_test.go`:

```go
package web

import (
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/report"
)

func TestFilterFindingsBySeverityAndSource(t *testing.T) {
	findings := []report.Finding{
		{IP: "127.0.0.1", Port: 6379, Source: "nuclei", Severity: "high", ID: "redis-default-logins"},
		{IP: "127.0.0.1", Port: 8080, Source: "nse", Severity: "info", ID: "http-title"},
	}
	got := filterFindings(findings, reportFilters{Severity: "high", Source: "nuclei"})
	if len(got) != 1 || got[0].ID != "redis-default-logins" {
		t.Fatalf("unexpected findings: %#v", got)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/web -run TestFilterFindingsBySeverityAndSource -count=1
```

Expected: FAIL because filter types do not exist.

- [ ] **Step 3: Implement report filtering**

Create `internal/web/reports.go`:

```go
package web

import (
	"strconv"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type reportFilters struct {
	IP       string
	Port     string
	Service  string
	Severity string
	Source   string
}

func filterFingerprints(items []fingerprint.ServiceFingerprint, filters reportFilters) []fingerprint.ServiceFingerprint
func filterFindings(items []report.Finding, filters reportFilters) []report.Finding
```

`filterFindings` should match exact non-empty `IP`, `Severity`, `Source`, and numeric `Port` when provided.

- [ ] **Step 4: Write failing report page test**

Add to `internal/web/server_test.go`:

```go
func TestReportPageRendersFindings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveScanRun(store.ScanRun{RunID: "run-1", Target: "127.0.0.1", Ports: "6379", Profile: "normal", Status: "completed", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0)}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "127.0.0.1", Port: 6379, Service: "redis", Product: "Redis", Normalized: "redis"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := scanStore.SaveFinding("run-1", report.Finding{IP: "127.0.0.1", Port: 6379, Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "127.0.0.1:6379"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/reports/run-1", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "redis-default-logins") {
		t.Fatalf("unexpected response: %d %s", res.Code, res.Body.String())
	}
}
```

- [ ] **Step 5: Add templates**

Create `internal/web/templates/runs.html`:

```html
{{define "content"}}
<h2>Runs</h2>
<table><tr><th>Run</th><th>Status</th><th>Target</th><th>Profile</th><th>Report</th></tr>
{{range .Runs}}<tr><td><a href="/runs/{{.RunID}}">{{.RunID}}</a></td><td>{{.Status}}</td><td>{{.Target}}</td><td>{{.Profile}}</td><td><a href="/reports/{{.RunID}}">Report</a></td></tr>{{else}}<tr><td colspan="5">No runs.</td></tr>{{end}}
</table>
{{end}}
```

Create `internal/web/templates/report.html`:

```html
{{define "content"}}
<h2>Report {{.Run.RunID}}</h2>
<p>Status: {{.Run.Status}} | Target: {{.Run.Target}} | Ports: {{.Run.Ports}} | Profile: {{.Run.Profile}}</p>
<p><a href="/reports/{{.Run.RunID}}.json">Download JSON</a> <a href="/reports/{{.Run.RunID}}.html">Download HTML</a></p>
<form method="get"><input name="ip" placeholder="IP" value="{{.Filters.IP}}"><input name="port" placeholder="Port" value="{{.Filters.Port}}"><input name="severity" placeholder="Severity" value="{{.Filters.Severity}}"><input name="source" placeholder="Source" value="{{.Filters.Source}}"><button>Filter</button></form>
<h3>Assets</h3>
<table><tr><th>IP</th><th>Port</th><th>Service</th><th>Product</th><th>Web</th><th>URL</th></tr>{{range .Fingerprints}}<tr><td>{{.IP}}</td><td>{{.Port}}</td><td>{{.Service}}</td><td>{{.Product}}</td><td>{{.IsWeb}}</td><td>{{.URL}}</td></tr>{{end}}</table>
<h3>Findings</h3>
<table><tr><th>Severity</th><th>Source</th><th>ID</th><th>Target</th><th>Summary</th></tr>{{range .Findings}}<tr><td>{{.Severity}}</td><td>{{.Source}}</td><td>{{.ID}}</td><td>{{.Target}}</td><td>{{.Summary}}</td></tr>{{else}}<tr><td colspan="5">No findings.</td></tr>{{end}}</table>
{{end}}
```

- [ ] **Step 6: Add routes**

Register:

```go
mux.HandleFunc("/runs", s.runs)
mux.HandleFunc("/reports/", s.reportDetail)
```

Implement run list and report detail. For `.json`, build report and encode JSON. For `.html`, call existing `report.WriteHTML` into a temporary file or use a new `report.RenderHTML(w, builtReport)` if easier; if adding `RenderHTML`, add a test in `internal/report/html_test.go` first.

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/web ./internal/report -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/web internal/report
git commit -m "feat: add web report views"
```

---

### Task 13: Add Web Configuration Editing With Backup

**Files:**
- Create: `internal/config/write.go`
- Create: `internal/config/write_test.go`
- Modify: `internal/web/server.go`
- Create: `internal/web/templates/config.html`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Consumes: `config.Config`
- Produces: `config.SaveWithBackup(path string, cfg Config, now time.Time) (string, error)` and Web config form

- [ ] **Step 1: Write failing config backup test**

Create `internal/config/write_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveWithBackupCreatesTimestampedBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	if err := os.WriteFile(path, []byte("scan:\n  ports: top100\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var cfg Config
	cfg.Scan.Ports = "8080"
	cfg.Scan.Profile = "normal"
	cfg.Profiles = map[string]Profile{"normal": {HostWorkers: 1}}

	backup, err := SaveWithBackup(path, cfg, time.Date(2026, 7, 7, 21, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SaveWithBackup returned error: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("expected backup: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Scan.Ports != "8080" {
		t.Fatalf("ports mismatch: %#v", loaded.Scan)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
go test ./internal/config -run TestSaveWithBackupCreatesTimestampedBackup -count=1
```

Expected: FAIL because `SaveWithBackup` does not exist.

- [ ] **Step 3: Implement SaveWithBackup**

Create `internal/config/write.go`:

```go
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func SaveWithBackup(path string, cfg Config, now time.Time) (string, error) {
	backup := path + ".bak." + now.Format("20060102-150405")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", err
	}
	return backup, nil
}
```

- [ ] **Step 4: Write failing config page test**

Add to `internal/web/server_test.go`:

```go
func TestConfigPageUpdatesToolPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("tools:\n  rustscan: /old/rustscan\n  nmap: /old/nmap\nscan:\n  ports: top100\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	handler, err := NewServer(ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(dir, "scan.db"), Now: func() time.Time { return time.Date(2026, 7, 7, 21, 30, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	form := strings.NewReader("rustscan=/new/rustscan&nmap=/new/nmap&httpx=&nuclei=&ports=8080&profile=normal")
	req := httptest.NewRequest(http.MethodPost, "/config", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Tools.Rustscan != "/new/rustscan" || cfg.Scan.Ports != "8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
```

- [ ] **Step 5: Add config template and route**

Create `internal/web/templates/config.html`:

```html
{{define "content"}}
<h2>Config</h2>
<form method="post" action="/config">
  <h3>Tools</h3>
  <label>Rustscan <input name="rustscan" value="{{.Tools.Rustscan}}"></label><br>
  <label>Nmap <input name="nmap" value="{{.Tools.Nmap}}"></label><br>
  <label>Httpx <input name="httpx" value="{{.Tools.Httpx}}"></label><br>
  <label>Nuclei <input name="nuclei" value="{{.Tools.Nuclei}}"></label><br>
  <h3>Scan</h3>
  <label>Ports <input name="ports" value="{{.Scan.Ports}}"></label><br>
  <label>Profile <select name="profile"><option>slow</option><option selected>normal</option><option>fast</option></select></label><br>
  <button type="submit">Save Config</button>
</form>
{{end}}
```

Register:

```go
mux.HandleFunc("/config", s.configPage)
```

Implement GET load config and render form. Implement POST update tool paths, scan ports/profile, then call `config.SaveWithBackup`.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/config ./internal/web -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config internal/web
git commit -m "feat: edit config from web console"
```

---

### Task 14: Update Docs, Help Text, And Lab Checklist

**Files:**
- Modify: `README.md`
- Modify: `docs/testing-lab-checklist.md`
- Modify: `docs/troubleshooting-lab.md`
- Modify: `cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: all v1.1 commands and Web behavior
- Produces: user-facing docs for v1.1

- [ ] **Step 1: Write failing help test**

Update `TestExecuteRootHelpShowsCommands` in `cmd/anchorscan/main_test.go` to expect:

```go
for _, want := range []string{"Usage:", "scan", "report", "tools check", "doctor", "web", "cancel"} {
	if !strings.Contains(output, want) {
		t.Fatalf("expected %q in help output %q", want, output)
	}
}
```

- [ ] **Step 2: Run help test and verify failure**

Run:

```bash
go test ./cmd/anchorscan -run TestExecuteRootHelpShowsCommands -count=1
```

Expected: FAIL until help text lists new commands.

- [ ] **Step 3: Update root help**

In `printRootHelp`, include:

```text
anchorscan doctor --config config/default.yaml
anchorscan web --config config/default.yaml --db data/scans.sqlite
anchorscan cancel --run-id 20260707-120000
```

- [ ] **Step 4: Update README**

Add sections:

```markdown
## V1.1 Web Console

Start local Web Console:

```bash
go run ./cmd/anchorscan web \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --listen 127.0.0.1:8088
```

Open http://127.0.0.1:8088.

## Scan Profiles

- `slow`: fragile networks and old devices
- `normal`: default
- `fast`: healthy networks and many targets

CLI:

```bash
go run ./cmd/anchorscan scan --config config/default.yaml --target 127.0.0.1 --profile slow
```

## Doctor

```bash
go run ./cmd/anchorscan doctor --config config/default.yaml --db data/scans.sqlite --reports reports
```
```

- [ ] **Step 5: Update lab checklist**

Add v1.1 manual checks:

```markdown
## V1.1 Web Lab

1. Start Web Console.
2. Run doctor from Home.
3. Create Local Lab project.
4. Start scan with `normal` profile and ports `8080,6379`.
5. Confirm progress events update.
6. Confirm report page shows Redis and Tomcat assets.
7. Start a long full-port scan and cancel it.
8. Confirm canceled status and event log.
```

- [ ] **Step 6: Update troubleshooting**

Add sections for:

```markdown
## Web Console does not start

Check listen address, DB path, and config path.

## Scan cannot start from Web

Check if another scan is running. v1.1 allows one active scan.

## Cancel does not work

Cancel only affects scans started by the running Web Console process. Confirm `anchorscan cancel --server` points to the active Web Console.
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add README.md docs cmd/anchorscan
git commit -m "docs: document anchorscan v1.1 workflows"
```

---

## Final Verification

- [ ] **Step 1: Full automated test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Doctor smoke test**

Run:

```bash
go run ./cmd/anchorscan doctor --config config/default.yaml --db data/scans.sqlite --reports reports
```

Expected: prints checks. On the developer machine with configured tools installed, exits 0.

- [ ] **Step 3: CLI scan smoke test**

Run:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target 127.0.0.1 \
  --ports 8080,6379 \
  --profile normal \
  --db data/scans.sqlite \
  --json reports/v1.1-cli.json \
  --html reports/v1.1-cli.html
```

Expected: run completes, JSON and HTML files exist.

- [ ] **Step 4: Web smoke test**

Run:

```bash
go run ./cmd/anchorscan web --config config/default.yaml --db data/scans.sqlite --listen 127.0.0.1:8088
```

Expected: open `http://127.0.0.1:8088`, create project, start scan, observe events, view report.

- [ ] **Step 5: Cancel smoke test**

With Web Console running and a long scan active, run:

```bash
go run ./cmd/anchorscan cancel --run-id 20260707-120000 --server http://127.0.0.1:8088
```

Expected: active run becomes `canceled` and event log records cancellation.

---

## Self-Review Notes

- Spec coverage: profiles, overrides, doctor, Web Console, projects, runs, events, cancellation, reports, config editing, docs, and compatibility are covered by Tasks 1-14.
- Scope control: no login, roles, distributed scan, knowledge base, or frontend framework appears in the implementation plan.
- Type consistency: `ToolArgs`, `Profile`, `EffectiveScan`, `ScanRun`, `ScanEvent`, `Manager`, and `ServerOptions` are named once and reused consistently.
- Known execution note: `anchorscan cancel` targets the active local Web Console via HTTP. This keeps cancellation simple and reliable for Web-managed scans.
