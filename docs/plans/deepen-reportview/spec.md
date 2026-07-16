# 深化报告视图模块（ReportView）

## 背景与动机

`internal/web/report_handler.go` 的 `reportDetail` 是近 4 个月 git 热点之一。其主 HTML 视图分支把**视图塑造**（view 模式解析、漏洞交付构建、三路分页、commandTools、导出/资产链接 URL）与 **HTTP 管线**（路径分发、Content-Type、ServeFile、query 解析）焊死在一个 handler 里，导致 `report_handler_test.go` 高达 1272 行、只能端到端（真实 SQLite + httptest）测试。

## 目标

把主 HTML 视图的塑造逻辑抽成一个纯函数 `buildReportViewModel`，输入 `url.Values`（纯 map，非 HTTP 运行时耦合），输出具名 `reportViewModel`。行为零变化，且无需 HTTP/SQLite 即可单测。

## 设计决策（自主执行，用户授权「按推荐持续推进」）

1. **范围**：仅抽主 HTML 视图分支的塑造。`format`（json/html）、`export`、`asset`（txt/csv）、`commands` 分支是合理的 HTTP 适配器（管 Content-Type/ServeFile/Content-Disposition），不动。
2. **`url.Values` 解耦（A 而非 B）**：复探发现过滤（`filterFingerprints`/`filterFindings`/`reportFiltersFromValues`）、分页（`paginateFingerprints` 等）、`report.Build` **早已抽出**。剩余内联的只是 ~30 行编排。分页函数需 `url.Values` 来生成「保留当前 filter 的翻页链接」——而 `url.Values` 本身只是 `map[string][]string`，非 HTTP 耦合。故 `buildReportViewModel` 直接吃 `url.Values` 即可单测（测试里构造 map 即可），无需再抽 `ReportQuery` 类型重做分页链接生成（B 方案，风险/收益不划算）。**这是相对最初「倾向 B」的更新，记此偏差。**
3. **`commandTools` 留作 server 方法**：它调用 `s.buildCommand`/`s.commandUnavailableReason`（依赖 server 持有的工具配置），非纯。故由 handler 算好 `s.commandTools(filteredFindings)` 作为 `reportViewInput.CommandTools` 传入；其余塑造（分页/视图模式/漏洞交付/URL）进纯函数。
4. **模板渲染**：`reportViewModel` 结构体字段名与原 `map[string]any` 的 key 一一对应；Go `html/template` 对 struct 字段与 map key 的访问语法相同，模板无需改动（已验证模板仅在嵌套字段 `.HostPage.SizeURLs` 上用 `index`，顶层模型可用 struct）。
5. **测试**：现有 `report_handler_test.go`（1272 行）**原样不动**作回归网（行为零变化）；新增 `TestBuildReportViewModel*`（构造 `url.Values` + 零值 `*knowledgebase.Catalog`，断言分页/视图默认/URL/catalog 状态），无需 HTTP/SQLite。

## 领域词汇（见 CONTEXT.md）

`Report`、`Fingerprint`、`Finding`、`Progress` 已在 CONTEXT.md。本候选新增视图层概念（`reportViewModel` 为内部实现，不必入 CONTEXT.md）。

## Review fixed point

`93e945d`（main）
