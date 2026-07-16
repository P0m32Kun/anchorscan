# Ticket 01 — 抽出 ReportView 视图塑造模块

- 状态：`ready-for-agent` / frontier
- Blocked by：无
- 所属 spec：`docs/plans/deepen-reportview/spec.md`
- Review fixed point：`93e945d`

## 目标

把 `reportDetail` 主 HTML 视图分支的塑造逻辑抽成纯函数 `buildReportViewModel`，行为零变化，且无需 HTTP/SQLite 即可单测。

## 变更清单

1. **新增 `internal/web/report_view.go`**：`reportViewModel` 结构体（21 字段，对应原模板 map key）+ `reportViewInput` 输入束 + `buildReportViewModel(in) reportViewModel`（解析 view 模式 → 漏洞交付 → 三路分页 → copyBase → 导出/资产 URL）。
2. **`internal/web/report_handler.go`**：主 HTML 视图分支的 ~30 行内联组装（含 `render(map[string]any{...})`）替换为 `render(buildReportViewModel(reportViewInput{...}))`。`commandTools` 留作 server 方法，由 handler 算好传入。
3. **新增 `internal/web/report_view_test.go`**：`TestBuildReportViewModel`（12 指纹 + size 10 + page 2 → 2 项/HasPrev；URL 含 runID；零值 catalog 状态）+ `TestBuildReportViewModelDefaultsInvalidView`（非法 view 回退 ports）。无 HTTP/SQLite。

## 验收

- `go build/vet/test ./...` 全绿；`gofmt -l` 空。
- 现有 `report_handler_test.go`（1272 行）零修改通过（行为零变化）。
- 新增 `TestBuildReportViewModel*` 不依赖 HTTP/SQLite 即可验证视图塑造。

## Review 结果

（code-review 后填写）
