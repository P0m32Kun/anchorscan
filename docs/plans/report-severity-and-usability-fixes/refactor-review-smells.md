# 审查坏味道重构（code-review 后续）

**状态：approved（2026-07-22）**

来源：对 codex「报告风险等级与可用性修复」改动的 code-review，Standards 轴的 3 条 judgement call。行为不变，纯重构。

## 范围

1. **Feature Envy** — `internal/web/report_filters.go` 的 `catalogEntryMatchesKeyword` 深入 `knowledgebase.Entry` 内部字段。
   - 在 `internal/knowledgebase/catalog.go` 新增方法 `func (e Entry) MatchesKeyword(keyword string) bool`：对 ID、Name、Aliases、Match.NucleiIDs/NSEIDs/CVEs/Names 做大小写不敏感的包含匹配（keyword 先 TrimSpace+ToLower）。
   - `catalogEntryMatchesKeyword` 简化为：`match.Status == knowledgebase.MatchMatched && match.Entry.MatchesKeyword(keyword)`。
2. **Duplicated Code** — `internal/report/vulnerability_delivery.go` 的 `deliveryAssetCopyText` 与 `internal/web/report_view.go` 的 `vulnerabilityAssetCopyText` 重复「去重→排序→按 \n 拼接」。
   - 在 `internal/report/vulnerability_delivery.go` 导出 `func SortedUniqueJoin(lines []string) string`：跳过空串、去重、sort.Strings、strings.Join("\n")。
   - 两个调用点改为收集字符串后调用它（web 包已 import report）。
   - 注意清理 `report_view.go` 可能不再使用的 `sort` import。
3. **Primitive Obsession** — `internal/web/report_handler.go` 的 `summarizeRisk` 对裸字符串 switch。
   - 改为 `switch knowledgebase.Severity(strings.ToLower(finding.Severity))`，case 用 `knowledgebase.SeverityCritical/High/Medium/Low` 常量（文件已 import knowledgebase）。

## 非范围

- 不改任何行为、模板、测试断言语义；不触碰 `internal/tools/httpx*.go`。
- 不提交（提交由主会话验收后执行）。

## 测试接缝

- 新方法 `Entry.MatchesKeyword`：在 `internal/knowledgebase/catalog_test.go` 先写一个失败的小测试（命中 Name / 命中 CVE / 未命中 / 大小写），再实现（red-green）。
- 其余两处由既有测试覆盖：`go test ./internal/...` 必须全绿，外加 `internal/web/static` 的 `node --test app.test.mjs`。

## 验收标准

- `go build ./...`、`go vet ./...`、`go test ./internal/...` 全部通过。
- diff 中无行为变化（仅移动/复用逻辑）。
