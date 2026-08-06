# ISSUE-002 根因定位与修复报告：租约竞争测试 CI 偶发失败

> 任务书：`docs/issue-002-brief.md`。结论先行：**根因是测试自身的时序设计缺陷**（`sleepRunner` 50ms 后自行完成并释放租约），生产租约代码（`internal/app` / `internal/store`）无缺陷、未改动。修复为 test-only。

## 1. 根因（代码级）

### 失败链路

`TestManagerRejectsRunHeldByAnotherManager` 断言「第二个 manager（独立 store 连接）在第一个 manager 持有 run-1 租约期间被拒绝」。该断言只在 **run-1 租约行真实存在** 时成立，而修复前的测试里租约只存活约 **60ms**：

1. 测试 helper `sleepRunner.Run` 在 `time.After(50 * time.Millisecond)` 后**自行返回**（不等 ctx 取消），模拟的工具进程 50ms 即"跑完"。
2. runner 返回 → `RunScan` 正常完成 → defer 调用 `finishLease("completed", ...)`（`internal/app/run_lease.go:107`）→ `store.FinishRunWithLease` → `DELETE FROM run_leases ...`（`internal/store/leases.go`）——**租约行被删除**。
3. `first.Start` 的同步部分（reserveRunLease + recordScanStart）约 1ms 即返回；从那一刻起约 60ms（50ms 定时器 + RunScan teardown）租约即释放（实测见 §3 变体 C）。
4. 测试 goroutine 在 `first.Start` 返回与 `second.Start` 执行之间若被调度停顿 **>~60ms**，`second.Start` 的 `reserveRunLease` 到达 `AcquireRunLease` 时租约行已不存在 → INSERT 成功 → `second.Start` 返回 `(run-2, nil)` → 测试报 `second manager error = <nil>`。

这正是 CI 观察到的失败形态（attempt-7 整段耗时 0.13s > 60ms 窗口，与此一致）。

### 为什么 CI 偶发、本地难复现

- 本地：两次 `Start` 调用相邻，间隔为微秒级，远小于 60ms 窗口，`-count=3`（45 次 attempt）从不触发。
- CI：GitHub Actions 共享 runner 负载高，测试 goroutine 被抢占数十至上百毫秒是常态（quality-gate 并行编译/测试、GC、race 检测等都会放大停顿）。一旦某次 attempt 的停顿越过 60ms 窗口即失败；rerun 时序不同即恢复。观测概率 ~1/20 attempt 与"停顿超窗口"的低频事件吻合。

### 排除的其他方向（任务书线索逐条核对）

- **(a) `ReconcileInterruptedRuns` 误删新鲜租约**：不成立。`runLeaseFresh` 以 TTL=30s 判定，run-1 心跳在 second 的 Reconcile 前毫秒级写入，必判 fresh；且 Reconcile 的第二条 UPDATE 只动 `scan_runs`/`detection_checks`，不删租约。
- **(b) WAL 下两连接可见性/快照**：不成立。每个 store `SetMaxOpenConns(1)`，每条语句独立 autocommit；WAL 读事务在语句开始时取快照，secondStore 在 `AcquireRunLease` 前没有未结束的读事务，必然看到 firstStore 已提交的租约行。决定性反证：§3 变体 B 中同样的双连接、同样注入 150ms 停顿，只要第一个扫描真实在跑，second 被**确定性**拒绝。
- **(c) `AcquireRunLease` 非原子**：不成立。`INSERT ... ON CONFLICT(scope) DO UPDATE ... WHERE <过期条件>` 是单条语句（配合 `_txlock=immediate`），冲突判定与写入原子完成；0 行变更后再 SELECT 返回 `ErrRunLeaseHeld` 的路径正确。

### 为什么生产语义是对的（修复不改生产代码的依据）

扫描完成后释放租约、此后允许其他 manager 启动新扫描，正是租约的预期语义（TTL/heartbeat/中断恢复行为均未涉及）。真实扫描持续分钟级且有 5s 心跳续租，"扫描还在跑而租约消失"的情形在生产路径不存在。缺陷只在测试：用一个会自行结束的 runner 去断言"扫描仍在运行期间的行为"。

## 2. 修复方案与改动文件清单

**改动文件（唯一）**：`internal/app/manager_test.go`（+17/-15 行）

- 将 `sleepRunner`（50ms 后自行完成）替换为 `waitForCancelRunner`：**只随 ctx 取消而结束**（`<-ctx.Done(); return nil, ctx.Err()`），并附注释说明为何不得自行完成。
- 该 helper 的 5 处用法（`TestManagerAllowsOnlyOneActiveScan`、`TestManagerAllowsOnlyOneActiveToolRun`、`testManagerRejectsRunHeldByAnotherManager`、`TestManagerStartRecordsScanRunBeforeReturning`、`TestManagerStartToolRecordsRunBeforeReturning`）全部相应改名。这 5 个测试原本就都显式 `cancel()` 后才 `waitForInactive`，无任何测试依赖 50ms 自动完成或其返回的输出内容，语义等价。

