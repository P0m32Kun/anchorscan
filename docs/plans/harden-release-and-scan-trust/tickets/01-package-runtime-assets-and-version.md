# 01 — 完整发布运行时资源并统一版本

**What to build:** 让发布归档携带默认扫描和 DOCX 导出所需的全部 sidecar，并让归档名、CLI、Web 与 Git tag 使用同一构建版本。

**Blocked by:** None — can start immediately.

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 发布归档必须包含 spec 列出的 config 与 DOCX sidecar，不携带本机 `default.yaml`、数据库或报告。
- `internal/version.Version` 的源码默认值是明确开发版本，release/build 通过 ldflags 注入。
- 解包后的二进制版本与归档版本一致。
- 测试必须读取真实归档或 staging 目录，不通过匹配 Makefile 文本冒充行为测试。

## 测试 seam

- Packaging integration：临时目录执行 package staging/解包并检查资源与 loader。
- CLI command：执行构建二进制的 `--version`。

## 验收

- [x] 先增加会因缺失 `nse.yaml`、`service-tags.yaml` 和端口预设而失败的 package test。
- [x] 最小修改 Makefile，使 package test 通过。
- [x] 先增加会因硬编码 `1.9.2` 而失败的版本注入测试。
- [x] `version.Version` 可注入，开发构建值明确，CLI 与 Web 使用同一值。
- [x] 发布 workflow 传入 tag 版本；归档命名和二进制输出一致。
- [x] 聚焦测试、`make test`、`go vet ./...`、`make pr-check` 通过。

## 验收记录

- packaging integration 先因缺少四个运行时资源失败；补齐显式复制后，真实归档可由生产 loader 加载 NSE、service-tag 和两个端口预设。
- 版本 integration 先观察到 ldflags 无法覆盖硬编码 `1.9.2`；改为默认 `dev` 的变量后，注入构建、开发构建、解包二进制、CLI 和 Web 页脚使用同一值。
- 归档测试同时拒绝本机 `config/default.yaml`、`data/` 和 `reports/`。
- `make pr-check`、`go vet ./...` 与修改 Go 文件的 LSP diagnostics 均通过。
- 双轴 review 未发现 blocker；异平台归档的原生执行 smoke 由 ticket 09 的 release gate 处理，本 ticket 已验证本机真实归档和 release tag 接线。

## 非目标

- 不复制整个 `config/`。
- 不增加安装器、自动更新器、签名或 SBOM。
