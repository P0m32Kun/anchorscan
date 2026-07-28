# Technical Design

## 1. Scope and boundaries

This task repairs one release-trust path with two observable failures:

1. a standalone archive lacks mandatory runtime configuration and silently routes no vulnerability checks;
2. partial discovery-mode edits leave the Web binary uncompilable.

The implementation remains inside existing boundaries:

- `internal/config` owns locating, parsing, and validating sidecar rule files;
- `internal/doctor` reports the same rule-loading contract to operators;
- `internal/app` prepares scans, validates discovery mode, and executes discovery behavior;
- `internal/vuln` remains the pure rule matcher;
- `internal/report` exposes the selected discovery mode without changing historical DetectionChecks;
- CLI/Web adapters only collect, validate, snapshot, and pass values;
- `Makefile` owns release archive contents and archive verification.

No rule embedding, database migration, or historical backfill is required.

## 2. Mandatory runtime configuration

### 2.1 Package manifest

Define one explicit Make variable containing the standalone runtime config contract:

- `config/default.yaml.example`
- `config/nse.yaml`
- `config/service-tags.yaml`
- `config/ports-highrisk.txt`
- `config/ports-top100.txt`
- `config/ports-top1000.txt`

The package recipe copies exactly these files. It must not copy `config/*`, because an operator's untracked `config/default.yaml` may contain local paths or secrets.

After creating the tarball, the package recipe verifies that every manifest entry exists and is non-empty in the staging directory and appears at the expected path inside the archive. A missing source, failed copy, or incomplete archive therefore fails `make package`, which is already part of `make pr-check` and the release workflow.

### 2.2 Fail-closed rule loading

`LoadNSERulesForConfig` and `LoadTagRulesForConfig` keep the existing search order:

1. next to the selected config file;
2. repository/process working directory `config/` fallback.

If neither candidate exists, return an error naming the required file and checked locations instead of an empty ruleset. A present but malformed or empty rules file is also unusable and returns an actionable error.

`PrepareScan` propagates this error before a scan run is created. This is intentional: no successful-looking fingerprint-only run is allowed merely because mandatory detection routing disappeared.

`doctor` calls the same `ForConfig` loaders rather than implementing a looser sidecar-only rule. Missing, malformed, or empty rule files produce failed checks. This keeps startup behavior and operator diagnostics consistent.

## 3. X11 nuclei routing

Add a `service-tags.yaml` rule:

```yaml
- name: x11
  service: ["x11"]
  nuclei_tags: ["x11"]
  target: "hostport"
```

The fingerprint normalizer already lowercases Nmap's X11 service to `x11`. `hostport` produces `IP:port`, matching the installed nuclei JavaScript network template's input contract. No X11 NSE mapping is added.

This does not add global `default-login` or brute-force tags.

## 4. Configuration-loaded routing regression

The regression seam must include real sidecar discovery instead of directly constructing `ScanOptions` with hand-written maps.

A test fixture creates a package-shaped temporary directory:

```text
package/
  config/
    default.yaml
    nse.yaml
    service-tags.yaml
    ports-*.txt (where required by the selected preset)
```

It calls `PrepareScan` with `package/config/default.yaml`, then uses the returned `NSERules` and `TagRules` through the real matcher/execution seam with a recording runner. Assertions cover:

- SSH: NSE scripts `ssh2-enum-algos,ssh-hostkey` and nuclei tag `ssh` execute;
- Tomcat Web fingerprint: NSE remains skipped by policy and nuclei receives Tomcat tags with a URL target;
- X11: no NSE rule, nuclei receives tag `x11` and `IP:port` target;
- successful zero-finding tool output records `completed`, not a failure;
- missing sidecars stop preparation before any runner invocation.

Existing narrower matcher tests remain useful, but this test closes the prior gap where manually supplied rules hid packaging/config-discovery failures.

## 5. Discovery mode repair

### 5.1 Contract

Use two values only:

- `auto`: safe default; run the existing Nmap alive sweep, then scan discovered hosts;
- `assume-up`: skip only the alive sweep and treat the already parsed, excluded, authorized target list as alive.

An empty value normalizes to `auto` for backward compatibility. Any other value is rejected before scan execution.

### 5.2 Minimal data flow

Add `DiscoveryMode` to:

- `app.PrepareScanRequest`;
- `app.ScanOptions`;
- Web `scanForm` JSON snapshot;
- report `ScanData` / `ScanReport`.

CLI `--discovery` and Web `discovery_mode` pass through `PrepareScan`. Web rendering defaults empty historical forms to `auto`; reruns preserve a recorded valid mode and default snapshots from older runs to `auto`.

`scanTargets` branches only around the alive sweep. In `assume-up`, it copies `opts.Targets` into `aliveIPs` and proceeds through the unchanged RustScan, fingerprinting, and vulnerability pipeline. Target parsing and exclusions still happen before this branch, so excluded targets are never reintroduced.

The implementation does not import unrelated hardening-branch work such as process groups, heartbeat persistence, polling, IPv6 scope redesign, or cancellation changes.

### 5.3 Report compatibility

New JSON/HTML reports expose the effective discovery mode. Report rebuilding reads it from the stored config snapshot. Missing or invalid values in historical snapshots resolve to `auto`; historical rows are not modified.

## 6. Error and audit semantics

- Missing rules fail before run creation, so there is no fabricated DetectionCheck history.
- Once a scan starts, existing `running`, `completed`, `skipped`, `failed`, and cancellation transitions remain unchanged.
- A completed engine invocation with zero findings remains `completed`.
- Existing historical `skipped/no_matching_rule` facts remain untouched and continue to describe what the old package actually executed.

## 7. Compatibility and rollback

- Existing source-tree setups continue to work through the current fallback lookup.
- Standalone archives become self-contained for baseline configuration.
- Older Web snapshots remain rerunnable because empty discovery mode defaults to `auto`.
- Rollback is file-local: revert package manifest checks, strict loaders/doctor, X11 rule, and discovery fields/branch. No schema or stored-data rollback is needed.

## 8. Verification strategy

Use the lowest sufficient seams:

1. `internal/config`: missing, empty, malformed, local-sidecar, and fallback loading;
2. `internal/doctor`: absent rules are failed checks;
3. `internal/vuln` / `internal/app`: package-shaped config routes SSH, Tomcat, and X11 and preserves DetectionCheck outcomes;
4. `internal/app`: discovery validation and runner call-order behavior;
5. `internal/web`: form parsing, validation, defaulting, and rerun snapshots;
6. `internal/report`: JSON/model visibility and historical default;
7. `make package VERSION=test`: staged and archived runtime manifest;
8. relevant Go/Node suites, build/Web smoke where proportionate, then independent code review.
