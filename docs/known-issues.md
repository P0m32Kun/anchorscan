# 已知问题

> 活文档：解决的问题删除，未解决的留下。其他 agent 可直接读取本文件获取待修问题。
>
> 解决一条后直接从本文件删除对应条目，并在提交消息中引用其 ID。
>
> 新增条目时分配 `ISSUE-NNN` 编号，写清：现象、复现步骤、影响、来源 ticket、建议方向。

---

## ISSUE-002 — internal/app 租约竞争测试 CI 偶发失败（flaky）

**现象：** `TestManagerRejectsRunHeldByAnotherManager` 在 GitHub Actions quality-gate 偶发失败（某个 attempt-N 报 `second manager error = <nil>`，期望 `scan already running: run-1`）。

**复现步骤：** 不固定。本地 `go test ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=3`（45 次 attempt）全过；CI 上观察到 1/20 attempt 单次失败一次，rerun 即恢复。

**影响：** 低。CI 偶发红、rerun 可恢复；不影响功能正确性（租约逻辑另有 run_lease_test.go 覆盖）。

**来源：** PR #36 quality-gate（2026-08-06，run 31110136851，修复 scroll-spy 后的 rerun 上出现）。

**建议方向：**
- 疑似两个独立 store 连接（modernc sqlite）对同一 db 文件的锁时序竞态，涉及 `reserveRunLease` → `ReconcileInterruptedRuns` / `AcquireRunLease`（internal/app/run_lease.go）。
- 排查需先在 CI 稳定复现；可用 `-race` 或隔离 attempt 失败定位。修复前如再遇，rerun 即可。
