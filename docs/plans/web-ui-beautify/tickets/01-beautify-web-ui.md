# 01 — Web 控制台排版美化实施

**What to build:** 对全局设置、控制面板、项目详情（含扫描历史、网络分区）和扫描报告页面进行 UI 美化和排版重构，解决 1280px 下大纲不可用、配置页滚动高亮错误、reduced-motion 抖动、文档与实际偏差等审查发现的问题。

**Blocked by:** None — can start after plan is approved.

**Status:** done

**Execution skills:** `frontend-design`、`browser:control-in-app-browser`、`code-review`。

- [x] 补齐本实施 ticket 并开启流程。
- [x] 解决 1280px 及以下报告大纲不可用与键盘不可达问题。
- [x] 修复配置页 scroll-spy 高亮包含嵌套区域的问题。
- [x] 修复 reduced-motion 下 hover 仍然发生位移的问题。
- [x] 修正设计文档与实物中残留的 Accordion 说法。
- [x] 补全 smoke 测试：配置页滚动后 active 项变化、1280px 下报告导航仍可见且可聚焦。
- [x] 确保 `npm run test:web` 和 `go test ./...` 完整通过。
