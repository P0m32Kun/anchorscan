# ADR-0001：暂不统一扫描摄入结构体（ScanOptions / PrepareScanRequest / ToolRunOptions）

- 状态：Accepted
- 日期：2026-07-17
- 背景：架构深化评审候选 #3（见 `docs/plans/` 评审报告）

## 背景（Context）

三个「扫描摄入」结构体存在字段重叠：

| 字段 | ScanOptions | PrepareScanRequest | ToolRunOptions |
|------|:---:|:---:|:---:|
| RunID | ● | ● | ● |
| ProjectID | ● | ● | ● |
| JSONReportPath | ● | ● | ● |
| Logf | ● | ● | ● |
| Ports / PortSpec | ● | ●(PortSpec) | ● |
| ArtifactRoot | ● | ● | — |
| Tools / ExtraArgs | ● | — | ● |

架构评审据此提出候选 #3：抽一个共享类型（如 `ScanContext{RunID, ProjectID, JSONReportPath, Logf}`）嵌入三者，统一摄入词汇。

## 决策（Decision）

**暂不统一。** 维持三个独立结构体。理由（承载性，load-bearing）：

1. **无参数级 Data Clump**：全代码库没有任何函数以 `(RunID, ProjectID)` 配对作形参传递；这些字段始终是结构体字段。重叠是结构体定义层面的偶然，非调用面摩擦。
2. **唯一真实消费者仅 1 处**：只有 `PrepareScan`（`scan_prepare.go:94`）把 `PrepareScanRequest` 逐字段拷成 `ScanOptions`。按「一个适配器＝假设接缝，两个才算真实接缝」，这是假设接缝，未到提取阈值。
3. **删除测试不通过**：抽共享类型会**移动/增加复杂度**而非集中——
   - 组合：所有 `opts.RunID`/`opts.Logf` 读取点要改成 `opts.Context.RunID`，波及全库；
   - 嵌入：读取点（提升字段）不变，但所有 `ScanOptions{RunID:...}` **字面量构造**必须改成 `ScanOptions{ScanContext: ScanContext{...}}`，波及大量构造点（含 `scan_test.go`/`tool_run_test.go`/`scan_prepare_test.go`）。
   - 收益仅省 `PrepareScan` 里 ~4 行字段拷贝。成本/收益严重失衡。
4. **三结构体不可互换**：分别服务于「完整扫描 / 从输入构建扫描 / 单工具执行」三种不同流程，并非同一概念的不同视图。

## 后果（Consequences）

- 可接受：RunID/ProjectID/JSONReportPath/Logf 在 3 个结构体定义里重复。若 RunID 语义变更，需同步 3 处（理论风险，可接受）。
- 领域词汇已在 `CONTEXT.md`（结构性概念表）为三者命名，提供讨论锚点。
- **重新评估触发条件**：当出现第 2 个真实消费者（例如某函数需同时接受「任意一种扫描摄入」并提取 RunID/ProjectID），或字段重叠增至 ≥6 个且含成对参数传递，再重启本候选。

## 关联

- 评审报告候选 #3（HTML：`<tmpdir>/architecture-review-*.html`，已过期）。
- `CONTEXT.md` 结构性概念表。
