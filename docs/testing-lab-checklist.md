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

## Recommended Lab Targets

Use at least one target for each service family:

| Service | Example | Why |
| --- | --- | --- |
| Tomcat | `8080/tcp` | Full web path: fingerprint -> httpx -> NSE -> nuclei |
| Redis | `6379/tcp` | Full non-web path with both NSE and nuclei |
| MySQL / MariaDB | `3306/tcp` | Database fingerprint normalization and rules |
| SMB | `445/tcp` | Alias normalization and NSE validation |
| SSH | `22/tcp` | Common non-web baseline |
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

For the bundled Docker lab, start small:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target 127.0.0.1 \
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

| Check | Expected |
| --- | --- |
| Open port | `3306` found |
| Fingerprint | MySQL or MariaDB identified |
| Normalized | `mysql` |
| NSE | mysql rules run |
| nuclei | `mysql` / `mariadb` tags run |

### 4. SMB

| Check | Expected |
| --- | --- |
| Open port | `445` found |
| Fingerprint | `microsoft-ds`, `netbios-ssn`, or smb-like |
| Normalized | `smb` |
| NSE | smb rules run |
| nuclei | `smb` tags run |

### 5. SSH

| Check | Expected |
| --- | --- |
| Open port | `22` found |
| Fingerprint | ssh/OpenSSH identified |
| Routing | non-web |
| httpx | skipped |
| NSE | ssh rules may run |
| Report | fingerprint exists even if findings are empty |

### 6. Unknown Service

| Check | Expected |
| --- | --- |
| Open port | found |
| Fingerprint | weak or unknown is acceptable |
| Routing | should not be forced to web |
| Stability | no crash, report still generated |

### 7. Mixed Host

Run one host with 2-3 services, for example `8080,6379,3306`.

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
| Packaged doctor | Reports config, tool, database, and reports checks |
| CLI scan preflight | Prints target count, ports, profile, and workers before rustscan |
| Web scan preflight | Blocks missing required tools before scan starts |
| Existing DB open | Migrates old schema and preserves rows |
