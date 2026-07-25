# 06 — 迁移其余页面交互

**What to build:** 审计并迁移项目、知识库、配置和共享外壳中仍值得由 Vue 管理的交互，保留静态服务端内容。

**Blocked by:** 05 — 迁移 Run 状态与工具交互。

**Status:** done

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

| 区域 | 状态所有者 | 本票处理 |
| --- | --- | --- |
| 项目详情、项目编辑、运行历史 | Go 负责持久化；Vue 仅负责删除前确认 | 以共享原生 dialog 统一确认，保留原 POST 表单。 |
| 扫描导入、知识库、全局配置 | Go 与原生 HTML 表单 | 不引入 Vue，保持直接提交。 |
| 共享外壳 | Vue 主题控件；静态导航 | 延续已有全局主题和导航行为。 |
| 验证工作台 | Vue | 截图删除接入同一共享确认 dialog。 |

- [x] 逐页列出状态所有者，只有存在客户端状态的区域才挂载 Vue。
- [x] 删除普通操作中的阻塞式 `alert`，破坏性动作使用可恢复焦点的确认 dialog。
- [x] 共享导航、反馈和主题行为在所有页面一致。
- [x] 不把静态表格、链接和简单 POST 表单组件化。
- [x] 删除被替代的通用旧脚本并完成明暗主题、键盘和 1280/1440px 检查。
