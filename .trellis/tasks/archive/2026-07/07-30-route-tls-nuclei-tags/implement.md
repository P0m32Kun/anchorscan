# 实施计划

1. 在 `internal/fingerprint` 新增或提炼单一 TLS 证据谓词，并让 `Classify` 使用它，保持 `tunnel: ssl`、`https`、`ssl/http` 三类证据的现有 URL 分类语义。
2. 在 `internal/vuln.MatchNucleiTags` 中仅对已有匹配规则且 TLS 谓词为真时追加 `ssl`；保留无匹配返回零值、target、默认排除和规则级排除。
3. 扩展 `internal/fingerprint` 与 `internal/vuln` 单元测试：三种 TLS 正例、普通 HTTP、unknown/tcpwrapped、无规则 TLS、tags/target/exclude tags 不变量。
4. 扩展 `internal/app` fake-runner seam 测试，断言 TLS 规则传给 Nuclei 的 `-tags` 含 `ssl`，普通 HTTP 与无规则 TLS 不因此触发新增 Nuclei 调用。
5. 更新 `scan-runtime-contracts.md`：明确这是既有规则命中后的 tag 追加，不是 TLS 探测、模板锁定或通用服务 fallback。
6. 运行 `go test ./internal/fingerprint ./internal/vuln ./internal/app ./internal/config`、`go vet ./...`、`git diff --check`、LSP diagnostics；进行 Standards 与 Spec/AC 独立复审。

## Rollback

移除 `ssl` 追加分支及相关测试/契约段落即可恢复原 tags；无需数据迁移。
