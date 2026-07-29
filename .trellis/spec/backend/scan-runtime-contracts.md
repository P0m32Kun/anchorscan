# Scan Runtime Contracts

## Scenario: Prepared scan runtime resources and discovery mode

### 1. Scope / Trigger

- `app.PrepareScan` is the single boundary that loads runtime rules and builds `ScanOptions` for CLI and Web submissions.
- Release archives must carry the rule and port resources that this boundary consumes. Missing routing rules otherwise turn real checks into historical `skipped/no_matching_rule` facts.

### 2. Signatures

- CLI: `anchorscan scan --discovery auto|assume-up`.
- Web form/JSON: `discovery_mode: "auto" | "assume-up"`.
- Preparation: `app.PrepareScan(app.PrepareScanRequest{DiscoveryMode: ...})`.
- Execution: `app.RunScan(ctx, runner, store, app.ScanOptions{DiscoveryMode: ...})` validates again before taking a run lease.
- Snapshot/report: `discovery_mode` is stored in `ConfigSnapshot` and emitted by `report.ScanReport`.

### 3. Contracts

- `nse.yaml` and `service-tags.yaml` are mandatory sidecars beside the selected configuration. `LoadNSERulesForConfig` and `LoadTagRulesForConfig` may fall back to `config/` only when its files are usable; absent, empty, or malformed rules are errors.
- `doctor` uses those same loaders, so a healthy rule check means a scan can load the rules.
- `auto` performs the nmap alive sweep. `assume-up` keeps the parsed and authorized target list, skips only that sweep, and continues port/fingerprint/vulnerability stages.
- Empty historical or malformed discovery snapshots resolve to `auto`; valid `assume-up` is retained on rerun.
- Package exactly `default.yaml.example`, `nse.yaml`, `service-tags.yaml`, `ports-highrisk.txt`, `ports-top100.txt`, and `ports-top1000.txt`. Never wildcard-copy local `default.yaml`.

### 4. Validation & Error Matrix

| Condition | Required behavior |
|---|---|
| Missing, empty, or malformed required rule sidecar | `PrepareScan` and `doctor` return/report an actionable failure; do not start tools. |
| Invalid discovery mode at CLI/Web/PrepareScan | Reject with `invalid discovery mode`. |
| Invalid discovery mode passed directly to `RunScan` | Reject before lease/store/runner use. |
| Missing or invalid stored snapshot mode | Rerun/report use `auto`. |
| `assume-up` | Do not invoke nmap alive discovery; use authorized targets as alive targets. |

### 5. Good/Base/Bad Cases

- Good: a release-shaped `package/config/default.yaml` has adjacent non-empty rules; SSH invokes NSE plus nuclei, Tomcat invokes nuclei against its URL, and X11 invokes nuclei with `-tags x11` against `IP:port`.
- Base: older runs without `discovery_mode` remain rerunnable and report `auto`.
- Bad: treating a missing sidecar as an empty map, or trusting an invalid direct `ScanOptions.DiscoveryMode` because a caller bypassed `PrepareScan`.

### 6. Tests Required

- Loader and doctor tests cover missing, empty, and malformed sidecars.
- Execution tests build `package/config/default.yaml` with real sidecars, call `PrepareScan`, and assert SSH/Tomcat/X11 tool arguments plus completed nuclei checks for zero findings.
- Tests assert invalid `RunScan` modes fail before dependencies are used and invalid rerun snapshots render `auto`.
- `make package VERSION=<test>` must pass; inspect the tar entries for all six required resources.

### 7. Wrong vs Correct

#### Wrong

```go
if os.IsNotExist(err) {
    return emptyRules, nil // scans silently skip all routed checks
}
```

#### Correct

```go
if _, err := config.LoadTagRulesForConfig(configPath); err != nil {
    return PreparedScan{}, err // fail before preflight and tool execution
}
```

Do not change persisted `DetectionCheck` rows when rules change. They are facts about the historical run, not a recalculated view of current configuration.

## Scenario: Zone-scoped verification aggregation

### 1. Scope / Trigger

- A project network zone, not an individual scan run or target subnet, is the verification and report aggregation boundary.
- This contract spans the Vue workbench, verification HTTP endpoints, store JSON decoding, command generation, and DOCX context.

