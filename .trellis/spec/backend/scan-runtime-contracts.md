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
