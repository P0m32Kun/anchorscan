# Changelog

All notable changes to AnchorScan are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to a manual local-operator versioning scheme (v1.0 → v1.1 → v1.2 → v1.3).

## [1.3] - 2026-07-09

v1.3 adds operator-controlled single tool runs and a BlueKeep manual-review
flag, on top of the v1.2 stability baseline.

### Added
- Single tool runs: run one engine (rustscan / nmap / httpx / nuclei) outside
  the automated pipeline via `anchorscan tool <name>`. Results still persist to
  SQLite and produce JSON reports.
- Nmap tool modes: `--mode alive` and `--mode service`, plus `--ports`,
  `--target`, `--url`, `--tags`, `--template`, and `--args` (native raw args).
- BlueKeep manual review: nmap service mode flags RDP endpoints with a
  `manual-review:CVE-2019-0708` critical finding instead of auto-confirming.
- Web Console single-tool page at `/tools/new` with per-tool forms and presets
  (sidebar entry: 单工具调用).
- `internal/version` package as the single version source, with an
  `anchorscan version` command and `--version` / `-v` flag.
- Report page per-page size selector (10 / 20 / 50) for the asset fingerprint
  table and the findings table.
- `CHANGELOG.md`.

### Changed
- Web Console footer now renders the version from the `version` package via a
  `{{version}}` template func instead of a hardcoded string.
- Bumped stale `v1.1` reference in troubleshooting docs to `v1.3`.

### Documentation
- Registry mirror guidance for Docker image pulls added to the lab checklist
  and troubleshooting (alongside the existing proxy guidance).
- Lab checklist gained a V1.3 Single Tool Runs section (CLI runs, native args,
  BlueKeep manual review, Web single-tool page, version checks).

## [1.2] - 2026-07-08

v1.2 stabilizes deployment and scan control: deterministic schema migrations,
shared scan preflight, stronger doctor output, and a packaging baseline.

### Added
- Deterministic SQLite migrations with a `schema_migrations` table and
  `(*Store).Close()`, so upgrades and new-machine setup are reproducible.
- Shared scan preflight used by both the CLI and Web Console.
- Stronger `doctor` output for config, tool paths, and environment checks.
- Minimal package workflow for new-machine deployment (tools configured by
  path, not bundled).
- Docker lab end-to-end test coverage.

### Changed
- Single active scan enforced across CLI and Web Console.
- Local worktrees ignored from version control.

## [1.1] - 2026-07-07

v1.1 builds the usability layer on the v1.0 scan pipeline: scan profiles,
fine-grained tool args, host workers, a local single-user Web Console, and
report review.

### Added
- Scan profiles (slow / normal / fast) and fine-grained per-tool args
  (`--rustscan-args`, `--nmap-args`, `--httpx-args`, `--nuclei-args`).
- Host-level worker control for multi-target scanning.
- `doctor` checks for config, tools, and paths.
- Local single-user Web Console: project/run management, scan launch,
  progress events, and cancellation.
- Web report views with asset/fingerprint tables, findings, and filtering.
- Config editing from the Web Console.
- Scan lifecycle event recording and chronological run history.

## [1.0] - 2026-07-07

v1.0 is the CLI scan pipeline baseline.

### Added
- CLI scan pipeline: rustscan (discovery) → nmap (fingerprinting) →
  httpx / NSE / nuclei (secondary), with web vs non-web routing.
- SQLite persistence for fingerprints and findings.
- JSON and HTML report generation.
- NSE script routing by normalized service and nuclei tag routing by
  service/product.
- Unknown service stability: weak fingerprints degrade gracefully without
  crashing the run.
