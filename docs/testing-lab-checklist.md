# AnchorScan V1 Local Lab Checklist

This checklist is for the first local lab pass of AnchorScan V1. Keep it small: verify the fixed pipeline works, confirm findings land in SQLite, and confirm JSON/HTML reports are readable.

## Scope

Verify these paths only:

- rustscan port discovery
- nmap fingerprinting
- web routing to httpx + nuclei
- non-web routing to NSE + nuclei
- SQLite persistence
- JSON / HTML report generation
- terminal progress logs, including nmap heartbeat during slow service detection
- Web report filtering, host aggregation, copy/export actions

## Automated E2E Baseline

Before manual walkthroughs, you can run the automated baseline:

```bash
go test -tags=e2e ./e2e -v
```

Current automated coverage:

- CLI 多 IP 扫描
- 终端英文逗号分隔目标
- 指定端口列表扫描
- Web 项目创建
- 项目默认目标发起扫描
- 排除目标
- 排除端口
- 项目删除时联动清理数据库与托管报告目录

说明：

- 自动化不会使用不存在的测试 IP，而是直接从 `docker-compose.lab.yml` 启动或复用容器，并通过 `docker inspect` 获取真实容器 IP。
- 如果你在 macOS 上跑这套用例，并希望从宿主机直接访问容器 IP，请保持 `docker-mac-net-connect` 可用。

## Lab Startup Baseline

Start or refresh the local lab:

```bash
docker compose -f docker-compose.lab.yml up -d
```

If `mariadb:11` is slow to pull, pre-pull it before the lab run:

```bash
docker pull mariadb:11
```

如果直连 Docker Hub 卡在 `Pulling fs layer`，优先走镜像加速源（不需要本机有代理），拉完 re-tag 回 `mariadb:11`，这样 compose 文件不用改：

```bash
docker pull docker.m.daocloud.io/library/mariadb:11
docker tag docker.m.daocloud.io/library/mariadb:11 mariadb:11
```

其它可用的 mirror 前缀（轮着试）：`docker.1ms.run`、`docker.xuanyuan.me`、`hub.rat.dev`。
一劳永逸：给 Docker Desktop（Settings → Docker Engine，即 `~/.docker/daemon.json`）加 `registry-mirrors`，之后 `docker pull mariadb:11` 会自动走 mirror。

如果你已有本地代理（如 Clash），仍可用代理方式：

```bash
export http_proxy=http://127.0.0.1:7897
export https_proxy=http://127.0.0.1:7897
export all_proxy=http://127.0.0.1:7897
docker pull mariadb:11
```

Resolve the real container IPs:

```bash
TOMCAT_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-tomcat)
REDIS_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-redis)
MARIADB_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-mariadb)
SSH_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-ssh)
SMB_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-samba)
UNKNOWN_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' anchorscan-lab-unknown)
```

Recommended service-to-port map:

| Service | Container | Real port |
| --- | --- | --- |
| Tomcat | `anchorscan-lab-tomcat` | `8080` |
| Redis | `anchorscan-lab-redis` | `6379` |
| MariaDB | `anchorscan-lab-mariadb` | `3306` |
| SSH | `anchorscan-lab-ssh` | `2222` |
| SMB | `anchorscan-lab-samba` | `445` |
| Unknown TCP | `anchorscan-lab-unknown` | `9099` |

## Recommended Lab Targets

Use at least one target for each service family:

| Service | Example | Why |
| --- | --- | --- |
| Tomcat | `8080/tcp` | Full web path: fingerprint -> httpx -> NSE -> nuclei |
| Redis | `6379/tcp` | Full non-web path with both NSE and nuclei |
| MySQL / MariaDB | `3306/tcp` | Database fingerprint normalization and rules |
| SMB | `445/tcp` | Alias normalization and NSE validation |
| SSH | `2222/tcp` | Common non-web baseline |
| Unknown service | custom listener | Stability check: no misroute, no crash |
| Mixed host | one host with 2-3 services | Report grouping and finding ownership |

## Standard Command

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <TARGET_IP> \
  --db data/scans.sqlite \
  --json reports/<name>.json \
  --html reports/<name>.html
```

Custom ports:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <TARGET_IP> \
  --ports 8080,6379,3306,445 \
  --db data/scans.sqlite \
  --json reports/<name>.json \
  --html reports/<name>.html
```

For the bundled Docker lab, prefer the real container IPs rather than `127.0.0.1`:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target "$TOMCAT_IP,$REDIS_IP" \
  --ports 8080,6379 \
  --db data/scans.sqlite \
  --json reports/lab.json \
  --html reports/lab.html
