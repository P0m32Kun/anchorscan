# AnchorScan

`anchorscan` 是一款面向已授权内网环境的便携式自动化扫描工具。它以“**指纹驱动、精准分类**”为核心，使用 `rustscan` 做端口发现，使用 `nmap -sV` 做服务指纹识别，再根据识别出的服务类型，进入 `httpx`、`nuclei`、NSE 等后续流程，并将结果统一落入 SQLite，最后导出 JSON / HTML 报告。

## Current Status

AnchorScan is currently at a V1.5 local-operator baseline. The CLI scan pipeline is usable, and the local Web Console is the preferred day-to-day interface for project setup, scan launch, progress tracking, config editing, and report review. The stability baseline (deterministic migrations, shared scan preflight, stronger doctor, packaging, and real-binary e2e) is in place.

Implemented:

- configurable external tool paths
- fixed pipeline: rustscan -> nmap fingerprinting -> fingerprint-driven httpx / NSE / nuclei
- custom ports, ranges like `100-1000`, `top100`, `top1000`, and `full`
- slow / normal / fast scan profiles with per-tool extra args
- CLI commands: `scan`, `tool`, `report`, `doctor`, `tools check`, `web`, `cancel`
- shared preflight for CLI and Web scans; blocking errors stop before `rustscan`
- SQLite schema migrations with legacy upgrade compatibility
- stronger `doctor` checks for config, tools, ports, rule files, database, and reports path
- SQLite persistence for runs, events, fingerprints, findings, projects, and config snapshots
- JSON / HTML report generation
- local Chinese Web Console for single-user operation
- single-tool runs for `rustscan`, `nmap`, `httpx`, and `nuclei`, with the same SQLite and report viewing flow
- project defaults, target file import, excluded targets, and excluded ports
- live run event logs, cancel support, and nmap heartbeat during slow `-sV`
- report filtering, finding evidence details, pagination, host aggregation, and copy/export for `IP`, `IP:PORT`, and `URL` lists
- manual-review findings for vulnerabilities that should not require bundling large exploit frameworks, including BlueKeep / CVE-2019-0708 when RDP is fingerprinted on 3389

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

## Single Tool Runs

Use `anchorscan tool` when you want to run one engine without the full rustscan -> nmap -> httpx/NSE/nuclei pipeline. Results are still written to SQLite and JSON reports, so they can be reviewed through the same `/runs/<run_id>` and `/reports/<run_id>` pages.

Examples:

```bash
# Port discovery only
go run ./cmd/anchorscan tool rustscan \
  --config config/default.yaml \
  --target 192.0.2.10 \
  --ports 80,443,3389 \
  --db data/scans.sqlite \
  --json reports/rustscan-only.json

# Nmap host alive check
go run ./cmd/anchorscan tool nmap \
  --config config/default.yaml \
  --mode alive \
  --target 192.0.2.10 \
  --db data/scans.sqlite \
  --json reports/nmap-alive.json

# Nmap service fingerprinting for known ports
go run ./cmd/anchorscan tool nmap \
  --config config/default.yaml \
  --mode service \
  --target 192.0.2.10 \
  --ports 22,80,3389 \
  --db data/scans.sqlite \
  --json reports/nmap-service.json

# Web fingerprinting only
go run ./cmd/anchorscan tool httpx \
  --config config/default.yaml \
  --url http://192.0.2.10:8080 \
  --db data/scans.sqlite \
  --json reports/httpx-only.json

# Nuclei by tag or by a specific template
go run ./cmd/anchorscan tool nuclei \
  --config config/default.yaml \
  --url http://192.0.2.10:8080 \
  --tags tomcat,cve \
  --db data/scans.sqlite \
  --json reports/nuclei-tags.json

go run ./cmd/anchorscan tool nuclei \
  --config config/default.yaml \
  --url http://192.0.2.10:8080 \
  --template cves/2021/CVE-2021-41773.yaml \
  --db data/scans.sqlite \
  --json reports/nuclei-template.json
```

Use `--args` to pass extra arguments to the selected tool only, for example `--args "--min-rate 50"` for nmap or `--args "-rate-limit 5"` for nuclei.

In the Web Console, open `http://127.0.0.1:8088/tools/new` or use the sidebar “单工具调用” entry.

## Build / Package

常用命令：

```bash
make test
make e2e
make build
make package
```

- `make test`: 运行全部测试
- `make e2e`: 运行真实二进制 + 本地 lab 的 smoke e2e
- `make build`: 输出当前平台二进制到 `dist/anchorscan`
- `make package`: 生成当前平台归档包到 `dist/`

部署到新设备时，优先参考部署文档：

- [docs/deploy.md](./docs/deploy.md)

## Web Console

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
- 单工具调用页面可单独执行 rustscan / nmap / httpx / nuclei
- 报告页面支持筛选、证据详情展开、资产/漏洞分页
- 报告页面支持按主机聚合视图，以及按当前筛选结果复制/导出 `IP`、`IP:PORT`、`URL` 清单
- Web Console 发起的扫描会把托管 JSON 报告写入 `data/` 目录：
  - 项目扫描：`data/projects/<project_id>/runs/<run_id>/report.json`
  - 非项目扫描：`data/runs/<run_id>/report.json`
- 删除项目时会同时清理该项目关联的扫描历史、指纹、Finding、事件日志，以及 `data/projects/<project_id>/` 下的托管报告文件

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

