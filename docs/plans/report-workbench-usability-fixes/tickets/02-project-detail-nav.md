# 02 — 项目详情导航修正（issues 7, 8）

**What to build:** 让项目详情"扫描任务"tab 可定位到运行表区；删除顶部"正式报告"tab（与"预览 HTML 报告"重复）。

**Blocked by:** None

**Status:** done

## 完成条件

- `project_detail.html` 运行表所在 `<section>` 增加 `id="runs"`，"扫描任务"tab 的 `#runs` 锚点生效。
- 删除顶部 `正式报告` tab；面板内"预览 HTML 报告"与"导出 DOCX 报告"保留且可用。
- 结构改动，以渲染/视觉检查验收，无单测。