```

Use `--ports full` or `--ports 1-65535` only when you intentionally want every local listening service. Full local scans often include app IPC/proxy ports, and `nmap -sV` can be slow on those ports.

## Pass Criteria For Every Run

Check these before moving on:

1. Terminal logs show the expected stages
2. `fingerprints` table has rows
3. `findings` table has rows when rules/templates match
4. JSON report nests findings under the correct port
5. HTML report shows the same findings summary

## Expected Log Markers

At minimum:

- `[scan] run`
- `[scan] target`
- `[scan] rustscan`
- `[scan] nmap ... (service detection may be slow)`
- `[scan] nmap ... still running elapsed=...` for long `nmap -sV` runs
- `[scan] nmap ... services=... elapsed=...`
- `[scan] httpx` for web targets
- `[scan] nse` when NSE rules match
- `[scan] nuclei` when nuclei tags match
- `[scan] report json`
- `[scan] report html`
- `[scan] done`

## Test Cases

### 1. Tomcat

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <TOMCAT_IP> \
  --ports 8080 \
  --db data/scans.sqlite \
  --json reports/tomcat.json \
  --html reports/tomcat.html
```

Expected:

| Check | Expected |
| --- | --- |
| Open port | `8080` found |
| Fingerprint | service like `http`, product includes `Tomcat` |
| Routing | `is_web=true` |
| httpx | URL, title, status, tech |
| NSE | tomcat/http rules run |
| nuclei | tomcat tags run |
| Report | findings under port `8080` |

### 2. Redis

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target <REDIS_IP> \
  --ports 6379 \
  --db data/scans.sqlite \
  --json reports/redis.json \
  --html reports/redis.html
```

Expected:

| Check | Expected |
| --- | --- |
| Open port | `6379` found |
| Fingerprint | redis identified |
| Routing | `is_web=false` |
| httpx | skipped |
| NSE | `redis-info` or mapped scripts run |
| nuclei | `redis` tags run |
| Report | findings under port `6379` |

### 3. MySQL / MariaDB

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target "$MARIADB_IP" \
  --ports 3306 \
  --db data/scans.sqlite \
  --json reports/mariadb.json \
  --html reports/mariadb.html
```

| Check | Expected |
| --- | --- |
| Open port | `3306` found |
| Fingerprint | MySQL or MariaDB identified |
| Normalized | `mysql` |
| NSE | mysql rules run |
| nuclei | `mysql` / `mariadb` tags run |

### 4. SMB

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target "$SMB_IP" \
  --ports 445 \
  --db data/scans.sqlite \
  --json reports/smb.json \
  --html reports/smb.html
```

| Check | Expected |
| --- | --- |
| Open port | `445` found |
| Fingerprint | `microsoft-ds`, `netbios-ssn`, or smb-like |
| Normalized | `smb` |
| NSE | smb rules run |
| nuclei | `smb` tags run |

### 5. SSH

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target "$SSH_IP" \
  --ports 2222 \
  --db data/scans.sqlite \
  --json reports/ssh.json \
  --html reports/ssh.html
```

| Check | Expected |
| --- | --- |
| Open port | `22` found |
| Fingerprint | ssh/OpenSSH identified |
| Routing | non-web |
| httpx | skipped |
| NSE | ssh rules may run |
| Report | fingerprint exists even if findings are empty |

### 6. Unknown Service

Command:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target "$UNKNOWN_IP" \
  --ports 9099 \
  --db data/scans.sqlite \
  --json reports/unknown.json \
  --html reports/unknown.html
