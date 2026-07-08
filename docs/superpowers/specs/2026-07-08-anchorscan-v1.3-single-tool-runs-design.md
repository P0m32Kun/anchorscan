# AnchorScan V1.3 Single Tool Runs Design

Date: 2026-07-08

## Goal

V1.3 adds operator-controlled single tool runs while keeping the current automated pipeline intact.

AnchorScan should support both paths:

- automated pipeline scan: `rustscan -> nmap -> httpx / NSE / nuclei`
- manual single tool run: run only `rustscan`, `nmap`, `httpx`, or `nuclei`

Single tool results must be stored in SQLite, appear in run history, and be viewable through the existing report pages.

## Scope

Included:

- CLI command for single tool runs
- Web Console page for single tool runs
- shared app-level runner for manual tool execution
- persistence through existing `scan_runs`, `scan_events`, `fingerprints`, and `findings` tables
- JSON report generation for manual runs
- manual-review findings for vulnerabilities that AnchorScan cannot safely verify without heavy external frameworks
- a first manual-review rule for exposed RDP / BlueKeep follow-up

Not included:

- bundling Metasploit
- adding a generic workflow DAG engine
- adding a plugin system
- adding a vulnerability encyclopedia
- running destructive exploit checks
- adding a new table unless implementation proves the current schema is insufficient

## User Experience

### CLI

Add a new command group:

```bash
anchorscan tool rustscan --target 192.168.1.10 --ports top1000
anchorscan tool nmap --target 192.168.1.10 --ports 80,443,3389
anchorscan tool nmap --target 192.168.1.10 --mode alive
anchorscan tool httpx --url http://192.168.1.10:8080
anchorscan tool nuclei --url http://192.168.1.10:8080 --tags tomcat
anchorscan tool nuclei --url http://192.168.1.10:8080 --template cves/2021/example.yaml
```

Common flags should mirror existing scan flags where practical:

- `--config`
- `--db`
- `--json`
- `--project`
- per-tool extra args, using existing config defaults first

Each command prints:

```text
run_id=<id>
json=<path>
```

### Web Console

Add one page: "单工具调用".

The form should include:

- tool selector: `rustscan`, `nmap`, `httpx`, `nuclei`
- nmap mode selector: service fingerprint or host liveness
- target input for host-based tools
- URL input for web tools
- ports input for `rustscan` and `nmap`
- nuclei tags input
- nuclei template input
- optional extra args
- project selector, optional

After submission, use the same run detail and report pages used by normal scans.

## Architecture

### Shared Runner

Add a small app-level entry point:

```go
type ToolRunOptions struct {
    RunID      string
    ProjectID  string
    Tool       string
    Mode       string
    Target     string
    Ports      string
    URL        string
    Tags       []string
    Template   string
    ExtraArgs  []string
    JSONReportPath string
}

func RunTool(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ToolRunOptions) error
```

Implementation can use a simple `switch opts.Tool`. No plugin abstraction is needed for V1.3.

### Persistence

Reuse existing tables:

- `scan_runs` records the run.
- `scan_events` records tool progress and errors.
- `fingerprints` stores ports, service fingerprints, and web assets.
- `findings` stores nuclei results, NSE-style results, manual-review items, and informational tool output when useful.

Avoid a schema migration at the start of V1.3.

Use these existing fields:

- `scan_runs.profile`: store `tool:rustscan`, `tool:nmap`, `tool:httpx`, or `tool:nuclei`
- `scan_runs.target`: store the submitted target or URL
- `scan_runs.ports`: store submitted ports when relevant
- `scan_runs.config_snapshot`: store compact JSON of manual run inputs

If implementation becomes awkward, add `run_type` and `tool_name` later through the existing migration policy. Do not add them speculatively.

### rustscan

Inputs:

- target
- ports

Execution:

```text
rustscan -a <target> --ports/--range <ports> -g --no-banner
```

Persistence:

- save one fingerprint per open port
- `IP = target`
- `Port = open port`
- service fields empty
- `IsWeb = false`

This lets the existing report page show discovered open ports even before service fingerprinting.

### nmap

Inputs:

- target
- mode: `service` or `alive`
- ports for `service` mode

Service fingerprint execution:

```text
nmap -sV --version-intensity 7 -p <ports> <target> -oX -
```

Host liveness execution:

```text
nmap -sn <target> -oX -
```

Service mode persistence:

- parse XML with existing nmap parser
- classify fingerprints with existing fingerprint classifier
- save fingerprints

Alive mode persistence:

- `Source = "nmap"`
- `ID = "host-alive"`
- `Severity = "info"`
- `Summary = "Host is alive"` or `"Host did not respond"`

### httpx

Inputs:

- URL, or host/port converted to URL by the UI or CLI

Execution:

```text
httpx -json -status-code -title -tech-detect -follow-redirects -u <url>
```

Persistence:

- save a web fingerprint with `IsWeb = true` and `URL`
- save one informational finding only when it helps preserve status code, title, and detected technologies

Do not add columns for status code or title in V1.3.

### nuclei

Inputs:

- URL
- tags, or one template path

Execution by tags:

```text
nuclei -target <url> -tags <tags> -jsonl
```

Execution by template:

```text
nuclei -target <url> -t <template> -jsonl
```

Persistence:

- parse JSONL with existing parser
- save each match as a finding
- `Source = "nuclei"`
- `ID = template-id`
- `Severity = nuclei severity`
- `Target = matched-at`
- `Output = evidence`

## Heavy Vulnerability Checks

Some checks previously depended on Metasploit modules. AnchorScan should not bundle Metasploit because it is large, operationally noisy, and outside this project's portable local-scanner scope.

V1.3 handles these checks in three tiers.

### Tier 1: Built-in Lightweight Probe

Use this only when a safe, non-destructive protocol probe is small enough to maintain in-tree.

Results use:

- `Source = "builtin"`
- `ID = CVE or probe id`
- `Severity = mapped severity`

The finding wording must describe confidence honestly, for example "suspected", "exposure", or "requires confirmation" when the probe is not a full exploit confirmation.

### Tier 2: Manual Review Finding

Use this when reliable verification requires Metasploit, exploit-like behavior, credentials, or a complex protocol implementation.

Results use:

- `Source = "manual-review"`
- `ID = "manual-review:<CVE>"`
- `Severity = mapped severity`
- `Summary = clear follow-up action`
- `Output = why this requires manual verification`

These findings are first-class report findings. Operators can filter, export, and track them without AnchorScan pretending to have verified exploitation.

### Tier 3: Optional External Adapter

Defer this. A future version may allow configured external check binaries, but V1.3 should not add a generic adapter unless a concrete local need appears.

## BlueKeep Policy

For `CVE-2019-0708` BlueKeep:

V1.3 should not bundle Metasploit and should not claim exploit verification.

Minimum behavior:

1. If port `3389/tcp` is discovered and service fingerprinting indicates RDP or `ms-wbt-server`, create a manual-review finding.
2. The finding should use:
   - `Source = "manual-review"`
   - `ID = "manual-review:CVE-2019-0708"`
   - `Severity = "critical"`
   - `Summary = "RDP service requires BlueKeep verification"`
3. Evidence should include IP, port, service, product/version if available, and a note that external validation is required.

A later version may add a safe built-in RDP probe if it remains small and non-destructive.

## Error Handling

- Missing required input blocks the run before any external tool starts.
- Invalid ports block `rustscan` and `nmap` runs.
- Missing configured tool path blocks that tool only.
- Tool execution errors mark the manual run failed and preserve events.
- Partial parsed results should be saved before returning failure only when the tool output is valid enough to parse safely.
- Report generation failure marks the run failed.

## Testing

Add focused tests:

- `RunTool` saves rustscan open ports as fingerprints.
- `RunTool` saves nmap fingerprints.
- `RunTool` saves nmap alive-mode output as informational findings.
- `RunTool` saves httpx web fingerprints.
- `RunTool` saves nuclei JSONL matches as findings.
- nuclei template mode builds `-t` arguments instead of `-tags`.
- manual-review BlueKeep rule emits a finding for RDP on 3389.
- CLI rejects missing required input per tool.
- Web form submits a manual run and shows it in run history.

Manual verification:

```bash
go test ./...
anchorscan tool rustscan --config config/default.yaml --target 127.0.0.1 --ports 80,443
anchorscan tool nmap --config config/default.yaml --target 127.0.0.1 --ports 80,443
anchorscan tool nmap --config config/default.yaml --target 127.0.0.1 --mode alive
anchorscan tool httpx --config config/default.yaml --url http://127.0.0.1:8080
anchorscan tool nuclei --config config/default.yaml --url http://127.0.0.1:8080 --tags tech
```

## Acceptance Criteria

- Existing `anchorscan scan` behavior remains unchanged.
- Operators can run `rustscan`, `nmap`, `httpx`, and `nuclei` individually from CLI.
- Operators can run the same single tool operations from Web Console.
- Manual tool runs appear in run history.
- Manual tool run results appear in existing reports.
- Results are persisted in SQLite.
- BlueKeep-like heavy checks produce honest manual-review findings instead of requiring bundled Metasploit.
- `go test ./...` passes.

## Deferred

- bundled Metasploit
- exploit-based verification
- generic external check adapter
- generic workflow DAG editor
- vulnerability knowledge base
- new report schema for HTTP status/title
- new run schema fields unless required during implementation