### 2. Signatures

- Create: `POST /projects/{projectID}/verifications` with `zone_id`, `vulnerability_key`, and optional `assets` / `sources`.
- Update: `POST /projects/{projectID}/verifications/{verificationID}` with the editable verification fields; it does not replace assets or sources.
- Candidate command form: `POST /projects/{projectID}/candidates/{vulnerabilityKey}/commands` with `zone_id`.

### 3. Contracts

- JSON association fields are snake_case: assets use `asset_name`; sources use `run_id` and `finding_id`.
- The frontend lookup identity is `(zone_id, vulnerability_key)`. Identical vulnerabilities in different zones must never share edit state or evidence.
- Within one zone, matching vulnerabilities from any included runs aggregate their assets and sources into one candidate and one verification.
- DOCX outputs one chapter per zone and joins unique access points, tester IPs, targets, exclusions, and notes in first-seen order; it must not create a subchapter per scan run.
- DOCX multi-line zone context fields (`access_points_text`, `tester_ips_text`, `targets_text`, `exclusions_text`, `notes_text`) are newline-delimited strings. The template must render each value as a separate, indented paragraph using a Jinja loop over `splitlines()`; never as a single paragraph with embedded literal newlines.
- Verification descriptions are provided as a pre-split list `description_lines`. The template must render each element as a separate paragraph with the same first-line indent as the surrounding body text; do not render the raw `description` string as a single paragraph.

### 4. Validation & Error Matrix

| Condition | Required behavior |
|---|---|
| Create or update `zone_id` is absent or not owned by the project | HTTP 400: `zone_id is not part of this project`. |
| Candidate command has `zone_id` | Resolve the candidate only inside that zone. |
| Same vulnerability key in two zones | Maintain two independent verification records and UI entries. |
| DOCX layout verification | CI gate is `make docx-test` (OOXML structure + rendered content assertions). LibreOffice/PDF rasterization is a local diagnostic only, not an acceptance gate. |

### 5. Good/Base/Bad Cases

- Good: two I-zone runs for different subnets both find SSH weak credentials; the workbench has one I-zone candidate with both IPs, and the report lists both scan access contexts in its one I-zone chapter.
- Base: a single run retains the same zone-level lists and verification behavior.
- Bad: keying the workbench map only by `vulnerability_key`, or silently decoding snake_case association fields into zero values.

### 6. Tests Required

- HTTP regression: create verification with snake_case asset/source fields, assert they persist, then upload PNG evidence.
- HTTP regression: reject updates that move a verification outside its project zones.
- HTTP regression: update a `not_observed` verification's title/description/severity while it has evidence.
- Frontend static/type check: verify snake_case payloads, composite zone/key identity, and command `zone_id` propagation.
- Report unit/render tests: two included runs in one zone produce one zone chapter and deduplicated aggregated access context; each access point/IP/target renders in its own indented paragraph; each `description_lines` paragraph has consistent first-line indent.
- Web browser smoke: verification create/update payloads use snake_case and preserve multi-run assets/sources; footer shows the linked version without a `v` prefix. |

### 7. Wrong vs Correct

#### Wrong

```ts
verificationMap[v.vulnerability_key] = v
```

#### Correct

```ts
verificationMap[`${v.zone_id}\x00${v.vulnerability_key}`] = v
```

## Scenario: Release version injection

### 1. Scope / Trigger

- The visible CLI and Web version must be derived from the release tag, not a manually maintained Go constant.

### 2. Signatures

- Build input: `make build VERSION=vX.Y.Z` or `make package VERSION=vX.Y.Z`.
- Verification: `make release-check VERSION=vX.Y.Z`.
- Linker target: `github.com/P0m32Kun/anchorscan/internal/version.Version`.

### 3. Contracts

- `version.Version` is a mutable development fallback (`dev`) so Go linker `-X` can override it.
- Build display version strips only the leading `v`; both CLI and Web consume the same linked value.
- The release workflow runs `release-check` against the Git tag before producing cross-platform archives.

### 4. Validation & Error Matrix

| Condition | Required behavior |
|---|---|
| Host release check build | `anchorscan version` exactly equals `anchorscan version X.Y.Z`. |
| Cross-compiled target | Do not execute its binary locally; package it after host release check passes. |

