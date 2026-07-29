# Technical Design

## Boundary and Data Flow

`scanTarget` owns DetectionCheck persistence and Run partial-failure behavior. `tools.RunDamengDefaultPassword` owns verdict classification. The production `damengDriverChecker.Check` is the sole boundary that calls the untrusted third-party database driver:

`scanTarget -> RunDamengDefaultPassword -> damengDriverChecker.Check -> sql.Open/PingContext -> DM driver`

Add a named-return helper around exactly `checker.Check`, called by `tools.RunDamengDefaultPassword`. This is the tool integration boundary: production passes `damengDriverChecker`, whose implementation calls `sql.Open` and `PingContext`; tests may inject a checker that panics. Convert a recovered value to an error prefixed `dameng driver panic:`. Do not add a broad recovery to `scanTarget` or worker orchestration.

## Error Contract

`RunDamengDefaultPassword` must preserve its existing successful and ordinary error verdict mapping. It treats a recovered driver-panic error and `context.DeadlineExceeded` as execution errors: return the diagnostic output plus a non-nil error. This lets the existing `scanTarget` error path write `failed/command_failed`, suppress a Finding, mark `HadErrors`, and allow the overall Run to become `completed_with_errors`.

An authentication rejection remains `DamengSafe`; a non-auth connection/protocol error remains `DamengUnknown` with nil error. This prevents network instability from being asserted as either vulnerability or execution failure beyond the newly decided panic/timeout cases.

## Timeout Compatibility

Set `Dameng: "15s"` in `config.Default()` and `config/default.yaml.example`. Do not alter duration parsing, `toolContext`, or other tool defaults. Existing user configuration is read unchanged, so an explicit `dameng: "0"` remains unlimited and any positive duration overrides 15 seconds.

## Test Strategy

- Tools unit seam: use an injectable checker that panics; assert recovery and the explicit error contract. Add deadline error coverage without real network I/O.
- App scan seam: use an injectable checker returning a panic-derived or deadline error; assert `failed/command_failed`, no Finding, and a successful Run invocation whose persisted status is `completed_with_errors`.
- Config unit seam: assert default and example values agree on `15s`, while explicit zero parsing remains valid.

No real DM instance is needed for the mandatory regression suite: the risk is error containment and persistence semantics, which the injected checker reproduces deterministically. Docker validation is deferred because both investigated third-party images failed to provide a reliable, configurable `SYSDBA/SYSDBA` fixture. Keep the recorded evidence, but do not continue Docker investigation or add image dependencies in this task.

## Rollback

Revert the localized driver boundary and default timeout values. No schema, migration, historical Run, release artifact, or authorization change is involved.
