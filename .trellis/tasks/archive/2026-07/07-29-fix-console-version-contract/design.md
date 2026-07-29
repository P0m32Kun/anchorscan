# Console 版本显示契约

## 边界

CLI 与 Web Console 共享单一 `internal/version.Version` 变量。本地开发构建直接显示 `dev`；release/package 构建通过 Go linker 注入去掉 `v` 前缀的 Tag 版本。

## 契约

- `internal/version/version.go` 中 `Version = "dev"` 是唯一的开发回退值。
- `Makefile` 的 `DISPLAY_VERSION := $(patsubst v%,%,$(VERSION))` 负责在构建时去掉 leading `v`。
- `go build -ldflags="-X github.com/P0m32Kun/anchorscan/internal/version.Version=$(DISPLAY_VERSION)"` 同时注入 CLI 与 Web 可见值。
- Web 模板通过 `internal/web/templates.go` 的 `version` 函数读取 `version.Version`。
- 不运行时查询 Git 分支或远端 Tag；构建产物版本完全由构建输入决定。

## 验证

- `cmd/anchorscan/main_test.go:TestVersionCommandPrintsVersion`：CLI `version` / `--version` / `-v` 输出包含 `anchorscan version <version.Version>`。
- `scripts/package_smoke_test.go:TestBuildVersionCanBeInjected`：linker 注入 `v9.8.7-linker-test` 后二进制 `--version` 显示 `anchorscan version v9.8.7-linker-test`；无注入时显示 `anchorscan version dev`。
- `scripts/package_smoke_test.go:TestPackageArchiveIncludesRuntimeResources`：打包产物二进制版本与 `ANCHORSCAN_PACKAGE_VERSION` 一致。
- `internal/web/projects_test.go:TestHomePageRenders` 与 `internal/web/report_handler_test.go:TestReportPageRendersCurrentVersion`：HTML footer 包含 `AnchorScan Console <version.Version>`。
- `scripts/web-smoke.mjs`：在 `ANCHORSCAN_EXPECTED_VERSION=<DISPLAY_VERSION>` 下验证浏览器页面显示 `AnchorScan Console <DISPLAY_VERSION>`。
- `make release-check`：验证本地 release 构建的 `anchorscan version` 输出严格等于 `anchorscan version <DISPLAY_VERSION>`。

## 风险与回滚

仅文档与验证，无代码行为变更。若需调整契约，修改点为 `internal/version/version.go`、Makefile 的 `VERSION`/`DISPLAY_VERSION` 逻辑，以及 `internal/web/templates.go` 的模板函数。
