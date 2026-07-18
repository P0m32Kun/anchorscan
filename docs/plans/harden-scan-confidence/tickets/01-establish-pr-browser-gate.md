# 01 — 建立确定性 PR 浏览器门禁

**What to build:** 在现有测试与构建检查上增加最小 Playwright Chromium 基础设施，让 PR 能通过真实 Web 入口、临时 SQLite 和本地工具 fixture 执行一个稳定冒烟流程，并在失败时产出可读诊断制品。

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [ ] PR 门禁运行 Go 测试、原生 JavaScript 测试、构建/打包检查和 Playwright Chromium。
- [ ] Playwright 使用临时配置、数据库、报告目录和确定性本地工具 fixture，不依赖外网或真实扫描器。
- [ ] 冒烟流程至少证明 Web 服务启动、Project 创建和一个页面间导航可用。
- [ ] 失败时上传 screenshot、trace 和 browser console log。
- [ ] 不引入前端框架、多浏览器矩阵、像素快照平台或生产测试 endpoint。
- [ ] 聚焦测试与完整 PR 命令在本地可重复运行，并在文档中给出非程序员可执行的单条入口命令。
