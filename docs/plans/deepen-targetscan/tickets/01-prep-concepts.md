# Ticket 01 — 备料：引入 Progress 接口与 TargetScan 类型

- 状态：`done`（实现 a2ad280，双轴 code-review 通过，无 blocker/major）
- Blocked by：无
- 所属 spec：`docs/plans/deepen-targetscan/spec.md`
- Review fixed point：`8622e312748fbbad93701bfd2d7eb056bd238ef5`

## 目标

**纯机械、行为零变化**地引入两个概念，为 Ticket 02 的责任搬迁铺路。本段独立可交付。

## 变更清单

1. **新增 `internal/app/progress.go`**：
   - `Progress` 接口：`Emit(level, stage, format string, args …any)`。
   - `storeProgress` 适配器：`{runID string; log func(string, …any); store *store.Store; now func() time.Time}`，`Emit` 做 `Sprintf` + `log` + （`runID!=""&&store!=nil` 时）`AppendScanEvent`。
2. **`scan.go` 的 `emit`**：函数签名不变（仍 `emit(opts, scanStore, level, stage, format, args…)`），函数体改为构造 `storeProgress{runID: opts.RunID, log: opts.Logf, store: scanStore, now: time.Now}` 并调 `Emit`。所有 `emit(...)` 调用点**不动**。
3. **`tool_run.go` 的 `emitTool`**：同样委托 `storeProgress`（顺带获得 `runID==""||store==nil` 守卫，修掉现有一致性缺口）。调用点不动。
4. **`scan_target.go` 新增 `TargetScan` 类型**（见 spec 决策 5）。
5. **`scanTarget` 返回值**：4 元组 → `(TargetScan, error)`。**签名仍保留 `scanStore`、仍内联持久化、仍调 `emit(...)`**——只改返回形状。
6. **`scan_targets.go`**：第 72 行调用点与 `targetResult` 适配 `TargetScan`（`targetResult` 改为 `{target string; scan TargetScan; err error}`，或保字段但源自 `TargetScan`）。

## 验收

- `go build ./...` 通过。
- `go test ./internal/app/...` 全绿（现有 16 个 `TestRunScan*` 行为不变，零修改即通过）。
- `go vet ./...` 无新增告警。
- 无行为变化：事件流、持久化、制品、报告输出均与 fixed point 一致（由现有测试覆盖）。

## Review 结果（双轴）

- **Spec: PASS** —— 6 项验收全过；residual：`emitTool` 统一后多了 RunID/store 守卫（spec 决策 4 既定的一致性修复，所有真实路径行为等价，非缺陷）。
- **Standards: 2 NIT，无 blocker/major** ——
  ① progress.go 导入分组：**误报**（文件本就正确，行 6 已有空行分隔 stdlib/internal 组）；
  ② emit/emitTool 的 `storeProgress{...}` 字面量重复：**留**（段 2 重构掉这两个 wrapper，加构造器即被删，Ponytail）。
- gofmt/build/vet/test/LSP 全绿。

## 不做（留给 Ticket 02）

- 不动 `scanTarget` 的 `scanStore` 参数。
- 不把持久化移到 fan-out。
- 不改 `scanTargets` 返回为 `[]TargetScan`。
- 不加直接 `TestScanTarget*`。
