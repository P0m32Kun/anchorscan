# 02 — 深化 Run Report 阅读查询 module

**What to build:** 用 `internal/web` 中的单一纯函数 interface 收拢 Run Report 的 query 解析、事实筛选、报告构建、视图选择、分页与 adapter 投影，使控制台、导出和命令入口共享同一查询语义。

**Blocked by:** 01 — 深化 Project Report 交付组装 module。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`update-spec`、`ponytail`。

- [x] 在 `buildRunReportReading` interface seam 写失败测试，输入原始 Run、Fingerprint、Finding、DetectionCheck、Catalog 与 query。
- [x] 覆盖 ports/hosts/vulnerabilities 视图、筛选、分页、风险、Detection Coverage、导出事实和命令筛选。
- [x] 建立单一 `runReportReadingInput` 与单一阅读结果；不新增 package、Store interface 或对象层级。
- [x] 把 query 解析、`filter*`、`paginate*`、Run Report Build 与 view shaping 收入 module implementation。
- [x] 让控制台、独立 HTML、`assets.txt`、单条命令和批量 Nuclei/Nmap/MSF 命令复用同一查询结果。
- [x] 保持命令生成位于现有 `internal/report` implementation，不把执行命令职责放入阅读 module。
- [x] 删除 handler 中重复的编排与只证明调用顺序的测试；分页窗口算法可保留一个聚焦内部测试。
- [x] 严格保持 query 名称、默认值、无效值处理、排序、分页链接、导出链接、命令范围、Vue 状态和响应格式。
- [x] 运行聚焦测试、全量 Go 测试与静态检查。
- [x] 以 ticket 01 完成后的 fixed point 和 `spec.md` 运行 Standards/Spec 双轴 code review，修正发现后将 ticket 标为 done。
