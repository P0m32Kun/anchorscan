# AnchorScan

`anchorscan` 是一款面向已授权内网环境的便携式自动化扫描工具。它以“**指纹驱动、精准分类**”为核心，使用 `rustscan` 做端口发现，使用 `nmap -sV` 做服务指纹识别，再根据识别出的服务类型，进入 `httpx`、`nuclei`、NSE 等后续流程，并将结果统一落入 SQLite，最后导出 JSON / HTML 报告。

## Current Status

AnchorScan 当前处于 **V1.2 稳定性增强阶段**。CLI 扫描链路和本地 Web Console 已可用，当前重点是把**部署、自检、数据库迁移、扫描前预检**这几个基础能力打牢，方便后续继续迭代扫描控制与前端体验。

Implemented:

- configurable external tool paths
- fixed pipeline: rustscan -> nmap fingerprinting -> fingerprint-driven httpx / NSE / nuclei
- custom ports, ranges like `100-1000`, `top100`, `top1000`, and `full`
- slow / normal / fast scan profiles with per-tool extra args
- shared preflight for CLI and Web scans; blocking errors stop before `rustscan`
- SQLite schema migrations with legacy upgrade compatibility
- stronger `doctor` checks for config, tools, ports, rule files, database, and reports path
- CLI commands: `scan`, `report`, `doctor`, `tools check`, `web`, `cancel`
- SQLite persistence for runs, events, fingerprints, findings, projects, and config snapshots
- JSON / HTML report generation
- local Chinese Web Console for single-user operation
- project defaults, target file import, excluded targets, and excluded ports
- live run event logs, cancel support, and nmap heartbeat during slow `-sV`
- report filtering, finding evidence details, pagination, host aggregation, and copy/export for `IP`, `IP:PORT`, and `URL` lists

Not in scope for this baseline:

- login, roles, or multi-user permissions
- distributed scanning
- public SaaS deployment
- bundled external tool binaries
- knowledge base or vulnerability encyclopedia

## Quick Start

1. Edit [config/default.yaml](./config/default.yaml) so the tool paths point at your local binaries.
2. Run self-check first:

```bash
go run ./cmd/anchorscan doctor --config config/default.yaml --db data/scans.sqlite --reports reports
```

3. Run a scan:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target 127.0.0.1 \
  --db data/scans.sqlite \
  --json reports/test.json \
  --html reports/test.html
```

4. Rebuild a report from stored data:

```bash
go run ./cmd/anchorscan report \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --run-id <run_id> \
  --json reports/rerender.json \
  --html reports/rerender.html
```

5. Check configured tools:

```bash
go run ./cmd/anchorscan tools check --config config/default.yaml
```

## Build / Package

常用命令：

```bash
make test
make build
make package
```

- `make test`: 运行全部测试
- `make build`: 输出当前平台二进制到 `dist/anchorscan`
- `make package`: 生成当前平台归档包到 `dist/`

部署到新设备时，优先参考部署文档：

- [docs/deploy.md](./docs/deploy.md)

## V1.1 Web Console

Start local Web Console:

```bash
go run ./cmd/anchorscan web \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --listen 127.0.0.1:8088
```

Open http://127.0.0.1:8088.

Web Console 当前特性：

- 中文界面，适合本机单兵使用
- 项目管理：保存默认目标、默认端口、默认档位
- 项目目标支持：
  - 支持英文逗号或换行分隔多个目标
  - `IP`
  - `CIDR`
  - 简单范围文本
  - 文件导入并追加到目标列表
- 项目端口支持：
  - `top100`
  - `top1000`
  - `full`
  - `100-1000`
  - `80,443,8080,3389`
- 可选排除项：
  - 排除目标
  - 排除端口
- 运行页面可查看实时事件日志
- 报告页面支持筛选、证据详情展开、资产/漏洞分页
- 报告页面支持按主机聚合视图，以及按当前筛选结果复制/导出 `IP`、`IP:PORT`、`URL` 清单

报告导出接口也可以直接访问，适合复制给其他验证工具继续使用：

- `http://127.0.0.1:8088/reports/<run_id>/assets.txt?kind=ip_port&q=redis`
- `http://127.0.0.1:8088/reports/<run_id>/assets.txt?kind=url&q=tomcat`
- `http://127.0.0.1:8088/reports/<run_id>/assets.csv?q=redis`

说明：

- 如果在“开始扫描”页面只选择项目，不手工填写目标/端口，则会自动使用项目默认值。
- 终端传多个目标时，使用英文逗号分隔，例如：`--target 172.22.0.2,172.22.0.3`
- 如果项目配置了排除目标或排除端口，发起扫描时会自动生效。

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

## Port Selection

Use one of these with `--ports`:

- `22,80,443,8080` for a custom list
- `100-1000` for a custom range
- `top100`
- `top1000`
- `full`

You can also set the default in the `scan.ports` field of the config file.

For local lab runs, prefer the exact ports you want to test, for example:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target 127.0.0.1 \
  --ports 8080,6379 \
  --db data/scans.sqlite \
  --json reports/lab.json \
  --html reports/lab.html
```

`full` / `1-65535` can discover many local application ports. `nmap -sV --version-intensity 7` may then take a while during service detection; this is expected.

## Output

- Progress logs go to stderr.
- Result paths go to stdout.
- JSON and HTML reports are written to the paths you choose.
- During nmap service detection, AnchorScan prints a heartbeat every 30 seconds so a slow `nmap -sV` run is visible:

```text
[scan] nmap 127.0.0.1 ports=[...] (service detection may be slow)
[scan] nmap 127.0.0.1 still running elapsed=30s
[scan] nmap 127.0.0.1 services=5 elapsed=1m12s
```

## Lab docs

- `/Users/kun/DEV/new-Anchor/docs/project-status.md`
- `/Users/kun/DEV/new-Anchor/docs/testing-lab-checklist.md`
- `/Users/kun/DEV/new-Anchor/docs/testing-results-template.md`
- `/Users/kun/DEV/new-Anchor/docs/troubleshooting-lab.md`

## Local lab

Start the minimal lab:

```bash
docker compose -f docker-compose.lab.yml up -d
```

Stop it:

```bash
docker compose -f docker-compose.lab.yml down
```
