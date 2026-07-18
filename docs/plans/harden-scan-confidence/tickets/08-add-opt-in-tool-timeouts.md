# 08 — 增加默认关闭的逐工具超时

**What to build:** 允许操作者为 rustscan、nmap、httpx、NSE、nuclei 显式配置全局 deadline，同时保证所有默认值为 `0` 且 profile 不会隐式覆盖。

**Blocked by:** `06-preserve-partial-results.md`。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] 配置接受 Go duration 字符串或 `0`，拒绝负数和无效文本。
- [x] 默认配置的五个 timeout 全部为 `0`，现有用户升级后行为不变。
- [x] preflight、CLI 摘要和配置页面显示实际生效值，并明确 `0` 表示不限时。
- [x] profile 不包含 timeout，也不能覆盖全局设置。
- [x] 零值直接复用原 context；非零值仅在对应工具调用周围使用 `context.WithTimeout`。
- [x] DeadlineExceeded 按基础/次要阶段语义进入 failed 或 completed_with_errors，不归类为操作者 canceled。
- [x] 测试证明默认 context 无 deadline，非零值生效，且没有新增 Runner 接口、重试器或 watchdog。