**修复效果**：run-1 租约从 `first.Start` 返回起一直持有到测试显式 `cancel()`（heartbeat 5s 续租，即使无心跳 TTL 30s 也远大于测试时长）。`second.Start` 的拒绝从"仅 60ms 窗口内成立"变为"整个测试期间确定性成立"，竞态在结构上被消除——未使用任何 `time.Sleep` 或魔法数值掩盖。

## 3. 复现 / 验证证据

### 确定性复现证据（修复前，临时测试文件，验证后已删除）

临时 `internal/app/issue002_repro_test.go`，三个变体，`go test ./internal/app/ -run 'TestIssue002' -v -count=1` 输出：

```
=== RUN   TestIssue002ReproStallWithSleepRunner
[ISSUE002-A] second.Start err = <nil>, first.ActiveRunID() = ""      ← 精确复现 CI 症状
--- PASS: TestIssue002ReproStallWithSleepRunner (0.17s)
=== RUN   TestIssue002ControlStallWithBlockingRunner
[ISSUE002-B] second.Start err = scan already running: run-1, first.ActiveRunID() = "run-1"
--- PASS: TestIssue002ControlStallWithBlockingRunner (0.17s)
=== RUN   TestIssue002MeasureLeaseLifetime
[ISSUE002-C] lease for run-1 released 60ms after Start returned (Start sync part took 1ms)
--- PASS: TestIssue002MeasureLeaseLifetime (0.07s)
```

- **变体 A（复现）**：原样复刻失败测试，仅在两次 `Start` 间注入 150ms 停顿（模拟 CI 调度抢占）→ `second.Start` 返回 `<nil>`，与 CI 失败日志 `second manager error = <nil>` 完全一致。停顿只用于**暴露**窗口，不进入提交代码。
- **变体 B（对照）**：同样的双连接、同样的 150ms 停顿，仅把 runner 换成阻塞式 → 确定性拒绝。证明租约 SQL、WAL 跨连接可见性、心跳/ TTL 判定均无缺陷；失败唯一诱因是"第一个扫描已自行完成"。
- **变体 C（测量）**：`ActiveRunLease` 轮询实测租约在 `Start` 返回后 **60ms** 释放——竞态窗口宽度的直接测量。

### 修复后验证（命令 + 结果）

| 命令 | 结果 |
|---|---|
| `go vet ./internal/app/` | 通过 |
| `go test ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=5` | ok（100 次 attempt 全过，2.114s） |
| `go test -race ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=2` | ok（6.805s） |
| `go test ./internal/... -count=1` | 全部 ok（app / store / web 及其余 13 个包） |
| 压测：8 路 CPU 饱和 burner + `go test ./internal/app/ -run TestManagerRejectsRunHeldByAnotherManager -count=25` | ok（500 次 attempt 全过，6.821s）——模拟 CI 高负载调度停顿 |

### 推理论证（与实证互补）

修复后的不变量：`first.Start` 返回 ⇒ 租约行已提交且由阻塞中的扫描持续持有（心跳 5s 续租，TTL 30s）；`second.Start` 的 `AcquireRunLease` 走 `INSERT ON CONFLICT ... WHERE <过期>` 单语句，面对新鲜租约必然 0 行变更 → 返回 `ErrRunLeaseHeld` → `scan already running: run-1`。该推导不依赖任何调度时序假设，测试 goroutine 任意停顿均不影响结论（压测行为此提供了经验佐证）。

## 4. 为何 CI 不会再偶发

失败充要条件是「second 的 AcquireRunLease 执行时 run-1 租约已释放」，而租约释放的唯一路径是扫描完成/取消（`FinishRunWithLease`/`ReleaseRunLease`）。修复后扫描只随测试末尾的显式 `cancel()` 结束，故在 `second.Start` 断言点租约必然存在且新鲜——失败条件被结构性移除，与 runner 负载、调度停顿、race 检测减速均无关。

## 5. 遗留风险与建议

- **同类隐患**：任何"断言扫描仍在运行"的测试若使用会自行完成的 runner，都会引入同类窗口。本次已在 `waitForCancelRunner` 注释中写明该约束；`internal/app` 内其余 runner fake（scan_test.go 的 `blockingRunner`、`cancelRunner` 等）用途不同，不受影响。
- **`docs/known-issues.md`**：本文件按仓库约定未改动；建议编排方在提交修复时删除 ISSUE-002 条目并在提交消息中引用 `ISSUE-002`。
- **临时复现文件** `issue002_repro_test.go` 已删除；如需回归该窗口，可按 §3 变体 A 重建（复刻旧测试 + 两次 Start 间 sleep 150ms）。
- 未做 git 操作；`git status` 仅 `M internal/app/manager_test.go` + 既有未跟踪 `spikes/`。
