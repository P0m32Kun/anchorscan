# 09 — 现场实测回归闭环

**What to build:** 修复 2026-07-23 现场复测发现的 rpcbind 规则、负向服务聚合与证据入口、工具输出/布局/历史、工作台残留入口、DOCX 测试范围换行和验证证据导出回归。

**Blocked by:** None

**Status:** done

## Review fixed point

`ba0b2e45ea9fd856741c7dacc453717fbe607354`

## 完成条件

- 默认 NSE 配置为 `rpcbind` 提供 RPC 脚本，且不放宽 `rdpscan` 的 RDP 服务限定。
- 同分区、同 service/product 的不同 IP/端口只生成一个负向组；Nmap 建议命令覆盖该组的所有唯一 IP 和端口。
- 负向组没有漏洞知识库条目，命令直接使用 NSE/tag 配置规则，不误用只接受正向漏洞候选的 `report.BuildCandidateCommands`。
- 负向卡片直接打开自己的验证弹窗，弹窗中清楚展示选择、拖放、Ctrl/Cmd+V 和多图能力。
- 用户给出的 `[[34mINF[0m]` 样例可读；工具输出在页面下半区全宽展示，完成后提供本次结果链接，并有只列工具运行的历史页。颜色保留要求由 ticket 10 取代本 ticket 的纯文本处理。
- 工作台 HTML 不含“正式报告”。
- DOCX context 把逗号/空白分隔的多个测试目标输出为逐行文本；既有验证项和证据映射端到端测试继续通过。

## 测试接缝

- `internal/config/config_test.go`：默认 rpcbind NSE 规则。
- `internal/web/workbench_test.go`：跨 IP/端口服务聚合和命令覆盖。
- `internal/app/tool_run_test.go`：双左括号 ANSI 残片。
- `internal/web/runs_test.go`：`/runs?kind=tool` 只列工具运行。
- `internal/report/docx_context_test.go`：测试目标逐行格式。
- 既有 `internal/web/project_report_test.go`：验证信息和证据导出端到端覆盖。
