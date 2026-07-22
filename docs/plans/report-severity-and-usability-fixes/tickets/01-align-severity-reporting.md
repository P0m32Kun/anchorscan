# 01 — 对齐扫描报告、DOCX 与手册的风险等级

**What to build:** 让扫描报告风险摘要累计低危；让项目 DOCX 的结论上下文和正式模板显示严重漏洞数量；在知识库手册中明确严重（critical）等级。风险等级统一为 Nuclei 的 critical/high/medium/low。

**Blocked by:** None

**Status:** done

## 完成条件

- 报告页面的严重、高危、中危、低危均显示实际计数。
- `DocxContext` 向模板提供 `conclusion.critical`，结论句式含“严重漏洞”。
- 正式模板经渲染与视觉检查后无残留占位符、无版式回归。
- 手册等级说明包含严重（critical），与解析器保持一致。
- 以报告 view/model 与 DOCX context 为测试接缝完成 red-green 回归测试。
