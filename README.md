# anchorscan

`anchorscan` is a portable internal network scanner for authorized environments. It chains `rustscan` for port discovery, `nmap -sV` for service fingerprinting, `httpx` for web enrichment, and fingerprint-driven `nuclei`/NSE checks, then stores results in SQLite and renders JSON/HTML reports.

## V1 MVP Status

V1 MVP is feature-complete for local lab testing:

- configurable external tool paths
- fixed pipeline: rustscan -> nmap fingerprinting -> fingerprint-driven httpx / NSE / nuclei
- custom ports, `top100`, `top1000`, and `full`
- SQLite persistence
- JSON and HTML reports
- terminal progress logs, including nmap heartbeat while `-sV` is still running

## Quick Start

1. Edit [config/default.yaml](./config/default.yaml) so the tool paths point at your local binaries.
2. Run a scan:

```bash
go run ./cmd/anchorscan scan \
  --config config/default.yaml \
  --target 127.0.0.1 \
  --db data/scans.sqlite \
  --json reports/test.json \
  --html reports/test.html
```

3. Rebuild a report from stored data:

```bash
go run ./cmd/anchorscan report \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --run-id <run_id> \
  --json reports/rerender.json \
  --html reports/rerender.html
```

4. Check configured tools:

```bash
go run ./cmd/anchorscan tools check --config config/default.yaml
```

## V1.1 Web Console

Start local Web Console:

```bash
go run ./cmd/anchorscan web \
  --config config/default.yaml \
  --db data/scans.sqlite \
  --listen 127.0.0.1:8088
```

Open http://127.0.0.1:8088.

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
