# AnchorScan v1.1 Usability Design

Date: 2026-07-07
Status: Draft for review

## Goal

AnchorScan v1.1 improves ease of use without changing the V1 security model or scan pipeline.

The release focuses on a local single-user workflow:

- configure tool paths and scan parameters
- choose a scan strategy: slow, normal, or fast
- start scans from CLI or local Web Console
- observe scan progress and logs
- cancel a running scan
- view and download reports

## Non-Goals

v1.1 will not include:

- login, users, roles, or permissions
- multi-user collaboration
- distributed scan nodes or remote agents
- vulnerability knowledge base
- automatic remediation advice
- large dashboard system
- React/Vue frontend split

AnchorScan remains a single-machine operator tool.

## Existing Pipeline

The V1 pipeline stays fixed:

```text
rustscan -> nmap -sV -> fingerprint classification -> httpx / NSE / nuclei -> SQLite -> JSON / HTML report
```

v1.1 only changes how users configure, launch, observe, and review that pipeline.

## Scan Profiles

v1.1 adds three scan profiles:

| Profile | Use Case | Default Behavior |
| --- | --- | --- |
| `slow` | old devices, weak networks, fragile environments | lowest concurrency, lower rate, safer retries |
| `normal` | default internal scan | balanced stability and speed |
| `fast` | healthy network, many targets | higher concurrency and lower wait time |

`normal` is the default.

Example config shape:

