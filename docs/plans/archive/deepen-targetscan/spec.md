# 深化单目标扫描模块（TargetScan）

## 背景与动机

`internal/app` 的扫描编排是近 4 个月 git 热点之一。单目标扫描函数 `scanTarget` 是一个**浅模块**：

- 返回位置式 4 元组 `(fingerprints, findings, openPorts, err)`，而它在唯一调用点（`scan_targets.go:72`）立即被重新装箱进已存在的 `targetResult` 结构体——真实接口其实是 `targetResult`，却以裸值穿过接缝。
- 内部直接调用 `store.SaveFingerprint` / `store.SaveFinding`，把**工具流水线**（rustscan→nmap→httpx）、**结果持久化**（SQLite）、**进度事件**（`emit` 写 `ScanEvent`）三条关注点揉在一个 187 行的函数里。
- 因此 `scanTarget` 必须接收 `*store.Store`（二十余方法的「山」）才能测，导致 `scan_target_test.go` / `scan_targets_test.go` 共 16 个测试只能走 `RunScan` 端到端、依赖真实 SQLite。

## 目标

把 `scanTarget` 深化为**纯流水线**：行为零变化的前提下，结果持久化与进度事件各自落到清晰接缝之后，签名收窄到无需 `*store.Store`，从而可在不依赖 SQLite 的情况下直接测试。

## 设计决策（已与用户 grill 共识）

1. **范围**：正名 + 抽出结果持久化（不做制品接缝、不做 intake 词汇统一——后者属另一候选）。
2. **结果持久化落点**：fan-out（`scanTargets`）逐 `targetResult` 落地，**精确保留**逐目标增量入库的现有行为（扫描中途查看报告仍可见已完成目标的部分结果）。
3. **进度事件处理**：注入窄 `Progress` 接口，取代散布在 `scanTarget` 内的 `emit(opts, scanStore, …)`。
4. **Progress 契约**（定义在 `internal/app/progress.go`，消费方一侧）：
   - 单方法 `Emit(level, stage, format string, args …any)`：`fmt.Sprintf` + 调 logger + `AppendScanEvent` 三合一。
   - `storeProgress` 适配器封装 `runID` + `log`（原 `opts.Logf`）+ `*store.Store` + `now func() time.Time`（默认 `time.Now()`）。
   - 统一 `emit`（scan.go）与 `emitTool`（tool_run.go）——二者当前干同一件事，顺带修掉 `emitTool` 缺失的 `RunID==""||store==nil` 守卫。
   - `opts.Logf` 保留给 `scanTarget` 内 2 处纯日志（目标标记、nmap 心跳「still running」——每 30s，不能进事件流）。
5. **TargetScan 结果类型**（`internal/app`，替换 `targetResult`）：
   ```go
   type TargetScan struct {
       Target       string
       Fingerprints []fingerprint.ServiceFingerprint
       Findings     []report.Finding
       OpenPorts    []int
   }
   func scanTarget(...) (TargetScan, error)   // err 单独返回，不进结构体
   ```
   - 不含制品（现状是写盘副作用，收进来是新行为、超范围）。
   - 导出（领域词，进 CONTEXT.md）。
6. **聚合返回**：`scanTargets` 返回 `([]TargetScan, aliveIPs []string, error)`；展平挪给 `RunScan`（它本就是报告边界，挨着 `report.BuildWithScanData`）。per-result 持久化仍在 `scanTargets` 结果循环内完成（保留增量），故 `scanTargets` 保留 `*store.Store` 做持久化。
7. **测试策略**：现有 16 个 `TestRunScan*` 黑盒测试**原样不动**（行为零变化→直接作回归网）；新增 1–2 个直接 `TestScanTarget*`（假 runner + noop/录制 `Progress` + `t.TempDir()`，断言返回的 `TargetScan`，复用 `recordingSequenceRunner`）；不加 fan-out 持久化专项测试（被 RunScan 覆盖）。
8. **落地节奏**：分两段，各自编译通过、测试全绿、可独立交付/回滚。

## 关键约束（事实核查结论）

- **实时进度靠 `emit` 写的 `ScanEvent` 驱动**（`/runs/:id/status` + `/runs/:id/events`），**不靠** fingerprint/finding；后者只被报告视图（`report_handler.go`）读取。→ 把结果持久化搬出 `scanTarget` 不破坏实时进度。
- `scanTarget` 唯一调用点：`scan_targets.go:72`；`scanTargets` 唯一调用点：`scan.go`（`RunScan`）。重构面小。

## 领域词汇（建议入 CONTEXT.md，项目暂无）

- **Scan Run**：一次扫描运行（RunID 标识）。
- **Target**：被扫描的单个主机/IP。
- **TargetScan**：单目标扫描产出的结果束（指纹 + 漏洞 + 开放端口）。
- **Progress**：扫描进度事件流（level/stage/message）。
- **Fingerprint** / **Finding** / **Report**：已有类型，沿用。

## Review fixed point

`8622e312748fbbad93701bfd2d7eb056bd238ef5`（main）