## E2E Smoke

默认的 `go test ./...` 不会跑真实扫描 e2e。  
真实链路 smoke 单独执行：

```bash
make e2e
```

它会做两件事：

1. 启动或复用 `docker-compose.lab.yml` 里的本地靶场
2. 运行两条真实 smoke：
   - CLI：真实执行 `anchorscan scan`
   - Web：真实启动 `anchorscan web`，通过 HTTP 发起扫描并检查报告页

当前 smoke 目标固定为：

- `127.0.0.1`
- 端口 `8080,6379`

如果工具不在 `PATH`，可以临时指定：

```bash
ANCHORSCAN_RUSTSCAN=/path/to/rustscan \
ANCHORSCAN_NMAP=/path/to/nmap \
ANCHORSCAN_HTTPX=/path/to/httpx \
ANCHORSCAN_NUCLEI=/path/to/nuclei \
make e2e
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

## Local Lab Startup

The bundled lab can now expose six service families for manual testing:

| Service | Container | Real port | Published host port |
| --- | --- | --- | --- |
| Tomcat | `anchorscan-lab-tomcat` | `8080` | `8080` |
| Redis | `anchorscan-lab-redis` | `6379` | `6379` |
| Unknown TCP | `anchorscan-lab-unknown` | `9099` | `19099` |
| SSH | `anchorscan-lab-ssh` | `2222` | `10022` |
| SMB | `anchorscan-lab-samba` | `445` | `1445` |
| MariaDB | `anchorscan-lab-mariadb` | `3306` | `13306` |

Start the lab:

```bash
docker compose -f docker-compose.lab.yml up -d
```

If `mariadb:11` is slow to pull in your network, pre-pull it first:

```bash
docker pull mariadb:11
```

If you need a shell proxy for image pulls:

```bash
export http_proxy=http://127.0.0.1:7897
export https_proxy=http://127.0.0.1:7897
export all_proxy=http://127.0.0.1:7897
docker pull mariadb:11
```

Check status:

```bash
docker compose -f docker-compose.lab.yml ps
```

Get the real container IPs that AnchorScan should scan:

```bash
for c in \
  anchorscan-lab-tomcat \
  anchorscan-lab-redis \
  anchorscan-lab-mariadb \
  anchorscan-lab-ssh \
  anchorscan-lab-samba \
  anchorscan-lab-unknown
do
  printf '%s ' "$c"
  docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$c"
done
```

On macOS, direct access from the host to container IPs usually needs `docker-mac-net-connect`. If that is not installed, host-port checks like `127.0.0.1:8080` are fine for quick service validation, but the recommended AnchorScan lab path is to scan the real container IPs.

Example mixed-service run after resolving the real IPs:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target "$TOMCAT_IP,$REDIS_IP,$UNKNOWN_IP,$SSH_IP,$SMB_IP" \
  --ports 8080,6379,9099,2222,445 \
  --db data/scans.sqlite \
  --json reports/lab-mixed.json \
  --html reports/lab-mixed.html
```

You can also run service-specific checks:

```bash
go run ./cmd/anchorscan scan --config config/default.yaml --target <TOMCAT_IP> --ports 8080 --db data/scans.sqlite --json reports/tomcat.json --html reports/tomcat.html
go run ./cmd/anchorscan scan --config config/default.yaml --target <REDIS_IP> --ports 6379 --db data/scans.sqlite --json reports/redis.json --html reports/redis.html
go run ./cmd/anchorscan scan --config config/default.yaml --target <MARIADB_IP> --ports 3306 --db data/scans.sqlite --json reports/mariadb.json --html reports/mariadb.html
go run ./cmd/anchorscan scan --config config/default.yaml --target <SSH_IP> --ports 2222 --db data/scans.sqlite --json reports/ssh.json --html reports/ssh.html
go run ./cmd/anchorscan scan --config config/default.yaml --target <SMB_IP> --ports 445 --db data/scans.sqlite --json reports/smb.json --html reports/smb.html
go run ./cmd/anchorscan scan --config config/default.yaml --target <UNKNOWN_IP> --ports 9099 --db data/scans.sqlite --json reports/unknown.json --html reports/unknown.html
```

Stop and clean up the lab:

```bash
docker compose -f docker-compose.lab.yml down
```

## Output

- Progress logs go to stderr.
- Result paths go to stdout.
- JSON and HTML reports are written to the paths you choose.
- CLI 手工指定的 `--json` / `--html` 输出路径不会被项目删除自动清理。
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

### Automated E2E

If you want to verify the local lab end to end, run:

```bash
go test -tags=e2e ./e2e -v
```

What this suite currently covers:

- CLI scan with real Docker lab container IPs
- comma-separated multi-target CLI input
- custom port list scan (`8080,6379`)
- Web project creation
- project default target scan
- excluded target / excluded port behavior
- project deletion with managed file cleanup

Notes:

- The E2E suite does not use fake IPs. It starts or reuses `docker-compose.lab.yml`, then discovers the real container IPs with `docker inspect`.
- On macOS, if you want to reach container IPs directly from the host, keep `docker-mac-net-connect` running.
- The suite currently requires `rustscan` and `nmap` in your PATH, or set `ANCHORSCAN_RUSTSCAN` / `ANCHORSCAN_NMAP`.
