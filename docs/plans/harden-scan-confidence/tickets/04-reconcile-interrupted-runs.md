# 04 — 协调中断 Run 并提供确认重跑

**What to build:** 在 Run Lease 基础上收敛崩溃或失联进程留下的状态，保留历史事实，并让用户从预填表单确认后创建新 Run。

**Blocked by:** `03-enforce-global-run-lease.md`。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] Web 启动和新任务获取租约时协调过期租约，把对应 `running` Run 标记为 `interrupted`。
- [x] 升级时把没有可续租所有者的遗留 `running` Run 一次性收敛为 `interrupted`。
- [x] 新增 interrupted 状态展示，保留已有 Fingerprint、Finding、Artifact、ScanEvent 和报告数据。
- [x] interrupted Run 提供预填重跑入口；读取白名单快照字段，必须再次提交才执行。
- [x] 新 Run 使用新 Run ID，旧 Run 和历史结果不可变。
- [x] 测试证明新鲜租约不会被误判、过期租约只协调一次、打开重跑页不会自动执行。
- [x] 不实现 checkpoint、断点恢复或跨进程远程取消。
