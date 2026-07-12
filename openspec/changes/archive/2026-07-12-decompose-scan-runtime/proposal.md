## Why

`internal/app/scan.go` 同时承担 run 生命周期、存活探测、多目标并发调度、单目标工具流水线、事件与失败汇总以及最终报告写入。任何局部优化都会触碰同一实现和超大测试文件，难以判断改动是否只影响预期阶段。

## What Changes

- 在保持 `internal/app` 包和现有公开调用边界的前提下，按生命周期、目标调度和单目标流水线的修改原因拆分扫描运行时。
- 让错误归一化、事件发射和 artifact 写入跟随其实际所有者，避免建立脱离流程的通用 helper 层。
- 按行为边界拆分扫描测试，固定运行状态、工具顺序、并发、取消、部分/全部失败、事件和产物契约。
- 保持固定扫描流水线显式可读，不引入阶段注册表、插件系统或新的单实现接口。
- 严格保持所有扫描执行及外部观察行为兼容。

## Capabilities

### New Capabilities

- `scan-runtime-boundaries`: 规定扫描生命周期、目标调度和单目标流水线具有清晰的内部责任边界，同时完整保留现有运行语义。

### Modified Capabilities

- 无。扫描行为和用户可见需求不变，本次仅重组既有运行时实现。

## Impact

- 主要影响 `internal/app/scan.go`、与扫描事件和 artifact 相关的同包文件，以及 `internal/app/scan_test.go` 的测试组织。
- 继续复用现有 `tools.Runner`、store、report、fingerprint 和 vuln 能力；不修改 CLI/Web 入口或 Manager 协议。
- 不改变数据库 schema、配置、工具参数与顺序、并发、取消、心跳、事件阶段、失败语义、artifact 名称或 JSON/HTML 输出。
- 不新增依赖、子系统接口、动态流水线框架或为了文件数量而进行的机械拆分。
