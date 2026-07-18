# 03 — 强制全局 Run Lease

**What to build:** 使用 SQLite 单行租约统一完整扫描和单工具入口的执行所有权，阻止 CLI/Web 跨进程并发，并用 owner token 安全续租和终结。

**Blocked by:** None — can start immediately.

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] 新 migration 创建全局 Run Lease 数据结构，不修改旧 migration。
- [x] 完整扫描、Web Manager 和单工具运行在执行前经过同一原子租约获取路径。
- [x] 新鲜租约存在时明确拒绝新任务并返回活动 Run ID，不创建幽灵 `running` Run。
- [x] heartbeat 使用 owner token 条件续租；所有权丢失时取消本地 context。
- [x] Run 终结与租约释放校验 owner token，旧进程不能覆盖新所有者状态或删除新租约。
- [x] store/app/CLI 公共 seam 使用临时 SQLite 证明跨入口冲突，过期时间测试不等待真实时钟。
- [x] 不增加队列、抢占、远程 worker 或新的协调服务。

## 验收记录

- v4/v5 migration 兼容：旧文本心跳与纳秒心跳同时过期才允许接管。
- 两个 Store/Manager 共用临时 SQLite 的测试证明跨入口冲突同步返回活动 Run ID，且不会创建第二条 Run。
- `make pr-check` 通过；Standards 与 Spec 双轴复审无遗留发现。
