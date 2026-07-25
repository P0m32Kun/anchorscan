# 07 — 现代化独立导出报告

**What to build:** 将自包含的独立 HTML 报告迁移到同一浅色视觉语言，同时保持离线打开、打印和现有数据内容。

**Blocked by:** 06 — 现代化知识库、配置与剩余页面。

**Status:** done

**Execution skills:** `frontend-visual-design`、`tdd`（报告生成 seam）、`browser:control-in-app-browser`、`ponytail`。

- [ ] 报告使用与 Web Console 一致的画布、文字、分隔线、风险色、表格和代码样式。
- [ ] 报告保持单文件、自包含，不引用 `/static/`、远程字体或外部资源。
- [ ] 不复制侧边栏、popover、吸顶工具栏等控制台专用结构。
- [ ] 所有现有资产、漏洞、证据和元数据继续输出，HTML 转义和导出接口不变。
- [ ] 增加或更新最小报告生成测试，证明内容与生成行为未回归。
- [ ] 提供 1440px 屏幕截图和打印预览，并与 Web 报告并排检查，经用户批准后完成。
