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
select ip, port, service, normalized from fingerprints order by ip, port;
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

## 8. Multi-service host mixes findings across ports

Check:

- JSON report groups findings under the correct `port`
- `findings` table rows use the correct `ip` and `port`
- the mixed host test uses a fresh output file

Quick check:

```bash
jq '.hosts[] | {ip, ports}' reports/<mixed>.json
```

## 9. Unknown service crashes the run

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

## 10. Minimal triage flow

When a lab case fails, answer these in order:

1. Did rustscan find the port?
2. Did nmap produce a usable fingerprint?
3. Was the service classified correctly?
4. Did the right secondary tool run?
5. Did findings land in SQLite?
6. Did JSON/HTML render those findings?

Do not debug all layers at once. Start at the first wrong answer.
