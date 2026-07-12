---
comet_change: decompose-scan-runtime
role: technical-design
canonical_spec: openspec
archived-with: 2026-07-12-decompose-scan-runtime
status: final
---

# 扫描运行时职责拆分技术设计

## 目标与约束

本设计细化 `openspec/changes/decompose-scan-runtime/`。它只重组 `internal/app` 内部实现，保持 `RunScan`、`ScanOptions`、Manager、`tools.Runner`、工具调用顺序、并发语义、取消优先级、事件、artifact 和报告输出不变。

不建立新子包、运行上下文对象、stage 接口或插件机制。拆分依据是状态所有权，而不是文件长度。

## 文件与符号边界

### `internal/app/scan.go`

保留：

- `ScanOptions`
- `RunScan`
- run 初始化和最终状态 `defer`
- artifact 目录建立
- 全局结果接收和最终 JSON 报告提交

`RunScan` 只编排现有三个步骤：初始化运行、可选存活探测及目标调度、提交报告。公开签名和调用方不动。

### `internal/app/scan_targets.go`

移动多目标职责：

- `targetResult`
- Nmap 存活探测及空结果处理
- `HostWorkers` 默认值与上限裁剪
- target/result channel、worker、wait/close 顺序
- 成功结果聚合、部分失败事件、全部失败和取消决策

实现先从现有代码整块移动，不改 channel 容量、goroutine 数量、发送顺序或首错选择。

### `internal/app/scan_target.go`

移动 `scanTarget` 及其直接拥有的固定流水线：RustScan 端口发现、Nmap 指纹、HTTPX 丰富、手工复核、NSE、Nuclei、单目标结果累积和阶段 artifact。

通用 helper 只有存在至少两个真实调用方时才独立保留；否则留在语义所有者旁。现有 `internal/app/artifacts.go` 继续复用，不新建 helper 层。

## 测试结构

先保留 `internal/app/scan_test.go` 作为现有端到端和共享 fake 的事实源，只新增以下最少测试文件：

- `scan_lifecycle_test.go`：完成、普通失败、取消、报告写入失败和最终状态
- `scan_targets_test.go`：worker 边界、部分/全部失败、分发取消
- `scan_target_test.go`：代表性工具顺序、参数、事件和 artifact

迁移已有测试时不重写断言；共享 fake 只有在多个文件实际使用后才移动到 `scan_test_helpers_test.go`。

## 实施顺序与回滚

1. 补齐会在当前实现上通过的特征测试。
2. 原样移动 `scanTarget`，运行 `go test ./internal/app`。
3. 原样移动存活探测和 worker pool，再运行测试及 race 检查。
4. 收敛 `RunScan`，删除只转发的一行 helper。
5. 运行全仓、竞态、打包和代表性扫描验证。

每一步保持可编译。出现行为差异时回退最近一次机械移动，不增加兼容分支。

## 验收标准

- `RunScan` 及其全部调用方无需修改。
- 完成、失败和取消状态及 message 与基线一致。
- 部分失败仍提交成功目标，全部失败仍包装首个错误，取消仍优先。
- 工具参数、调用顺序、事件、artifact、JSON 报告和打包结果不变。
- 不增加依赖、导出符号、接口或子包。


