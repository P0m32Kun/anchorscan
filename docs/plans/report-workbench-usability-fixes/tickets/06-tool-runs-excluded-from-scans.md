# 06 — 单工具运行不计入扫描任务（issue 5）

**What to build:** 扫描任务列表与首页计数只含 `kind='scan'`；工具运行（`kind='tool'`）不再出现在首页"最近扫描任务"、`/runs`、项目"扫描历史"；工具页从工作台跳转时不默认绑定项目。

**Blocked by:** None

**Status:** done

## 完成条件

- `store/runs.go`：`ListScanRuns`、`ListProjectScanRuns` 增加 `kind = 'scan'` 过滤（或提供按 kind 区分的查询）。
- 首页"历史任务"计数与"最近扫描任务"列表同步只统计 scan（`home.html` 数据源）。
- `tool_page.html` / `tools.go`：从工作台带入 `project_id` 时不自动选中绑定（下拉默认"不绑定项目"），或显式提示"工具调用不计入扫描任务"。
- 运行详情页 `isToolRun` 判断不变；工具运行详情仍可正常查看。
- 测试接缝：`runs_test.go` 对两个 List 函数的 kind 过滤 red-green。
