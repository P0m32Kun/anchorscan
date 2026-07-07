# Task 1 Report: Parse Scan Profiles And Extra Args

Status: complete

Implemented:
- `internal/config/config.go`: added `ToolArgs`, `Profile`, `Config.Profiles`, `Scan.Profile`, and `Load` defaults.
- `internal/config/args.go`: added `SplitArgs`.
- `internal/config/profile.go`: added `Overrides`, `EffectiveScan`, and `ResolveScan`.
- `internal/config/config_test.go`: added profile parsing and default profile tests.
- `internal/config/args_test.go`: added arg splitter tests.
- `internal/config/profile_test.go`: added scan resolution tests.
- `config/default.yaml`: added `scan.profile` and default `profiles`.

Verification:
- `rtk proxy go test ./internal/config -count=1`
- `rtk proxy go test ./... -count=1`

Concerns:
- None.

Fix update:
- Added a regression test for V1-style configs with no `profiles:` section in `internal/config/profile_test.go`.
- `ResolveScan` now injects built-in `slow/normal/fast` defaults only when `cfg.Profiles` is empty, so legacy configs resolve while explicitly unknown profile names still fail.

Additional verification:
- `rtk proxy go test ./internal/config -run TestResolveScanDefaultsV1ConfigWithoutProfilesSection -count=1`
