# Implementation Plan

1. Record the implementation fixed point and update the authoritative Dameng plan ticket to `ready-for-agent` only after this planning summary receives explicit approval.
2. Use TDD at the tools seam: add a failing panic-checker case and deadline case for `RunDamengDefaultPassword`, then implement localized panic recovery and the panic/deadline error contract while preserving auth and ordinary-network verdict behavior.
3. Use TDD at the app seam: add a failing scan test for the resulting execution error; verify no process panic, no Dameng Finding, `failed/command_failed`, and `completed_with_errors` persistence.
4. Change only the Dameng default in `internal/config/init.go` and `config/default.yaml.example` from `0` to `15s`; extend default/example consistency or parsing tests so explicit `0` remains supported.
5. Record Docker validation as deferred: the investigated third-party images did not yield a reliable configurable `SYSDBA/SYSDBA` fixture. Do not continue image investigation or add a container dependency in this task.
6. Run focused packages (`go test ./internal/tools ./internal/app ./internal/config`), then `make test` and `go vet ./...`. Run `make pr-check` if its toolchain prerequisites are available; otherwise record the exact blocker.
7. Run independent standards/spec review against the fixed point and `docs/plans/archive/dameng-default-password/tickets/02-harden-timeout-and-panic-isolation.md`; resolve blocker/high findings and repeat focused verification.
8. Update the ticket status to `done` only after verification and review; update the relevant project spec if implementation reveals a durable contract not captured here.

## Risky Files and Rollback

- `internal/tools/dameng.go`: recovery must not hide application panics or convert authentication rejection to an engine failure.
- `internal/app/scan_target.go`: prefer existing error persistence behavior rather than new orchestration.
- `internal/config/init.go` and `config/default.yaml.example`: only change the new-install default; do not rewrite existing configs.

Rollback consists of reverting these source/config changes. No persistent data is changed.
