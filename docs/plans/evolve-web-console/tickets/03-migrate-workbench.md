# 03 — 迁移验证工作台

**What to build:** 将工作台页签、筛选、候选动作、证据 dialog 和反馈状态迁入一个 Vue 页面模块，并收敛操作密度。

**Blocked by:** 02 — 简化扫描创建流程。

**Status:** proposed

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

- [ ] 常用筛选常驻，低频筛选进入“更多筛选”。
- [ ] 每个候选保留一个主操作，命令、复制和资产级操作按上下文展开。
- [ ] tabs、dialog、文件选择、粘贴与拖放具有完整键盘和焦点行为。
- [ ] 普通错误与保存反馈替换阻塞式 `alert`；危险删除继续确认。
- [ ] Vue 成为工作台交互状态唯一所有者，删除对应 `workbench.js` 逻辑。
- [ ] 既有验证、证据和命令生成契约通过浏览器回归。