```

| Check | Expected |
| --- | --- |
| Open port | found |
| Fingerprint | weak or unknown is acceptable |
| Routing | should not be forced to web |
| Stability | no crash, report still generated |

### 7. Mixed Host

Run one host with 2-3 services, or one multi-target command that combines the lab IPs, for example `8080,6379,3306`.

Expected:

| Check | Expected |
| --- | --- |
| Host count | one host node |
| Port count | one node per service |
| Finding ownership | findings stay on the correct port |
| HTML | all ports shown under the same host |

## SQLite Checks

Open DB:

```bash
sqlite3 data/scans.sqlite
```

Fingerprints:

```sql
select run_id, ip, port, service, product, version, normalized, is_web, url
from fingerprints
order by ip, port;
```

Findings:

```sql
select run_id, ip, port, source, finding_id, severity, summary, target
from findings
order by ip, port, source, finding_id;
```

## JSON Checks

```bash
jq . reports/tomcat.json
```

Expect:

- `scan_meta.tool == "anchorscan"`
- `hosts[].ports[]` exists
- `findings[]` is nested under the correct port

## First-Pass Order

Keep the first pass in this order:

1. Tomcat
2. Redis
3. MySQL / MariaDB
4. SMB
5. Mixed host
6. Unknown service

## Exit Rule

V1 lab pass is good enough when:

- web and non-web routing both work
- findings land in SQLite
- JSON and HTML agree on results
- multi-service reports do not mix findings across ports
- unknown services do not break the run

## V1.1 Web Lab

1. Start Web Console.
2. Run doctor from Home.
3. Create Local Lab project.
4. Start scan with `normal` profile and ports `8080,6379`.
5. Confirm progress events update.
6. Confirm report page shows Redis and Tomcat assets.
7. Use keyword `redis`, confirm only Redis-related assets/findings remain.
8. Switch asset view to `按主机聚合`, confirm one row per host and port list is correct.
9. Use `复制 IP:PORT`, confirm clipboard content is usable as `host:port` lines.
10. Open TXT/CSV export, confirm the exported rows match the current filter.
11. Start a long full-port scan and cancel it.
12. Confirm canceled status and event log.

## Report Workbench Checks

| Check | Expected |
| --- | --- |
| Keyword search `redis` | Redis assets/findings remain, unrelated Tomcat rows are hidden |
| Host aggregation view | Assets are grouped by IP and show the expected port list |
| Copy `IP` | Clipboard contains one IP per line |
| Copy `IP:PORT` | Clipboard contains one `host:port` per line |
| Copy `URL` | Clipboard contains only web assets with saved URLs |
| TXT export | Output follows the selected kind and current filters |
| CSV export | Header includes `ip,port,service,product,version,url` and rows match current filters |

## V1.2 Deployment And Preflight Checks

| Check | Expected |
| --- | --- |
| `make package` | Creates `dist/<package>/` and `dist/<package>.tar.gz` |
| `make e2e` | Runs CLI/Web smoke against the local lab with real binaries |
| Packaged doctor | Reports config, tool, database, and reports checks |
| CLI scan preflight | Prints target count, ports, profile, and workers before rustscan |
| Web scan preflight | Blocks missing required tools before scan starts |
| Existing DB open | Migrates old schema and preserves rows |

## E2E Smoke Command

```bash
make e2e
```

Expected:

- CLI smoke writes SQLite / JSON / HTML successfully
- JSON report contains `6379` and `8080`
- Web smoke can start a scan and open the generated report page

## V1.3 Single Tool Runs Lab

V1.3 lets an operator run one engine outside the automated pipeline. Each run still writes to SQLite and produces a JSON report, so it shows up under the same `/runs/<run_id>` and `/reports/<run_id>` pages.

### CLI Single Tool Runs

Use the lab container IPs resolved above. Keep `--ports` narrow.

| Tool | Command | Expected |
| --- | --- | --- |
| rustscan | `anchorscan tool rustscan --target <IP> --ports 8080,6379 --db data/scans.sqlite --json reports/tool-rustscan.json` | Open ports land in fingerprints table |
| nmap alive | `anchorscan tool nmap --mode alive --target <IP> --db data/scans.sqlite --json reports/tool-nmap-alive.json` | Host-up info finding recorded |
| nmap service | `anchorscan tool nmap --mode service --target <IP> --ports 6379 --db data/scans.sqlite --json reports/tool-nmap-service.json` | redis fingerprint saved, normalized `mysql`/`redis` as expected |
| httpx | `anchorscan tool httpx --url http://<IP>:8080 --db data/scans.sqlite --json reports/tool-httpx.json` | Web fingerprint with title/tech saved |
| nuclei (tags) | `anchorscan tool nuclei --url http://<IP>:8080 --tags tomcat,cve --db data/scans.sqlite --json reports/tool-nuclei-tags.json` | Matching findings recorded |
| nuclei (template) | `anchorscan tool nuclei --url http://<IP>:8080 --template cves/2021/CVE-2021-41773.yaml --db data/scans.sqlite --json reports/tool-nuclei-template.json` | Template finding recorded if it matches |
| native args | `anchorscan tool nmap --target <IP> --args "--version" --db data/scans.sqlite --json reports/tool-native.json` | Tool runs with the raw flags, no crash |

Pass criteria for each run:

1. Command exits 0.
2. `findings` or `fingerprints` table has rows for the new run id.
3. JSON report is written and openable.
4. The run appears on the Web Console runs list.

### BlueKeep Manual Review

When nmap service mode detects an RDP endpoint, the pipeline emits a manual-review finding instead of auto-confirming BlueKeep.

| Check | Expected |
| --- | --- |
| Trigger | Scan a host with RDP exposed (or a lab listener on 3389) via nmap service mode |
| Finding source | `source = manual-review` |
| Finding id | `manual-review:CVE-2019-0708` |
| Severity | `critical` |
| Summary | Mentions BlueKeep / RDP verification |

Quick DB check:

```sql
select source, finding_id, severity, summary from findings
where source = 'manual-review';
```

### Web Console Single Tool Page

1. Open `/tools/new` (or sidebar 单工具调用).
2. Pick a tool card (rustscan / nmap / httpx / nuclei).
3. Submit the per-tool form with a lab target.
4. Confirm the run starts and shows up under runs.
5. Open its report page; confirm the tool result is present.

### Version Checks

| Check | Expected |
| --- | --- |
| `anchorscan version` | Prints `anchorscan version 1.5.1` |
| `anchorscan --version` | Same as above |
| Web Console footer | Shows `AnchorScan Console v1.5.1` |
