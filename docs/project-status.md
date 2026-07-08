# AnchorScan Project Status

Last reviewed: 2026-07-08

This document records the current development baseline so later work can start from a shared understanding of what exists, what is intentionally out of scope, and what should be verified before changes.

## Product Scope

AnchorScan is a local, single-user internal scanning tool for authorized environments. It focuses on:

- tool configuration
- target/project management
- stable scan execution
- fingerprint-driven vulnerability checks
- readable reports and exportable asset lists

The current direction explicitly does not include:

- login, roles, or multi-user permissions
- distributed scanning
- public SaaS deployment
- a vulnerability knowledge base
- bundled third-party binaries

## Current Baseline

The project is at a V1.1 local-operator baseline.

Implemented capabilities:

- CLI commands: `scan`, `report`, `doctor`, `tools check`, `web`, `cancel`
- fixed scan pipeline: rustscan -> nmap fingerprinting -> fingerprint-driven httpx / NSE / nuclei
- port selection: custom lists, ranges like `100-1000`, `top100`, `top1000`, and `full`
- scan profiles: `slow`, `normal`, `fast`
- per-tool extra args through configuration
- SQLite persistence for scan runs, events, fingerprints, findings, projects, and config snapshots
- persisted fingerprint fields including service, product, version, normalized service, web flag, and URL
- JSON and HTML report generation
- local Chinese Web Console for project setup, scan launch, progress tracking, cancellation, config editing, and report review
- project targets using comma-separated text, newline-separated text, or imported files
- project-level excluded targets and excluded ports
- live run event logs and nmap heartbeat messages during slow `-sV` runs
- report filtering, finding evidence expansion, pagination, host aggregation, and copy/export for `IP`, `IP:PORT`, and `URL` lists

## Important Config Files

| File | Purpose |
| --- | --- |
| `config/default.yaml` | tool paths, scan defaults, scan profiles, and extra tool args |
| `config/service-tags.yaml` | fingerprint-driven nuclei tag mapping |
| `config/nse.yaml` | fingerprint-driven NSE script mapping |
| `config/service-aliases.yaml` | service normalization aliases |

Third-party tools are configured by path. V1.1 does not package `rustscan`, `nmap`, `httpx`, or `nuclei` into the AnchorScan binary.

## Runtime Artifacts

These are generated locally and should not be treated as source:

- `data/`
- `reports/`
- built binary such as `anchorscan`

## Known Operational Notes

- Web Console is designed for local single-user use.
- The current Web Console process supports one active scan at a time.
- `nmap -sV --version-intensity 7` can be slow on full-port scans. This is expected; use narrow ports for lab checks.
- nuclei execution is intentionally narrow and fingerprint-driven through tags such as `redis` or `tomcat`.
- Unknown services should not be forced into the Web pipeline.
- Findings are owned by IP and port. Similar findings on different IPs should remain separate.

## Current Documentation Set

- `README.md` - user-facing quick start and feature overview
- `docs/testing-lab-checklist.md` - local lab validation checklist
- `docs/testing-results-template.md` - reusable lab result record
- `docs/troubleshooting-lab.md` - stage-by-stage lab troubleshooting
- `docs/superpowers/plans/` - implementation plans used during development

## Recommended Next Steps

1. Run a full local Web Console regression with Playwright or manual browser testing.
2. Add more local lab services to exercise MySQL, SMB, SSH, unknown services, and mixed hosts.
3. Decide a lightweight SQLite migration policy before changing schema again.
4. Improve release packaging later, after tool paths and runtime behavior are stable.
5. Keep report exports practical for follow-up tooling: filtered `IP`, `IP:PORT`, `URL`, and CSV should remain first-class.

## Verification Command

Before claiming a branch is ready:

```bash
go test ./...
```