### 5. Good/Base/Bad Cases

- Good: tag `v2.0.1` renders as `2.0.1` in CLI and Web.
- Base: local untagged builds display `dev` or the configured make version.
- Bad: keeping a source constant synchronized by hand with tags.

### 6. Tests Required

- Run `make release-check VERSION=v9.8.7` and `make package VERSION=v9.8.7` in CI or release verification.

### 7. Wrong vs Correct

#### Wrong

```go
const Version = "2.0.1"
```

#### Correct

```go
var Version = "dev" // linked with -X during the build
```

## Scenario: Dameng driver failure containment

### 1. Scope / Trigger

- `gitee.com/chunanyong/dm` is an untrusted third-party dependency on the default-password check path and can panic while opening or handshaking a connection.
- The Dameng checker must turn only driver/checker panics and deadline expiry into an auditable optional-engine failure; worker orchestration must not recover arbitrary scan panics.

### 2. Signatures

- Tool boundary: `tools.RunDamengDefaultPassword(ctx context.Context, checker DamengAuthChecker, ip string, port int) (DamengResult, error)`.
- Driver seam: `DamengAuthChecker.Check(ctx, host, port, username, password) (ok bool, detail string, err error)`.
- New-install configuration: `timeouts.dameng: "15s"`; an explicitly configured `"0"` retains the existing no-deadline behavior.

### 3. Contracts

- Recover at a helper whose body is only the `checker.Check(...)` call. Its recovered error text starts with `dameng driver panic:`.
- A recovered driver panic or `context.DeadlineExceeded` returns `DamengUnknown` **and a non-nil error**. `scanTarget` then persists `failed/command_failed` with the diagnostic detail, writes no Finding, and completes the Run as `completed_with_errors` under its existing partial-failure aggregation.
- Authentication rejection remains `DamengSafe`; successful authentication remains `DamengVulnerable`; ordinary connection or protocol errors remain `DamengUnknown` with a nil error.

### 4. Validation & Error Matrix

| Condition | `RunDamengDefaultPassword` result | Scan persistence |
|---|---|---|
| `checker.Check` panics | `DamengUnknown`, non-nil `dameng driver panic:` error | `failed/command_failed`, diagnostic Detail, no Finding |
| Checker returns/wraps `context.DeadlineExceeded` | `DamengUnknown`, non-nil error | `failed/command_failed`, deadline Detail, no Finding |
| Authentication rejection | `DamengSafe`, nil error | completed check, no Finding |
| Ordinary non-auth connection/protocol error | `DamengUnknown`, nil error | existing completed unknown path |
| Authentication succeeds | `DamengVulnerable`, nil error | completed check and default-password Finding |

### 5. Good/Base/Bad Cases

- Good: a new configuration uses `15s`; a user can deliberately retain unlimited behavior with `dameng: "0"`.
- Base: a non-auth network error is still an unknown result rather than a scan engine failure.
- Bad: recovering at `RunScan`/a worker goroutine, which masks unrelated application defects; or returning nil for a driver panic, which falsely records a completed unknown check.

### 6. Tests Required

- Tool tests inject a panicking checker and a deadline error; assert the panic diagnostic/non-nil error and retain vulnerable, safe, and ordinary-network mappings.
- App/store tests for both panic and deadline assert `completed_with_errors`, Dameng `failed/command_failed`, diagnostic `DetectionCheck.Detail`, and zero Findings.
- Config tests assert generated and shipped defaults are `15s`, and `ToolTimeouts.Durations()` accepts explicit Dameng `0`.

### 7. Wrong vs Correct

#### Wrong

```go
ok, detail, err := checker.Check(ctx, ip, port, "SYSDBA", "SYSDBA") // panic escapes scan worker
if err != nil {
    return DamengResult{Verdict: DamengUnknown, Output: detail}, nil
}
```

#### Correct

```go
ok, detail, err := callDamengChecker(ctx, checker, ip, port) // recovery is scoped to Check
var panicErr *damengDriverPanicError
if errors.As(err, &panicErr) || errors.Is(err, context.DeadlineExceeded) {
    return DamengResult{Verdict: DamengUnknown, Output: err.Error()}, err
}
```
