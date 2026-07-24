# 06 — 迁移其余页面交互

**What to build:** 审计并迁移项目、知识库、配置和共享外壳中仍值得由 Vue 管理的交互，保留静态服务端内容。

**Blocked by:** 05 — 迁移 Run 状态与工具交互。

**Status:** ready-for-agent

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

- [ ] 逐页列出状态所有者，只有存在客户端状态的区域才挂载 Vue。
- [ ] 删除普通操作中的阻塞式 `alert`，破坏性动作使用可恢复焦点的确认 dialog。
- [ ] 共享导航、反馈和主题行为在所有页面一致。
- [ ] 不把静态表格、链接和简单 POST 表单组件化。
- [ ] 删除被替代的通用旧脚本并完成明暗主题、键盘和 1280/1440px 检查。
