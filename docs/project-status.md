# AnchorScan Project Status

Last reviewed: 2026-08-07

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

The project is a local-operator baseline; release builds derive their displayed version from the `v*` release tag, while development builds display an explicit development version.

Implemented capabilities:

- CLI commands: `scan`, `tool`, `report`, `doctor`, `tools check`, `web`, `cancel`
- fixed scan pipeline: nmap -sn alive sweep -> fathom (port/fingerprint/high-risk detection in one call) -> fingerprint-driven httpx / NSE / nuclei; nmap is retained as the NSE engine (and the alive-sweep engine until fathom's discover stage lands), rustscan is out of the pipeline
- single-tool runs (standalone tool page, independent of the scan pipeline) for rustscan port discovery, nmap alive/service checks, httpx web fingerprints, and nuclei tags/templates
- port selection follows rustscan-style expressions consumed by fathom: `top1000` -> common-1000 preset, numeric ranges like `100-1000`, and comma-separated numeric ports; `highrisk` is maintained as an insertable CSV preset
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
- live run event logs (nmap alive sweep and fathom stage progress), and persisted interruption recovery facts
- report filtering, detection coverage, finding evidence expansion, host/vulnerability aggregation, and copy/export for `IP`, `IP:PORT`, and `URL` lists
- local vulnerability knowledge-base guidance plus optional `rdpscan` BlueKeep / CVE-2019-0708 detection

## Important Config Files

| File | Purpose |
| --- | --- |
| `config/default.yaml` | tool paths, scan defaults, scan profiles, and extra tool args (auto-generated on first run; gitignored) |
| `config/default.yaml.example` | human-readable config template (committed) |
| `config/ports-highrisk.txt` | high-risk port preset (ops-remapped + ICS/SCADA + standard services) |
| `config/ports-top1000.txt` | common port preset used by `top1000` |
| `config/service-tags.yaml` | nuclei tag mapping (26+ services; SSH routes via `-tags ssh -exclude-tags default-login` to the RBKD-templates `ssh-mini-brute` template — RBKD-templates is the private set merged into the official nuclei-templates directory; this repo ships no templates) |
| `config/nse.yaml` | nmap NSE script mapping for services with applicable scripts |
| `internal/fingerprint/normalize.go` | service normalization aliases |

Third-party tools are configured by path. AnchorScan does not package `fathom`, `rustscan`, `nmap`, `httpx`, `nuclei`, or Metasploit into the binary.

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
- `nmap -sV --version-intensity 7` can be slow on `1-65535` full-range scans. This is expected; use narrow ports for lab checks. (fathom replaced `-sV` fingerprinting in the scan pipeline; the note applies to the single-tool nmap service mode only.)
- nuclei 与 NSE 根据服务指纹和各自规则独立调度：`config/service-tags.yaml` 映射 nuclei tags，`config/nse.yaml` 只为有适用 NSE 的服务映射脚本。服务可能运行两个引擎、其中一个，或在无规则时被跳过；Detection Coverage 记录实际执行事实。
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
- [docs/adr/README.md](adr/README.md) - accepted ADR index; historical/superseded decisions are under `docs/adr/archive/`
- [docs/research/](research/) - 外部资料调研与来源记录
- [docs/plans/archive/harden-release-and-scan-trust/](plans/archive/harden-release-and-scan-trust/) - 已完成的加固计划与 ticket
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
