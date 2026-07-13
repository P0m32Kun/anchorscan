---
comet_change: decompose-delivery-adapters
role: technical-design
canonical_spec: openspec
archived-with: 2026-07-13-decompose-delivery-adapters
status: final
---

# 交付适配器职责拆分技术设计

## 前置条件与范围

本设计细化 `openspec/changes/decompose-delivery-adapters/`。实施前必须完成 `decompose-scan-runtime`；CLI 和 Web 只消费已经稳定的共享扫描准备与运行时入口。

所有代码继续位于 `cmd/anchorscan` 和 `internal/web` 的现有 package。不得改变 CLI、HTTP、模板数据、DOM、数据库、报告或扫描行为，不新增依赖、框架、接口层或跨包 API。

## CLI 文件边界

- `main.go`：`main`、`Execute`、根 `run`、版本和根帮助。
- `scan_command.go`：`runScan`、扫描帮助以及扫描专用日志输出。
- `tool_command.go`：`runTool`、工具参数适配、工具帮助和 CSV 参数辅助。
- `report_command.go`：`runReport`、`runImportNmap` 及对应帮助。
- `admin_command.go`：`runDoctor`、`runWeb`、`runCancel`、`runTools`、路径检查和对应帮助。

这些是文件分组，不是新类型体系。函数保持 package-private，`cliDeps` 继续由根入口持有。若两个一行 helper 只有一个调用方，直接留在调用方文件。

测试首先继续使用 `main_test.go`；只有当生产函数完成移动后，才按 `scan_command_test.go`、`tool_command_test.go`、`report_command_test.go`、`admin_command_test.go` 移动现有测试。共享 fake 不复制。

## Web 文件边界

- `server.go`：`ServerOptions`、`server`、`NewServer`、完整路由注册、`ServeHTTP`、`Close`、通用 render/newID。
- `projects.go`：home、projects、projectNew、projectDetail 及项目删除相关逻辑。
- `scans.go`：扫描表单、`scanCreate`、项目默认值与目标输入辅助。
- `tools.go`：toolNew、toolPage、toolCreate、manual tool/preset 和工具参数辅助。
- `imports.go`：Nmap 导入表单与执行。
- `runs.go`：runDetail、runAPI、runs 及运行删除相关逻辑。
- `config.go`：configPage、configPorts、端口规范化。
- `report_handler.go`：reportDetail，只负责请求解析、调用报告纯函数和返回响应。

`server.go` 的路由注册代码只移动依赖符号，不重排路由。测试按资源移动，但跨资源的路由契约测试保留在 `server_test.go`。

## 报告职责边界

现有 `reports.go` 按最少四个真实修改原因整理：

- `report_filters.go`：过滤器类型、解析、fingerprint/finding 匹配。
- `report_pagination.go`：页码、页大小、泛型分页和 URL 构造。
- `report_exports.go`：TXT/CSV 导出。
- `report_views.go`：页面数据、run meta、host 聚合。

报告 Handler 仍使用现有 Store 查询和 `internal/report` 输出能力。不得引入查询 DSL、Presenter 或 ViewModel 接口。

## 迁移与验证策略

每个职责遵循同一循环：锁定现有测试或补一个特征测试、移动一组符号、格式化、运行最窄测试、再进入下一组。先 CLI，再 Web 资源，最后报告纯函数和测试文件。

最终验证覆盖 CLI 帮助/错误/输出、HTTP 方法/状态码/重定向、模板数据、报告筛选/分页/导出、Node 静态资源测试和打包。若某个建议文件最终只有极少且共同变化的代码，可与相邻职责合并，禁止为了清单制造空壳文件。

## 验收标准

- `main.go` 和 `server.go` 分别只保留根分派与服务器装配职责。
- CLI 参数、帮助、stdout/stderr 和错误优先级不变。
- 路由集合与注册顺序、HTTP 状态、重定向、表单字段和响应正文不变。
- 报告默认筛选、排序、分页、导出和模板数据不变。
- 不增加依赖、公共 API、单实现接口或通用交付框架。


