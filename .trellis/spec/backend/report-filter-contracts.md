# Report Filter Contracts

## Scenario: Run report service filters

### 1. Scope / Trigger

This cross-layer report contract adds service facets and the `exclude_unidentified=1` read-only query. It affects the report page, HTML export, assets.txt, and command endpoints without changing persisted Fingerprint, Finding, or DetectionCheck facts.

### 2. Signatures

- Query: `service=<raw service>` retains exact positive service filtering.
- Query: `exclude_unidentified=1` excludes the fixed raw service set: `tcpwrapped`, `unknown`, and `""`.
- View model: `service_facets: [{raw_value, label, count}]` is supplied to the report interaction mount point.

### 3. Contracts

- `runReportReading` is the sole report filter boundary. All report consumers must use its filtered facts.
- Facets apply IP, port, and keyword constraints, but must omit `service` and `exclude_unidentified` so available options do not self-filter.
- Facets are ordered by `raw_value`; an empty raw value uses the label `未识别（空）` and is informational, not a selectable `service` value.
- Changing report service filters removes `assets_page` and `findings_page` while retaining other query parameters.

### 4. Validation & Error Matrix

| Input | Result |
| --- | --- |
| `exclude_unidentified=1` | Exclude only the fixed three raw service values. |
| Other `exclude_unidentified` values | Do not exclude services. |
| Finding has an empty protocol | Associate it to a Fingerprint only when its IP:port has exactly one Fingerprint protocol. |
| Finding has no associated Fingerprint | Keep it visible; report filtering must not discard independent Finding facts. |

### 5. Good / Base / Bad Cases

- Good: `tcpwrapped/tcp` Fingerprint and protocol-less Finding on its unique IP:port are both excluded.
- Base: no service query preserves every Fingerprint and Finding.
- Bad: matching Findings by IP:port:protocol only leaks protocol-less Findings after their unique Fingerprint is excluded.

### 6. Tests Required

- Go unit tests cover the fixed unidentified set, facet counts/sort order, and protocol-less Finding fallback.
- Handler/view tests prove HTML/export URLs preserve filtering and omit pagination keys.
- Playwright smoke selects the report control, checks the facet labels/counts, verifies both page keys reset, and clears the active exclusion.

### 7. Wrong vs Correct

#### Wrong

Filter Fingerprints but leave Findings matched only by exact protocol. A historical Finding with an empty protocol then becomes an unintended orphan in the report.

#### Correct

Use the same unique-protocol fallback used by report construction when deciding whether a Finding belongs to a filtered Fingerprint; retain only genuinely unassociated Findings.
