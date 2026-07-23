# 10 — 第二轮现场实测回归

**What to build:** 修复 DOCX 全文缩进、工具 ANSI 彩色输出、正常 skipped 检查误入未完成队列、Samba 建议命令缺失、负向提交后队列丢失，以及负向证明标题文案。

**Blocked by:** 09-field-regression-closure（done）

**Status:** in-progress

## Review fixed point

`ba0b2e45ea9fd856741c7dacc453717fbe607354`

## 完成条件

- DOCX 测试范围列表和修改建议的每个段落使用统一缩进，渲染后逐页检查。
- 工具页安全呈现正常 ANSI 颜色；纯文本事件仍可读取，不显示控制码方框。
- `skipped/no_matching_rule` 仅表示单引擎不适用；任一引擎完成且其余引擎无匹配规则时生成负向候选，不显示为检查未完成；真正异常状态仍显示。
- Samba 负向组同时显示复制 Nmap、复制 Nuclei。
- 提交负向验证并刷新后仍停留在待负向验证队列。
- 前端和 DOCX 的负向标题均为“{service/product}相关漏洞不存在证明，端口（{ports}）”。

## 测试接缝

- `internal/report/project_test.go`：DetectionCheck 分类。
- `internal/config/config_test.go`、`internal/web/workbench_test.go`：Samba 默认规则与命令。
- `internal/app/tool_run_test.go`、`internal/web/static/app.test.mjs`：ANSI 保留与安全颜色渲染。
- `internal/web/project_report_test.go`、`internal/report/docx_context_test.go`：负向标题和 DOCX context。
- `tools/docx-render/test_render_docx.py`：渲染后 OOXML 段落缩进。
- 浏览器本地回归：负向队列提交后状态保持与工具输出视觉效果。
