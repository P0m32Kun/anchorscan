# 01 — 报告封面标题与 3.1 会话标签修正（issues 1, 2）

**What to build:** 移除项目表单的"报告标题"字段与详情页"报告标题"卡片，封面固定为 `{被测单位}安全渗透测试分析报告`；项目报告 3.1 会话行删除"接入记录"、`接入点`→`测试设备接入点`、`测试机 IP`→`测试设备 IP`，HTML 与 DOCX 一致；`scan_project.html` 表单标签与 placeholder 同步。

**Blocked by:** None

**Status:** done

## 完成条件

- `project_form.html` 无"报告标题"字段；`projects.go` 停止解析 `report_title`（按 spec 技术说明决定是否删列，落定后回写）。
- `project_detail.html` 删除"报告标题"卡片；封面/交付标题经 `reportTitle()` 回退为 `{被测单位}安全渗透测试分析报告`。
- `scan_project.html`："接入点/交换机"→"测试设备接入点"（placeholder "XX屏柜/xxx交换机"），"测试机 IP"→"测试设备 IP"。
- `internal/report/templates/project_report.html` 会话行去掉"接入记录"段，两个标签改名。
- DOCX：`tools/docx-render/prototype.py` 三行标签改后重新生成 `templates/project-report.docx`（或直改 `word/document.xml`），`check_structure.py` 断言同步。
- 测试接缝：`reportTitle()` 回退为 red-green 单测；DOCX 结构断言更新。
