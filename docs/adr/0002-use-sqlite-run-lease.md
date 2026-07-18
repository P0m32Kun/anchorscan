# ADR-0002：使用 SQLite Run Lease 约束单活动任务

- 状态：Accepted
- 日期：2026-07-17
- 关联计划：`docs/plans/harden-scan-confidence/`

## 背景（Context）

AnchorScan 的 Web `Manager` 只能阻止本进程内同时运行两个任务。CLI、Web 和单工具入口可连接同一 SQLite 数据库，进程崩溃后也没有持久化所有权事实，因此 Run 可能永久停在 `running`，或出现跨进程并发执行。

产品仍是单机、单用户工具，不需要任务队列、远程 worker 或分布式调度。

## 决策（Decision）

使用 SQLite 中的单行全局 Run Lease 作为唯一跨进程执行所有权：

- CLI、Web 和单工具入口在执行工具前原子获取租约。
- 有效租约存在时拒绝新任务，不排队、不抢占。
- 所有者使用随机 token 定期续租；终结和释放必须校验 token。
- 只有租约过期时，协调器才把旧 `running` Run 标记为 `interrupted`。
- 保留已持久化结果，不实现 checkpoint 或断点续跑；重跑必须由用户确认新表单。

工具默认不设置 deadline。Run Lease 解决的是执行所有权和进程失联，不是用超时猜测工具是否卡死。

## 后果（Consequences）

- 同一数据库同时最多有一个扫描或单工具任务。
- 进程崩溃后的状态会在下一次启动协调或租约获取时收敛，而不是依赖常驻 watchdog。
- Web 进程内 `Manager` 可继续提供即时取消，但不再承担跨进程正确性。
- 暂不支持远程取消、任务队列、并行 Run、抢占和恢复执行。
- 若未来明确需要多 worker，再重新评估按资源作用域拆分租约；当前全局单行方案是有意的吞吐上限。

