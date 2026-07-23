# 01 — Vue 构建基础与全局主题

**What to build:** 接入 Vue 3、Vite、TypeScript 和确定性嵌入构建，以 `system/light/dark` 全局主题作为首个生产垂直切片。

**Blocked by:** None — can start after this plan is approved.

**Status:** ready-for-agent

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

- [ ] 前端类型检查、生产构建和 Go embed 接入 Makefile、PR CI 与 release。
- [ ] 首次绘制前解析主题，显式偏好跨刷新保留并支持系统运行时变化。
- [ ] 所有控制台表面、表单、浮层、表格、代码区和滚动条覆盖明暗令牌。
- [ ] 侧边栏外观入口与配置页共享同一偏好来源。
- [ ] 独立导出报告保持浅色打印行为。
- [ ] 主题 smoke、键盘检查、1440px 明暗截图与 `make pr-check` 通过。
