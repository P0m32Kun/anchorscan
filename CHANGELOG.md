# Changelog

All notable changes to AnchorScan are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to a manual local-operator versioning scheme (v1.0 → v1.1 → v1.2 → v1.3 → v1.4 → v1.5 → v1.5.1 → v1.6.0).

## [1.6.2] - 2026-07-10

v1.6.2 聚焦控制台 UI/UX 深度重设计，大幅优化视觉对比度、响应式移动端布局、实时扫描状态监控，并重构了更加安全的智能过滤面板。

### Added
- 扫描实时 Stepper 进度条：在扫描详情页直观显示五个扫描阶段（主机发现、端口扫描、指纹探测、漏洞评估、报告生成），利用前端 JS 解析阶段事件动态流式推进。
- 智能过滤面板：将漏洞与资产过滤拆分为一键应用检索与浮窗维度过滤。支持自动识别并分流 IP 与文本关键字，无需点击多余按钮即可通过 Popover 气泡菜单完成危险等级、端口服务、数据源、视图的筛选。
- 活动过滤徽章（Filter Tags）：过滤条件生效后自动渲染为小胶囊徽章，点击徽章 ✕ 按钮即时清除对应条件并更新结果。
- 漏洞原始证据块（Evidence Container）IDE 风格化重构，支持一键安全复制日志并提供临时绿勾反馈。

### Changed
- 将 muted 文本对比度从 #666a73 调优至 #8e94a0，通过 WCAG AA 级标准（4.71:1）无障碍评估。
- 控制台在宽度 <= 1024px 时自适应展示，左侧菜单滑动收纳为高斯模糊遮罩覆盖下的侧边抽屉。
- 安全加固：使用 textContent 彻底消除了由 URL Query 输入引发的 DOM-based 反射型 XSS 漏洞。
- 清理遗留事件：移除了第一阶段遗留的下拉框 change 直接提交逻辑，交互机制完全由 Popover 应用及 requestSubmit 统一控制。

## [1.6.1] - 2026-07-10

### Added
- 项目详情页与项目内发起扫描流程，运行列表显示所属项目。
- 扩展 nuclei 与 NSE 服务规则，覆盖更多常见服务和默认凭据检测。

### Changed
- Web 扫描必须绑定项目，报告与扫描产物按项目和运行归档。
- 端口输入与 rustscan 原生语义对齐：支持 `top1000`、数字范围和端口 CSV；高危端口作为可插入 CSV 预设。
- Web 服务使用 httpx 与 nuclei，非 Web 服务按配置运行 nuclei 与 NSE，避免重复探测。

## [1.6.0] - 2026-07-09

v1.6.0 聚焦"开箱即用"与跨平台分发：高危端口预设、配置自动初始化、首次运行零手动配置，以及 GitHub Actions 自动打包多平台二进制。

### Added
- 新增 `highrisk` 端口预设（`config/ports-highrisk.txt`），覆盖运维改端口（10022/13306 等）、工控/SCADA 端口（502/2404 等）、标准高危服务，约 50 个端口。扫描页、项目表单、单工具页（rustscan）均支持一键插入。
- 全局配置页新增「高危端口列表」可视化编辑面板，保存写回 `config/ports-highrisk.txt`（带时间戳备份），支持随使用持续积累常见高危端口。
- `config.Load` 首次运行自动生成 `config/default.yaml`，工具路径通过 `exec.LookPath` 从系统 PATH 自动检测，无需手动编辑配置。
- `store.Open` 自动创建数据库父目录，解决全新 clone 后 `data/` 缺失导致的 `out of memory (14)` 启动失败。
- GitHub Actions（`.github/workflows/release.yml`）：打 tag 时自动交叉编译 linux/amd64、darwin/arm64、windows/amd64 裸二进制并发布到 Release；tag 含 `-rc`/`-beta`/`-alpha` 后缀自动标记为预发布。

### Changed
- 扫描页与项目表单的端口输入框改为自适应高度 textarea，避免长端口列表被截断。
- `config/default.yaml` 移出版本控制（含机器相关绝对路径），新增 `config/default.yaml.example` 作为参考模板。
- Docker 靶场（`docker-compose.lab.yml`）、e2e 测试、lab 文档移出版本控制（本地测试设施，不进仓库）。
- Makefile 移除 `e2e` target；`package` target 改用 `default.yaml.example` 打包。
- README 重写为简洁中文版，命令行参数依赖内置默认值精简；新增「下载预编译二进制」使用方式。
- `doctor` 的 databaseCheck 修复父目录不存在时的误报。

## [1.5.1] - 2026-07-09

v1.5.1 completes the v1.5 merge by adding the v1.4 inline HTML report restyle
that was still only present on `codex/v1.4-inline`.

### Changed
- Restyled HTML security reports and finding details from the v1.4 inline
  report branch.
- Ignored generated `cmd/anchorscan/data/` scan artifacts.

## [1.5] - 2026-07-09

v1.5 backfills the v1.2 stability work that was developed on a parallel
branch but never merged into `main`. These capabilities were listed in the
v1.2 changelog but absent from the released code; this release lands them on
the current baseline alongside the v1.3/v1.4 features.

### Added
- Deterministic SQLite schema migrations via a `schema_migrations` table and
  versioned migrations (`internal/store/migrations.go`). The v1.4
  `artifact_dir` column is included as migration v2.
- Shared scan preflight (`internal/preflight`) used by both the CLI and the
  Web Console: blocking errors (missing required tools, bad ports, no
  targets, unwritable paths) stop a scan before `rustscan`.
- Stronger `doctor` checks for config, tools, ports, rule files, database,
  and reports path.
- Current-platform package workflow (`Makefile`: `test`, `e2e`, `build`,
  `package`) and a deployment guide (`docs/deploy.md`).
- Real-binary smoke e2e coverage under `e2e/`.

### Changed
- CLI scan prints a preflight summary (targets, ports, profile, workers)
  before scanning and stops on preflight errors.
- Web Console scan form surfaces preflight validation errors on rejected
  submissions.
- `Store.Open` runs migrations instead of an inline schema.

## [1.4] - 2026-07-09

v1.4 adds scan artifact persistence and richer reporting on top of v1.3: each
run now saves its raw tool outputs and an audit artifact bundle under an
operator-configured artifact root, and report filters and exports gain more
control.

### Added
- Scan artifact directories persisted by the store layer (`store/runs.go`,
  `models.go`) so each run carries its artifact root.
- Raw tool outputs exposed by the engine tools (rustscan, nmap, NSE, httpx,
  nuclei) for downstream artifact capture.
- Scan audit artifact saving via the new `internal/app/artifacts.go` helper,
  invoked from the scan pipeline.
- Artifact-root inputs and cleanup wired into both the CLI
  (`cmd/anchorscan/main.go`) and the Web Console (`scan_new.html`, server),
  with cleanup on run deletion.
- Report filters and exports improvements in the HTML report (`report.html`)
  and Web report views (`internal/web/reports.go`, `server.go`), with
  accompanying tests.

### Documentation
- v1.4 optimization design and implementation plan committed alongside the
  feature work.

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
