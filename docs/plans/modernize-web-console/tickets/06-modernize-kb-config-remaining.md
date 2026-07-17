# 06 — 现代化知识库、配置与剩余页面

**What to build:** 完成知识库列表/详情、全局配置及仍未迁移的控制台页面，使整个 Web Console 不再混用新旧视觉。

**Blocked by:** 05 — 现代化任务、工具与导入流程。

**Status:** done

**Execution skills:** `frontend-visual-design`、`browser:control-in-app-browser`、`ponytail`。

- [ ] 知识库列表突出漏洞名、风险和匹配标识，详情按说明、匹配、命令和修复信息分区。
- [ ] 配置页的基础字段、YAML 编辑器和高危端口维护形成稳定层级。
- [ ] 成功、警告、错误、空状态和禁用状态使用统一语义样式。
- [ ] 检查所有 `internal/web/templates/*.html`，移除剩余黑橙规则和非必要展示内联样式。
- [ ] 不改变知识库内容、配置字段、保存语义或重启提示。
- [ ] 相关 Go/JS 测试通过。
- [ ] 提供 1440px 知识库列表/详情和配置页截图，经用户批准后完成。