```yaml
scan:
  profile: normal
  ports: top100

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

The values above are starting defaults and may be tuned during lab validation.

## Fine-Grained Overrides

Users can override profile defaults through CLI flags and Web advanced settings.

CLI examples:

```bash
anchorscan scan --profile slow
anchorscan scan --profile fast --host-workers 5
anchorscan scan --nmap-args "-T2 --max-retries 5"
anchorscan scan --nuclei-args "-rate-limit 5 -c 3"
```

Initial override flags:

- `--profile slow|normal|fast`
- `--host-workers <n>`
- `--rustscan-args "..."`
- `--nmap-args "..."`
- `--httpx-args "..."`
- `--nuclei-args "..."`

The implementation should parse args with shell-like quoting rules and pass them to existing tool wrappers as `extraArgs`.

## CLI Commands

Existing commands remain:

```text
anchorscan scan
anchorscan report
anchorscan tools check
```

New commands:

```text
anchorscan doctor
anchorscan web
anchorscan cancel
```

### `doctor`

Validates local readiness:

- config YAML parses
- rustscan, nmap, httpx, nuclei paths exist and are executable
- SQLite path is writable
- report directory is writable
- NSE rules parse
- nuclei tag rules parse
- port presets resolve

### `web`

Starts local Web Console:

```bash
anchorscan web \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --listen 127.0.0.1:8088
```

Default listen address is `127.0.0.1:8088`.

### `cancel`

Cancels a running local scan:

```bash
anchorscan cancel --run-id <run_id>
```

## Local Web Console

The Web Console is local-only and single-user.

Recommended implementation:

- Go `net/http`
- Go `html/template`
- SQLite
- static CSS
- small vanilla JavaScript polling

No frontend framework is required for v1.1.

### Pages

#### Home

Shows:

- doctor summary
- recent projects
- recent scan runs
- current running scan, if any

#### Projects

Supports:

- list projects
- create project
- edit project
- delete project
- open project details

Project fields:

```text
id
name
description
default_targets
default_ports
default_profile
created_at
updated_at
```

#### New Scan

Supports:

- select project
- enter target
- choose ports: custom, top100, top1000, full
- choose profile: slow, normal, fast
- advanced tool args overrides
- start scan

#### Run Progress

Shows:

- run status
- current stage
- elapsed time
- recent events
- cancel button

Stages:

```text
queued
rustscan
nmap
httpx
nse
nuclei
report
completed
failed
canceled
```

Polling is enough for v1.1:

```text
GET /api/runs/{run_id}/events
```

#### Reports

Supports:

- scan run list
- report detail
- asset table
- finding table
- severity summary
- filter by IP, port, service, severity, source
- download JSON
- download HTML

## Persistence Changes

v1.1 adds project and run state tables.

### `projects`

```text
id TEXT PRIMARY KEY
name TEXT NOT NULL
description TEXT
default_targets TEXT
default_ports TEXT
default_profile TEXT
created_at TEXT NOT NULL
updated_at TEXT NOT NULL
```

### `scan_runs`

```text
run_id TEXT PRIMARY KEY
project_id TEXT
target TEXT NOT NULL
ports TEXT NOT NULL
profile TEXT NOT NULL
status TEXT NOT NULL
started_at TEXT
finished_at TEXT
error TEXT
config_snapshot TEXT
```

### `scan_events`

```text
id INTEGER PRIMARY KEY AUTOINCREMENT
run_id TEXT NOT NULL
time TEXT NOT NULL
level TEXT NOT NULL
stage TEXT NOT NULL
message TEXT NOT NULL
```

Existing V1 tables for fingerprints and findings remain.

## Scan Execution Model

v1.1 should allow only one running scan at a time.

Reason:

- simpler for a single-user local tool
- safer for nmap / nuclei resource use
- avoids accidental overload of fragile networks

If a scan is already running, a new Web scan request should return a clear message. CLI can return a non-zero error.

For multiple targets inside one scan, `host_workers` controls host-level concurrency.

Target-level errors should be recorded as events and should not necessarily abort the whole run. The final run is `failed` only when the scan cannot continue or no report can be produced.

## Cancellation

Scan cancellation uses Go context cancellation.

Because V1 already executes external tools with `exec.CommandContext`, canceling the context should terminate rustscan, nmap, httpx, nuclei, and NSE subprocesses.

Cancellation should record:

- `scan_runs.status = canceled`
- final event with stage and message
- `finished_at`

## Configuration Editing

Web Console should support editing:

- tool paths
- default scan profile
- default ports
- profile args
- NSE rules
- nuclei tag rules

Before writing config changes, create a timestamped backup:

```text
config/default.yaml.bak.20260707-213000
```

v1.1 does not need complex config versioning.

## Reporting Improvements

Report detail should show:

- run metadata: target, ports, profile, timestamps, duration
- config snapshot
- host count
- open port count
- web asset count
- finding count by severity
- assets table
- findings table

Finding ownership remains per IP and port. Same vulnerability on different IPs is not deduped. Same template with different evidence should remain visible.

## Error Handling

Expected behavior:

- config parse errors are shown before scan starts
- missing tools are caught by `doctor`
- stage failures are persisted as scan events
- failed external command output is recorded in the event log
- report generation failure marks the run as failed
- canceled scans do not generate misleading completed reports

## Testing Strategy

Use TDD for implementation.

Minimum tests:

- profile config parsing
- CLI profile and extra args override
- profile args passed to rustscan, nmap, httpx, nuclei wrappers
- host worker limit behavior
- doctor success and failure cases
- project CRUD store methods
- scan run status transitions
- scan event append/list methods
- cancel updates status and stops execution context
- Web handlers render key pages
- Web API returns run events
- report filters work against stored data

Manual lab validation:

- start Web Console on `127.0.0.1:8088`
- create project
- run Tomcat + Redis lab with `normal`
- verify progress events
- verify report page
- run `slow` scan and confirm lower-rate args are used
- start long scan and cancel it
- run `doctor`

## Delivery Plan

### v1.1-alpha

- profile config and CLI flags
- tool extra args
- host workers
- doctor command
- scan run and event persistence
- local Web Console start command
- Web new scan form
- Web progress page
- Web report list and detail

### v1.1-final

- Web config editing
- Web NSE rule editing
- Web nuclei tag rule editing
- cancel scan from Web and CLI
- report filtering/search
- config backup
- config snapshot display

## Acceptance Criteria

v1.1 is complete when:

1. A user can launch `anchorscan web` and operate from a browser without hand-writing scan commands.
2. A user can choose `slow`, `normal`, or `fast` and see those settings affect tool args and host concurrency.
3. A user can override tool args when needed.
4. A user can create a project, start a scan, observe progress, cancel a scan, and view a report.
5. `doctor` catches common setup problems before a scan starts.
6. Existing V1 CLI scan/report behavior remains compatible.
7. `go test ./...` passes.
