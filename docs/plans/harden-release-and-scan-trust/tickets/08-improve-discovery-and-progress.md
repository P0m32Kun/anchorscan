# 08 — 改善发现策略、进度与取消

**What to build:** 增加显式主机发现模式、持久化 heartbeat、增量事件读取和经过平台验证的进程树取消。

**Blocked by:** 07 — 提供可靠备份与恢复。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- `auto` 使用 alive discovery；`assume-up` 跳过 `-sn` 并直接对 Scope 执行端口发现。
- 模式进入 ConfigSnapshot、Web 预填和报告说明。
- nmap heartbeat 使用 `Progress.Emit`，Web Run 中可见。
- events API 接受 `after_id` 并只返回新事件，终态停止轮询。
- 取消或超时终止扫描器进程树；平台行为无法证明时不宣称已支持。
- UDP 只修正文案/规则可达性，不在本 ticket 增加默认 UDP 扫描。

## 测试 seam

- App fake Runner：发现模式和 heartbeat。
- Store/HTTP：单调 event ID 与增量查询。
- OS-specific integration：进程树取消。
- Vue unit/Playwright：增量轮询和终态停止。

## 验收

- [ ] 先写屏蔽 alive discovery 时 `assume-up` 仍执行后续扫描的失败测试。
- [ ] 先写 Web 运行收不到 heartbeat 的失败测试。
- [ ] 先写 `after_id` 仍返回全部事件的失败测试。
- [ ] 取消分类保持 canceled，工具 timeout 保持失败/超时语义。
- [ ] 聚焦测试、`make test`、`go vet ./...`、`make pr-check` 通过。

## 非目标

- 不实现 checkpoint、任务队列、SSE/WebSocket 或默认全 UDP。
