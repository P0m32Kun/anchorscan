# 03 — 扫描报告"检测执行事实"默认折叠（issue 10）

**What to build:** 把扫描报告的"检测执行事实"区块包进 `<details>`，默认收起，`<summary>` 标注"检测执行事实（NSE/nuclei）"。console 模板与导出模板都要改。

**Blocked by:** None

**Status:** done

## 完成条件

- `internal/web/templates/report.html` 的"检测执行事实" `<section>` 改为 `<details>`（无 `open`），标题进 `<summary>`。
- `internal/report/templates/report.html` 同样处理。
- 展开后表格内容不回归；`DetectionCoverage` 汇总卡保持不变。
- 结构改动，以渲染检查验收，无单测。
