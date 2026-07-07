# AnchorScan Lab Results Template

Copy this file per lab round, for example:

- `docs/lab-results-2026-07-07.md`

## Run Metadata

| Field | Value |
| --- | --- |
| Date | |
| Operator | |
| Branch / Commit | |
| Config file | |
| DB path | |
| JSON path | |
| HTML path | |

## Case Summary

| Case | Target | Ports | Fingerprint OK | NSE Triggered | Nuclei Triggered | Report OK | Result | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Tomcat | | 8080 | | | | | | |
| Redis | | 6379 | | | | | | |
| MySQL / MariaDB | | 3306 | | | | | | |
| SMB | | 445 | | | | | | |
| SSH | | 22 | | | | | | |
| Unknown service | | | | | | | | |
| Mixed host | | | | | | | | |

## Per-Case Notes

### Tomcat

- Command:
- Observed fingerprint:
- Observed findings:
- JSON/HTML check:
- Follow-up:

### Redis

- Command:
- Observed fingerprint:
- Observed findings:
- JSON/HTML check:
- Follow-up:

### MySQL / MariaDB

- Command:
- Observed fingerprint:
- Observed findings:
- JSON/HTML check:
- Follow-up:

### SMB

- Command:
- Observed fingerprint:
- Observed findings:
- JSON/HTML check:
- Follow-up:

### SSH

- Command:
- Observed fingerprint:
- Observed findings:
- JSON/HTML check:
- Follow-up:

### Unknown service

- Command:
- Observed fingerprint:
- Observed findings:
- JSON/HTML check:
- Follow-up:

### Mixed host

- Command:
- Observed ports:
- Finding ownership check:
- JSON/HTML check:
- Follow-up:

## SQLite Validation

### Fingerprints

```sql
select run_id, ip, port, service, product, normalized, is_web, url
from fingerprints
order by ip, port;
```

Notes:

- 

### Findings

```sql
select run_id, ip, port, source, finding_id, severity, summary, target
from findings
order by ip, port, source, finding_id;
```

Notes:

- 

## Final Verdict

| Area | Status | Notes |
| --- | --- | --- |
| Port discovery | | |
| Fingerprinting | | |
| Web routing | | |
| Non-web routing | | |
| NSE execution | | |
| Nuclei execution | | |
| SQLite persistence | | |
| JSON report | | |
| HTML report | | |
| Stability | | |

## Bugs / Follow-Ups

- [ ] 
