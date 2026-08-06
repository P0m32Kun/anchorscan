# 任务书：ISSUE-002 租约竞争测试 CI 偶发失败（根因定位 + 修复）

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。

## 背景

`docs/known-issues.md` ISSUE-002：`TestManagerRejectsRunHeldByAnotherManager` 在 GitHub Actions quality-gate 偶发失败（PR #36 的 rerun 上出现一次），失败形态：

```
--- FAIL: TestManagerRejectsRunHeldByAnotherManager/attempt-7 (0.13s)
    manager_test.go:85: second manager error = <nil>
```

期望第二个 manager 因 run-1 租约被持有而返回 `scan already running: run-1`，实际返回 nil（AcquireRunLease 成功）。本地 `-count=3`（45 次 attempt）全过，CI 上 1/20 概率单次 attempt 失败。

## 必读

1. `AGENTS.md`（仓库根，编排约定与硬约束）
2. `docs/known-issues.md`（ISSUE-002 条目）
3. `internal/app/run_lease.go`：`reserveRunLease` → `ReconcileInterruptedRuns` / `AcquireRunLease` / `RenewRunLease`
4. `internal/app/manager.go`：`Manager.Start`（m.mu + reserveRunLease 组合）
5. `internal/app/manager_test.go` 第 55-90 行：失败测试（两个独立 `store.Open(dbPath)` 连接）
6. `internal/store/` 的租约实现：`AcquireRunLease` / `ReconcileInterruptedRuns` / `RenewRunLease` / `ReleaseRunLease`（含 SQL 与事务逻辑）

## 已知线索（编排方初步分析，需验证）

- 测试用**两个独立 store 连接**（firstStore/secondStore）打开同一 sqlite 文件（modernc sqlite，WAL）。
- `Manager.Start` 先查 `m.activeID`（per-manager 内存锁，两个 manager 各自独立，无共享）→ 再 `reserveRunLease`（DB 租约）。
- second.Start 返回 nil = `AcquireRunLease` 没检测到 run-1 的租约。可能方向（不限于）：
  a. `ReconcileInterruptedRuns` 误删刚写入的租约（TTL 30s 判定竞态：first 写入的 heartbeat 时间戳与 second 的 Reconcile 读取之间）
  b. 两个连接在 WAL 下的可见性/锁时序（secondStore 的事务快照早于 firstStore 的提交）
  c. `AcquireRunLease` 的 INSERT/条件检查在并发连接下非原子（如先 SELECT 后 INSERT 无事务包裹）
- CI 比本地更容易触发（慢速 runner 放大时序窗口）。本地可用 `-race` + 高 attempt 数尝试复现，或构造确定性竞态（并发 Start 两个 manager 不同 run）。

## Scope

**要做**：
- 根因定位（给出代码级解释 + 复现证据或确定性推理）
- 修复：改 `internal/app/`（run_lease.go / manager.go）或 `internal/store/` 的租约实现；**若根因是测试本身的设计缺陷**（如时序假设不成立），改测试必须附充分论证
- 回归：修复后本地 `go test ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=5` 全过 + 说明为什么 CI 不会再偶发

**不要做**：
- 不做 git 操作（commit/push/checkout 一律禁止，完成后编排方会核对 git log）
- 不扩大改动面（不重构无关代码，不引入新依赖）
- 不用 `time.Sleep` 等魔法数值掩盖竞态（如有等待，说明其正确性论证）
- 不删除/弱化现有租约语义（TTL、heartbeat、中断恢复）

## 铁律

1. 零新依赖；改动限于 internal/app 与 internal/store 的租约相关代码 + 必要测试
2. 诚实报告：报告必须分列「确定性复现证据」与「推理论证」；无法复现时如实说明
3. 已知未跟踪文件（spikes/、docs/plans/fathom-integration/ 等）为既往产物，不得删除或修改，无需确认
4. 完成后不得自行 commit；`git status` 应只剩你的改动 + 既有未跟踪文件
5. 报告文件：`docs/reports/issue-002-report.md`，内容含：根因（代码级）、修复方案与改动文件清单、复现/验证证据（命令 + 输出）、为何 CI 不再偶发、遗留风险

## 验收

1. 本地 `go test ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=5` 全过
2. `go test ./internal/... -count=1` 全过（重点 internal/store、internal/app、internal/web）
3. `go test -race ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=2` 通过（若 -race 可用）
4. 报告给出根因的代码级解释，且能解释"CI 偶发、本地难复现"的差异
5. 修复不改变租约语义（TTL/heartbeat/中断恢复行为）
