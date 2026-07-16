# Ticket 02 — 搬迁：scanTarget 纯流水线化 + fan-out 持久化

- 状态：`done`（实现 c248ffa + 修正 3315423，双轴 code-review 通过，无 blocker/major）
- Blocked by：`01-prep-concepts.md`（已 done）
- 所属 spec：`docs/plans/deepen-targetscan/spec.md`

## 目标

把结果持久化与进度事件从 `scanTarget` 内搬出，使其成为不依赖 `*store.Store` 的纯流水线；兑现可测性杠杆。

## 变更清单

1. **`scanTarget`**：`emit(opts, scanStore, …)` → `progress.Emit(…)`；移除 `scanStore` 参数，新增 `progress Progress`；移除内联 `SaveFingerprint`/`SaveFinding`。返回 `(TargetScan, error)`。保留 2 处纯 `logf`（目标标记、nmap 心跳）。
2. **`scanTargets`**：接收 `progress Progress`（取代通过 `emit` 间接拿 store 写事件）；在结果循环内对每个成功 `targetResult` 执行 `SaveFingerprint`/`SaveFinding`（逐目标落地）；返回 `([]TargetScan, aliveIPs []string, error)`。
3. **`scan.go` 的 `RunScan`**：构造 `storeProgress{runID: opts.RunID, log: opts.Logf, store: scanStore, now: time.Now}` 传入 `scanTargets`；将 `[]TargetScan` 展平为 `report.BuildWithScanData` 所需的 fingerprints/findings/`ScanData{AliveIPs, OpenPorts}`。
4. **新增直接测试** `TestScanTarget*`（`scan_target_test.go` 或新文件）：假 `recordingSequenceRunner` + noop/录制 `Progress` + `t.TempDir()`，断言返回的 `TargetScan`（指纹/漏洞/开放端口），**不建 SQLite**。

## 验收

- `go build ./...`、`go test ./...` 全绿。
- 现有 16 个 `TestRunScan*` 仍零修改通过（行为不变）。
- 新增 `TestScanTarget*` 覆盖流水线分支（有/无开放端口、web→httpx、NSE、nuclei）且不依赖 SQLite。
- `scanTarget` 签名不再含 `*store.Store`。

## Review 结果（双轴）

- **Spec: PASS** —— 6 项验收全过（scanTarget 甩 store、progress seam、fan-out 逐 target 持久化、`[]TargetScan` 返回、RunScan 展平、直接无 SQLite 测试）；行为等价已核实（fingerprints/findings 表无 FK，List 查询按 ip/port 排序而非插入顺序）。
- **Standards: 1 MINOR（已修）+ 1 NIT（既有，留）** ——
  ① `[MINOR]` 局部变量 `scanTargets` 遮蔽函数名且语义双关 → 已重命名为 `targets`（commit 3315423）；
  ② `[NIT]` writeArtifact 错误检查模式重复 ~6×（既有、非本变更引入）→ 留待后续。
- 残余风险：persist 时机从「逐发现交织」变「逐 target 批量」—— 已由 ip/port 稳定排序保证观测等价（spec 决策 2 既定）。
- gofmt/build/vet/test/LSP 全绿；16 个 `TestRunScan*` 零修改通过。

## 完成后

按 tracker 契约：以 fixed point + spec 跑 `code-review`，分别处理 Standards 与 Spec 发现，修正、重验、提交，置本 ticket 为 `done`。
