# AnchorScan Lab Troubleshooting

This is the short version. Start at the failing stage, not from the top.

## 1. No open ports found

Check:

- target service is actually listening
- `--ports` matches the service port
- rustscan path in `config/default.yaml` is correct
- local firewall / VM network is not blocking the path

Quick check:

```bash
ss -lntp
```

If you are testing the Docker lab from macOS, confirm you are scanning the real container IP and that `docker-mac-net-connect` is installed. Without it, a container can be healthy but still unreachable from the host by its bridge IP.

## 2. rustscan works, nmap finds nothing useful

Check:

- target accepts normal TCP connection but gives little banner data
- service exists but product detection is weak
- nmap path is correct
- full local scans may include non-standard app ports where `nmap -sV` is slow

Quick check:

```bash
<nmap-path> -sV --version-intensity 7 -p <PORT> <TARGET_IP> -oX -
```

If XML shows a service but no product, that is a fingerprint-quality issue, not an app crash.

If the terminal shows:

```text
[scan] nmap <target> still running elapsed=...
```

the scan is not stuck; `nmap -sV --version-intensity 7` is still probing service fingerprints. For lab checks, narrow `--ports` to the services under test.

## 3. Web target did not run httpx

Check:

- `service` contains `http` or `https`
- `product` contains a known web server string
- fingerprint was classified as `is_web=true`
- httpx path is correct

What to inspect:

- terminal log around `[scan] nmap`
- `fingerprints.is_web`
- `fingerprints.url`

## 4. NSE did not run

Check:

- normalized service matches a key in `config/nse.yaml`
- the target port has a saved fingerprint row
- nmap binary has the required NSE scripts available

Quick DB check:

```sql
select ip, port, service, product, version, normalized from fingerprints order by ip, port;
```

If `normalized` is not what you expected, fix the mapping or the lab assumption first.

## 5. nuclei did not run

Check:

- nuclei path is correct
- service or product matched a rule in `config/service-tags.yaml`
- for web assets, tech detection from httpx returned useful values
- the local nuclei templates actually contain the requested tags

Quick checks:

```bash
<nuclei-path> -tl
<nuclei-path> -target <TARGET> -tags <TAG>
```

## 6. Findings missing from report

Work in order:

1. confirm terminal logs show `[scan] nse` or `[scan] nuclei`
2. confirm `findings` table has rows
3. if DB has rows but report is empty, inspect JSON output
4. if DB has no rows, the problem is rule matching, tool execution, or parser output

DB check:

```sql
select ip, port, source, finding_id, severity, summary, target
from findings
order by ip, port, source, finding_id;
```

## 7. HTML report looks wrong but JSON looks right

This is almost always a rendering issue, not a scan issue.

Check:

- JSON contains the expected host/port/finding structure
- the same run id is being used for both outputs
- HTML file was regenerated after the latest run

## 8. Report filter/export looks empty

Check:

- `q` is a keyword search across asset and finding text
- `service` is an exact normalized service filter
- export endpoints use the same query string as the report page
- URL export only includes web assets with a saved URL
- CSV export should include `ip,port,service,product,version,url`

Quick checks:

```bash
curl 'http://127.0.0.1:8088/reports/<run_id>/assets.txt?kind=ip_port&q=redis'
curl 'http://127.0.0.1:8088/reports/<run_id>/assets.csv?q=redis'
```

If the report page shows data but export is empty, keep the exact URL and query string. That is likely a report endpoint bug, not a scanner bug.

## 9. Multi-service host mixes findings across ports

Check:

- JSON report groups findings under the correct `port`
- `findings` table rows use the correct `ip` and `port`
- the mixed host test uses a fresh output file

Quick check:

```bash
jq '.hosts[] | {ip, ports}' reports/<mixed>.json
```

## 10. Unknown service crashes the run

That is a real bug. Unknown should degrade to:

- fingerprint saved if available
- no forced web route
- no NSE if no rule matches
- no nuclei if no tag rule matches
- report still generated

Capture:

- exact command
- stderr logs
- saved JSON
- saved SQLite rows for the run

If the service stays stable and the run completes, a weak fingerprint like `unknown`, missing `product`, or no secondary routing is acceptable for this lab target.

## 11. Image pull is slow or stalls

The most common first-run case is `mariadb:11` taking longer than the other lab images. When every layer sits at `Pulling fs layer` and never progresses, the Docker Hub connection is being throttled or reset. Try the options below in order.

### 11a. Use a registry mirror (no local proxy needed)

Pull through a mirror, then re-tag it back to `mariadb:11` so `docker-compose.lab.yml` does not need to change:

```bash
docker pull docker.m.daocloud.io/library/mariadb:11
docker tag docker.m.daocloud.io/library/mariadb:11 mariadb:11
```

Other mirror prefixes if the first one is slow or down — rotate through them:

- `docker.1ms.run/library/mariadb:11`
- `docker.xuanyuan.me/library/mariadb:11`
- `hub.rat.dev/library/mariadb:11`

### 11b. Pin the mirror once for all pulls

Add a `registry-mirrors` entry to Docker Desktop (Settings → Docker Engine, i.e. `~/.docker/daemon.json`):

```json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
```

Apply & Restart, then a plain `docker pull mariadb:11` routes through the mirror automatically.

### 11c. Use a local proxy

If you already run a local proxy (e.g. Clash on `127.0.0.1:7897`):

```bash
export http_proxy=http://127.0.0.1:7897
export https_proxy=http://127.0.0.1:7897
export all_proxy=http://127.0.0.1:7897
docker pull mariadb:11
```

### After the image is available

Restart the lab:

```bash
docker compose -f docker-compose.lab.yml up -d
```

## 12. Minimal triage flow

When a lab case fails, answer these in order:

1. Did rustscan find the port?
2. Did nmap produce a usable fingerprint?
3. Was the service classified correctly?
4. Did the right secondary tool run?
5. Did findings land in SQLite?
6. Did JSON/HTML render those findings?

Do not debug all layers at once. Start at the first wrong answer.

## Web Console does not start

Check listen address, DB path, and config path.

## Scan cannot start from Web

Check if another scan is running. v1.3 allows one active scan.

## Cancel does not work

Cancel only affects scans started by the running Web Console process. Confirm `anchorscan cancel --server` points to the active Web Console.
