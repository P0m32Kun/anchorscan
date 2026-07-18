# AnchorScan Project Status

Last reviewed: 2026-07-09

This document records the current development baseline so later work can start from a shared understanding of what exists, what is intentionally out of scope, and what should be verified before changes.

## Product Scope

AnchorScan is a local, single-user internal scanning tool for authorized environments. It focuses on:

- tool configuration
- target/project management
- stable scan execution
- single-tool execution for targeted verification
- fingerprint-driven vulnerability checks
- readable reports and exportable asset lists

The current direction explicitly does not include:

- login, roles, or multi-user permissions
- distributed scanning
- public SaaS deployment
- a vulnerability knowledge base
- bundled third-party binaries or large exploit frameworks such as Metasploit

## Current Baseline

The project is at a V1.6.1 local-operator baseline.

Implemented capabilities:

- CLI commands: `scan`, `tool`, `report`, `doctor`, `tools check`, `web`, `cancel`
- fixed scan pipeline: rustscan -> nmap fingerprinting -> fingerprint-driven httpx / NSE / nuclei
- single-tool runs for rustscan port discovery, nmap alive/service checks, httpx web fingerprints, and nuclei tags/templates
- port selection mirrors rustscan usage: `top1000` -> `--top`, numeric ranges like `100-1000` -> `--range`, and comma-separated numeric ports -> `--ports`; `highrisk` is maintained as an insertable CSV preset
- scan profiles: `slow`, `normal`, `fast`
- per-tool extra args through configuration
- shared scan preflight for CLI and Web Console
- SQLite migrations through `schema_migrations`
- current-platform package workflow through `make package`
- cross-platform binary releases via GitHub Actions (linux/amd64, darwin/arm64, windows/amd64) on tag push
- `highrisk` port preset covering ops-remapped, ICS/SCADA, and standard high-risk service ports, editable from the Web config page
- stronger doctor checks for tools, ports, rule files, database, and reports path
- SQLite persistence for scan runs, events, fingerprints, findings, projects, and config snapshots
- persisted fingerprint fields including service, product, version, normalized service, web flag, and URL
- JSON and HTML report generation
- local Chinese Web Console for project setup, scan launch, progress tracking, cancellation, config editing, and report review
- project targets using comma-separated text, newline-separated text, or imported files
- project-level excluded targets and excluded ports
- live run event logs and nmap heartbeat messages during slow `-sV` runs
- report filtering, finding evidence expansion, pagination, host aggregation, and copy/export for `IP`, `IP:PORT`, and `URL` lists
- manual-review findings for vulnerabilities that require operator confirmation instead of bundled exploit frameworks, including BlueKeep / CVE-2019-0708 when RDP is fingerprinted on 3389

## Important Config Files

| File | Purpose |
| --- | --- |
| `config/default.yaml` | tool paths, scan defaults, scan profiles, and extra tool args (auto-generated on first run; gitignored) |
| `config/default.yaml.example` | human-readable config template (committed) |
| `config/ports-highrisk.txt` | high-risk port preset (ops-remapped + ICS/SCADA + standard services) |
| `config/ports-top1000.txt` | common port preset used by `top1000` |
| `config/service-tags.yaml` | dual-engine nuclei tag mapping (26+ services, each with `default-login`) |
| `config/nse.yaml` | dual-engine NSE script mapping (information-collection scripts per service) |
| `internal/fingerprint/normalize.go` | service normalization aliases |

Third-party tools are configured by path. AnchorScan does not package `rustscan`, `nmap`, `httpx`, `nuclei`, or Metasploit into the binary.

## Runtime Artifacts

These are generated locally and should not be treated as source:

- `data/`
- `reports/`
- `dist/`
- built binary such as `anchorscan`

## Known Operational Notes

- Web Console is designed for local single-user use.
- The current Web Console process supports one active pipeline scan or single-tool run at a time.
- `nmap -sV --version-intensity 7` can be slow on `1-65535` full-range scans. This is expected; use narrow ports for lab checks.
- nuclei and NSE run as a dual-engine matrix: every discovered service with configured rules runs both engines. `config/service-tags.yaml` maps 26+ common services (SSH, FTP, Redis, MySQL, SMB, etc.) to nuclei tags (each appending `default-login` for weak-credential coverage), while `config/nse.yaml` maps the same services to nmap NSE scripts. Services without NSE scripts (elasticsearch, kafka, kubernetes, winrm) run nuclei only.
- Manual nuclei runs can target explicit tags or one template path from the CLI/Web single-tool flow.
- BlueKeep / CVE-2019-0708 is flagged for manual review from RDP fingerprint evidence; AnchorScan does not attempt exploit-based confirmation.
- Unknown services should not be forced into the Web pipeline.
- Findings are owned by IP and port. Similar findings on different IPs should remain separate.

## Current Documentation Set

- `README.md` - user-facing quick start and feature overview
- `docs/testing-lab-checklist.md` - local lab validation checklist
- `docs/testing-results-template.md` - reusable lab result record
- `docs/troubleshooting-lab.md` - stage-by-stage lab troubleshooting
- `docs/plans/` - approved specifications and implementation plans that are still actionable

## Recommended Next Steps

1. Run a full local Web Console regression with Playwright or manual browser testing.
2. Add more local lab services to exercise MySQL, SMB, SSH, unknown services, and mixed hosts.
3. Keep refining scan controllability so fast/normal/slow defaults and per-tool overrides stay observable and safe.
4. Improve the front-end usability and visual polish without sacrificing Chinese-first operator workflow.
5. Keep report exports practical for follow-up tooling: filtered `IP`, `IP:PORT`, `URL`, and CSV should remain first-class.

## Verification Command

Before claiming a branch is ready:

```bash
go test ./...
make package
```
