# 深化 Project Report 与 Run Report modules

**Status:** approved

## Problem Statement

当前 Report 热区有两个缺少 locality 的 seam：

1. HTML 与 DOCX 两个 adapter 共用 `internal/web.buildProjectDeliverable`，但该 helper 的 interface 接受 `http.ResponseWriter` 和 `*http.Request`，在组装 `ProjectDeliverable` 的同时写 HTTP 错误。Project、Network Zone、Run、Verification 与 Evidence 的读取和验证因此被 Web seam 所有。
2. Run Report 的同一查询含义分散在 handler、筛选、分页和 view shaping 中。控制台、HTML 导出、资产文本与命令端点分别参与 query 编排，调用者需要理解 implementation 顺序，现有 module 的 interface 仍然 shallow。

本计划在同一次更新中依次深化这两个 module，不改变外部行为。

## Approved Direction A — Project Report 交付组装

- 由 `internal/app` 拥有 Project Report 交付组装 module。
- module 只负责读取、组装并验证规范化的 `report.ProjectDeliverable`。
- HTML 与 DOCX 保持为两个 adapter，负责 HTTP 状态、响应头和最终渲染。
- module 直接使用 `*store.Store` 和本地 Evidence 文件系统；不新增 Store interface、repository、fake、Builder 或 factory。
- module 使用单函数 interface：

  ```go
  app.BuildProjectDeliverable(
      scanStore *store.Store,
      projectID string,
      now time.Time,
  ) (report.ProjectDeliverable, error)
  ```

- interface 只返回 `ProjectDeliverable`；下载文件名从 `deliverable.Project.Name` 派生，不让 `store.Project` 穿过 seam。
- Evidence 在组装阶段 eager assembly：验证存在性、解析文件路径、读取 Data URI，并保留 DOCX 所需的文件路径。
- 保留 ADR-0005：HTML 与 DOCX 必须消费同一 `ProjectDeliverable`，不得建立第二个聚合模型。

## Approved Direction B — Run Report 阅读查询

- 由 `internal/web` 持有 Run Report 阅读 module；不新增 package。
- module 是纯函数，不读取 Store、不写 HTTP，也不创建临时导出文件。
- module 负责解释完整 URL query，并从原始 Run Report 事实生成规范化阅读结果：
  - Fingerprint、Finding 与 DetectionCheck 筛选；
  - `ports` / `hosts` / `vulnerabilities` 视图选择；
  - 分页、分页大小与导航链接；
  - 风险、Detection Coverage 与控制台 view model；
  - HTML、资产文本和命令 adapter 所需的筛选事实。
- 控制台、独立 HTML、`assets.txt`、单条检测命令及批量 Nuclei/Nmap/MSF 命令共享同一查询 seam；命令生成仍由现有 `internal/report` implementation 负责。
- module 只暴露一个输入结构和一个结果结构：

  ```go
  reading := buildRunReportReading(runReportReadingInput{...})
  ```

- `filter*`、`paginate*`、query 解析和 view shaping 都是 module implementation，不再由 handler 编排。
- module depth 不依赖物理文件合并；Go 文件可按可读性保留，但测试和调用者只把单一阅读入口视为 interface。

## Error Contract

Project Report module 不知道 HTTP，但保留三类结果：

1. Project 不存在：not found；adapter 映射为 404。
2. 元数据、Evidence、Network Zone 或交付验证不完整：invalid Project Report；adapter 映射为 400。
3. Store 或 Evidence 文件读取失败：内部错误；adapter 映射为 500。

只增加满足上述映射所需的最小错误分类，不建立错误层级。当前用户可见中文错误是本次重构的兼容契约，完整清单见 [`error-inventory.md`](error-inventory.md)。后续文案改造单独执行，不与 seam 移动混合。

Run Report 阅读 module 保持当前 query 容错与默认值，不新增用户错误。

## Test Seams

### Project Report

- 以 `app.BuildProjectDeliverable` 的 interface 为主要测试面。
- 使用临时 SQLite 与临时 Evidence 文件；不新增 fake Store。
- module 聚焦覆盖：成功、Project 不存在、invalid Project Report、Evidence 不可读。
- 保留少量 HTML/DOCX handler 回归，验证错误分类到 HTTP 状态的映射及 adapter 输出。
- `internal/report` 现有纯构建和验证测试保持不变。

### Run Report

- 以 `buildRunReportReading` 的 interface 为主要测试面。
- 输入原始 Run Report 事实与 query，覆盖筛选、视图、分页、导出事实与命令筛选。
- 删除只证明 handler 调用顺序的测试。
- 分页窗口算法可保留一个聚焦内部测试。
- handler 测试只保留路由、响应格式与 adapter 行为。

## Acceptance Criteria

### Project Report

- `internal/web.buildProjectDeliverable` 被删除，组装 implementation 位于 `internal/app`。
- HTML 与 DOCX adapter 都通过同一 app module 获取 `ProjectDeliverable`。
- Web adapter 不再把 `http.ResponseWriter` 或 `*http.Request` 传入组装 implementation。
- 路由、文件名、响应头、HTTP 状态、中文错误、Evidence 顺序与内容、Network Zone 推断和 docxtpl 调用行为保持不变。
- `ProjectDeliverable` 结构不变，HTML 与 DOCX 继续共享同一实例模型。

### Run Report

- `reportDetail`、资产文本和命令入口不再自行组合 query 解析、筛选、Build 与 view shaping 顺序。
- 控制台、HTML 导出、`assets.txt` 与命令入口共享 `buildRunReportReading` 的查询结果。
- query 参数、默认值、无效值处理、排序、风险统计、Detection Coverage、导出链接、分页链接与命令范围保持不变。
- Vue 客户端状态、URL 行为、路由和响应格式保持不变。
- 主要测试穿过新的阅读 interface，而不是分别固定浅 helper 的调用细节。

### Quality Gates

- 每个 ticket 分别执行 TDD、聚焦测试、全量 Go 测试和静态检查。
- 每个 ticket 分别完成 Standards/Spec 双轴 code review 并关闭发现。

## Out of Scope

- 修改中文错误文案、引入稳定错误码或国际化。
- 修改 Project Report 领域规则或 `ProjectDeliverable` 结构。
- 修改 Run Report query 语义、筛选规则、分页体验或 Vue 交互。
- 延迟加载 Evidence 或按输出格式构建不同数据。
- 新增 Store interface、内存 adapter、远程 Evidence adapter 或新 package。
- 改变 DOCX sidecar、模板或发布依赖。

## Domain and ADR Notes

`Project Report`、Run Report、Network Zone、Verification、Evidence、Fingerprint、Finding 与 Detection Coverage 均已定义于 `CONTEXT.md`，本次不引入新领域概念，无需修改词汇表。该重构遵守 ADR-0004 与 ADR-0005；它是可逆的 seam 移动，不新增 ADR。

## Delivery Rule

按 `tickets/` 的阻塞关系一次执行一个 ready frontier ticket：

1. ticket 01：Project Report 交付组装 module；
2. ticket 02：Run Report 阅读查询 module；
3. ticket 03：中文错误改造，阻塞于 ticket 01，且不属于本次架构行为变更。

两个 module 在同一次更新中连续完成，但分别验证与审查。计划假设失效时先修订本 spec 和受影响 ticket。
