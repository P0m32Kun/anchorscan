# AnchorScan Project Status

Last reviewed: 2026-07-24

This document records the current development baseline so later work can start from a shared understanding of what exists, what is intentionally out of scope, and what should be verified before changes.

## Product Scope

AnchorScan is a local, single-user internal scanning tool for authorized environments. It focuses on:

- tool configuration
- target/project management
- stable scan execution
- single-tool execution for targeted verification
- fingerprint-driven vulnerability checks
- project-scoped verification and evidence capture
- readable run reports and formal project delivery reports
- local vulnerability knowledge-base guidance and exportable asset lists

The current direction explicitly does not include:

- login, roles, or multi-user permissions
- distributed scanning
- public SaaS deployment
- a shared remote vulnerability intelligence service
- bundled third-party binaries or large exploit frameworks such as Metasploit

## Current Baseline

The project is at the v2.0.0 local-operator baseline.

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
- SQLite persistence for scan runs, events, leases, detection checks, fingerprints, findings, projects, zones, verifications, evidence, and config snapshots
- persisted fingerprint fields including service, product, version, normalized service, web flag, and URL
- run-level JSON/HTML reports plus project-level single-file HTML and DOCX delivery reports
- local Chinese Web Console with progressively enhanced Vue 3 / TypeScript interactions embedded in the Go binary
- system/light/dark themes, keyboard-visible focus, shared confirmation dialogs, and 1280/1440 browser smoke coverage
- projects organized by Network Zone; each project scan selects one zone and supplies its own targets, ports, exclusions, and profile
- verification workbench for confirmed, not-observed, and inconclusive conclusions with ordered screenshot evidence
- live run event logs, nmap heartbeat messages during slow `-sV` runs, and persisted interruption recovery facts
- report filtering, detection coverage, finding evidence expansion, host/vulnerability aggregation, and copy/export for `IP`, `IP:PORT`, and `URL` lists
- local vulnerability knowledge-base guidance plus optional `rdpscan` BlueKeep / CVE-2019-0708 detection

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
- One active pipeline scan or single-tool run is allowed per database; persisted Run Leases prevent competing processes from owning work concurrently.
- Web static resources are embedded in the Go binary. Rebuild the binary after frontend changes before judging browser behavior.
- `nmap -sV --version-intensity 7` can be slow on `1-65535` full-range scans. This is expected; use narrow ports for lab checks.
- nuclei and NSE run as a dual-engine matrix: every discovered service with configured rules runs both engines. `config/service-tags.yaml` maps 26+ common services (SSH, FTP, Redis, MySQL, SMB, etc.) to nuclei tags (each appending `default-login` for weak-credential coverage), while `config/nse.yaml` maps the same services to nmap NSE scripts. Services without NSE scripts (elasticsearch, kafka, kubernetes, winrm) run nuclei only.
- Manual nuclei runs can target explicit tags or one template path from the CLI/Web single-tool flow.
- BlueKeep / CVE-2019-0708 can be checked by the optional `rdpscan` engine. Missing configuration does not block scans; `SAFE` and `UNKNOWN` do not become confirmed vulnerabilities.
- Unknown services should not be forced into the Web pipeline.
- Findings are owned by IP and port. Similar findings on different IPs should remain separate.

## Current Documentation Set

- [README.md](../README.md) - 用户快速开始、运行方式和功能概览
- [docs/project-status.md](project-status.md) - 当前产品基线、边界和交付前验证命令
- [docs/deploy.md](deploy.md) - 打包、发布与部署说明
- [docs/testing-lab-checklist.md](testing-lab-checklist.md) - 外部 Docker 实验室的启动与验收清单
- [docs/testing-results-template.md](testing-results-template.md) - 可复制的实验室结果记录模板
- [docs/troubleshooting-lab.md](troubleshooting-lab.md) - 按扫描阶段组织的实验室故障排查
- [docs/adr/](adr/) - 仍生效的架构决策；被替代的决策位于 `docs/adr/archive/`
- [docs/research/](research/) - 外部资料调研与来源记录
- [docs/plans/](plans/) - 当前可执行计划（目前没有）
- [docs/plans/archive/](plans/archive/) - 已完成计划的规格、设计与验收历史记录

## Recommended Next Steps

1. Add more shared-lab services to exercise MySQL, SMB, SSH, unknown services, and mixed hosts.
2. Keep refining scan controllability so fast/normal/slow defaults and per-tool overrides stay observable and safe.
3. Keep report exports practical for follow-up tooling: filtered `IP`, `IP:PORT`, `URL`, and CSV should remain first-class.
4. Split shared CSS only when `style.css` ownership or the similar settings/report outline blocks make maintenance measurably harder.

## Verification Command

Before claiming a branch is ready:

```bash
make pr-check
```
