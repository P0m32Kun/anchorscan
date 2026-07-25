# 02 — 覆盖核心 Web 工作流

**What to build:** 在 Ticket 01 的稳定 Playwright 基础上覆盖 AnchorScan 当前最重要的桌面工作流，让页面交互回归能以截图和 trace 直接定位。

**Blocked by:** `01-establish-pr-browser-gate.md`。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] 覆盖 Project 创建、扫描表单必填/错误校验、Run 启动与状态轮询、当前进程 Run 取消。
- [x] 覆盖确定性报告导入、报告筛选、分页和复制入口。
- [x] 覆盖配置页面的主要字段与保存/校验反馈。
- [x] 主流程使用 1440px，关键报告与表单在 1280px 验证可用性和非预期页面级溢出。
- [x] 选择器优先使用 role、label 和可见名称；不锁定 CSS 类、完整 DOM 或 goroutine 时序。
- [x] 用例隔离数据库和端口，可独立重跑且不依赖执行顺序。

## 验收记录

- `make pr-check` 通过；所有浏览器流程使用真实 Web 二进制、临时配置/SQLite、随机回环端口和本地 fixture。
- 取消通过当前页面的 POST 发起，并断言 `run-status.js` 轮询将运行态更新为 `canceled`。
- Standards 与 Spec 双轴复审无遗留发现。
