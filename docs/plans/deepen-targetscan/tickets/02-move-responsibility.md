# Ticket 02 — 搬迁：scanTarget 纯流水线化 + fan-out 持久化

- 状态：`blocked`
- Blocked by：`01-prep-concepts.md`（须先 done）
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

## 完成后

按 tracker 契约：以 fixed point + spec 跑 `code-review`，分别处理 Standards 与 Spec 发现，修正、重验、提交，置本 ticket 为 `done`。
