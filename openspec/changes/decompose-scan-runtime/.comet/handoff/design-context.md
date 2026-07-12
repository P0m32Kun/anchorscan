# Comet Design Handoff

- Change: decompose-scan-runtime
- Phase: design
- Mode: compact
- Context hash: 5ffae416991d131fbfe8d6f1d1a3cbd15ae3463bcc9c9c0d253708b2f6075718

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/decompose-scan-runtime/proposal.md

- Source: openspec/changes/decompose-scan-runtime/proposal.md
- Lines: 1-28
- SHA256: 9b970e0e6be34d2fc68628e7f7ab75c0c1b7d89b3e5724e1e8d7cb6d0c387cf7

```md
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

```

## openspec/changes/decompose-scan-runtime/design.md

- Source: openspec/changes/decompose-scan-runtime/design.md
- Lines: 1-76
- SHA256: 8e97af6106cde5abc735310b844899ce782db55e912c242f6acd7be97eb85e20

```md
## Context

`RunScan` 是 CLI 直接执行和 Web `Manager` 异步执行共同依赖的稳定入口。当前实现从 run 初始化一直延伸到存活探测、worker pool、单目标扫描、失败汇总和 JSON 报告提交；同一文件中的 `scanTarget` 又串联 RustScan、Nmap、HTTPX、NSE 与 nuclei。对应测试集中在一个大型 `scan_test.go`，局部行为与全流程场景相互穿插。

前置 change `unify-scan-use-case` 负责把入口输入转换为最终 `ScanOptions`；本 change 只消费该结果并整理执行内部，不重新解释配置或请求。

## Goals / Non-Goals

**Goals:**

- 让 run 生命周期、目标调度和单目标流水线各有一个清晰修改原因。
- 保持 `RunScan`、`ScanOptions`、`Manager` 和现有工具 Runner 契约稳定。
- 让测试按可观察行为组织，能够独立定位生命周期、并发/失败和工具流水线回归。
- 每一步先做机械提取，再做删除和命名整理，保持可审查的小范围 diff。

**Non-Goals:**

- 不改变工具调用、并发、取消、事件、心跳、错误优先级、artifact 或报告行为。
- 不把固定流水线改为动态 stage registry、插件系统或配置驱动工作流。
- 不创建 `runtime`、`pipeline` 等新子包，不为每个阶段增加接口或对象。
- 不顺带重构 `tool_run.go`、Manager、store、report 或工具实现。

## Decisions

### 1. 保持同包拆分和单一公开入口

所有实现继续位于 `internal/app`，`RunScan` 仍是唯一公开的完整扫描运行入口。拆分只产生同包私有函数和按职责命名的文件，因此不需要导出新 API、依赖注入层或包间 DTO。

推荐同包拆分，而不是新建子包：当前流程高度共享 `ScanOptions`、store、事件和报告类型，强行跨包会制造接口与转换。保持单文件则无法降低修改冲突；动态流水线更灵活，但项目没有运行时重排阶段的需求。

### 2. 以状态所有权划分三个边界

- 生命周期边界拥有 artifact 目录建立、run 状态写入与最终状态更新、全局结果聚合和报告提交。
- 调度边界拥有存活主机结果、worker 数量限制、目标分发、结果收集、取消及部分/全部失败汇总。
- 单目标边界拥有现有固定工具顺序、单目标 fingerprint/finding 累积、阶段事件和对应 artifact。

文件名在详细设计时根据最终符号集合确定，不以行数为阈值。错误归一化、事件和 artifact helper 只有存在多个真实调用方时才独立保留；否则放在拥有其语义的边界旁边。

### 3. 不引入通用运行上下文对象

三个边界继续显式接收当前需要的 `context.Context`、`tools.Runner`、store、`ScanOptions` 和少量结果值。只有当同一组参数在真实调用中反复传递且能减少认知负担时，才使用一个私有值结构；不得创建可变的“全局 scan context”承载所有状态。

这比新增 orchestrator 类或每阶段对象更容易追踪数据来源，也避免隐藏并发共享状态。

### 4. 原样保留并发和失败决策顺序

目标 channel、结果 channel、worker 数量裁剪、等待与关闭顺序先原样迁移。结果处理继续区分成功、普通失败和 `context.Canceled`：部分目标失败时记录错误事件并提交成功结果；全部目标失败时返回包装后的首个错误；取消优先返回取消错误。

本 change 不借机优化 goroutine 数量、排序结果或错误聚合。任何性能或确定性改动必须以独立 change 和基准/行为需求提出。

### 5. 测试按行为边界拆分但复用现有测试基础

现有 Runner fake、临时 store 和通用断言继续复用；不创建测试框架。生命周期测试覆盖 run 状态与报告，调度测试覆盖 worker/取消/失败，单目标测试覆盖工具参数、顺序、事件与 artifact。仍保留少量端到端 `RunScan` 测试验证边界组合。

测试拆分不要求生产文件与测试文件一一对应，也不拆解只包含简单断言的小测试。

## Risks / Trade-offs

- [风险] 机械搬移 goroutine 或 defer 时改变关闭和取消时序。→ 先用并发/取消特征测试固定行为，再逐块迁移且不同时重写算法。
- [风险] 结果收集顺序变化导致报告或事件顺序变化。→ 保留现有 channel 缓冲和聚合顺序，并比较代表性输出与事件序列。
- [风险] run 状态 defer 被移动后覆盖原始错误。→ 生命周期测试分别覆盖完成、普通失败和取消状态及 message。
- [风险] 拆分产生大量只转发参数的小函数。→ 仅保留三个真实边界；一行包装或单调用 helper 直接内联。
- [取舍] 固定流水线仍不支持动态增删阶段。→ 当前产品没有该需求，显式代码更容易验证；出现真实可配置流程需求时再单独设计。

## Migration Plan

1. 补齐当前 `RunScan` 的生命周期、调度、事件、artifact 和失败优先级特征测试。
2. 先提取单目标流水线，再提取目标调度，最后让 `RunScan` 收敛为生命周期编排。
3. 按行为边界移动测试并删除重复 fixture，保证每一步 `go test ./internal/app` 通过。
4. 运行全仓测试、竞态检查和代表性扫描冒烟验证，确认调用方和输出无变化。

没有数据迁移或双轨运行；若行为不一致，回退最近一个机械提取步骤，而不是增加兼容分支。

## Open Questions

- 详细设计阶段根据现有 artifact/event helper 的实际调用方确定它们继续独立成文件还是归回生命周期/单目标边界；不预设额外模块。

```

## openspec/changes/decompose-scan-runtime/tasks.md

- Source: openspec/changes/decompose-scan-runtime/tasks.md
- Lines: 1-23
- SHA256: 2b1e3803101970575400b629eb545cd905789fd2be5279e4f767d5237382306b

```md
## 1. 固定运行时行为

- [ ] 1.1 补充 run 完成、普通失败、取消和报告写入失败的特征测试，固定状态、错误和 defer 行为。
- [ ] 1.2 补充 worker 边界、部分失败、全部失败、分发期间取消和结果聚合的并发特征测试。
- [ ] 1.3 补充代表性单目标工具顺序、参数、事件、心跳与 artifact 文件测试。

## 2. 按责任机械拆分

- [ ] 2.1 在 `internal/app` 同包内提取单目标固定流水线，保持现有函数调用和数据累积原样。
- [ ] 2.2 提取存活目标处理与多目标调度边界，保持 channel、worker、取消和失败汇总顺序原样。
- [ ] 2.3 收敛 `RunScan` 为生命周期编排入口，保留 artifact 目录、run 状态和最终报告职责。
- [ ] 2.4 将事件、错误和 artifact helper 放回实际所有者，删除无价值的一行包装与重复代码。

## 3. 整理测试职责

- [ ] 3.1 按生命周期、调度和单目标流水线拆分大型扫描测试，同时复用现有 Runner fake、store fixture 和通用断言。
- [ ] 3.2 保留少量完整 `RunScan` 组合测试，确认拆分后的边界可以共同复现现有流程。

## 4. 兼容性验证

- [ ] 4.1 运行 `go test ./internal/app`、`go test ./...` 和适用的 `go test -race ./internal/app`。
- [ ] 4.2 运行代表性扫描冒烟检查与 `make package`，比较工具参数、事件序列、run 状态、artifact 和报告。
- [ ] 4.3 确认公开运行入口、CLI/Web 调用、数据库与依赖文件均未变化。

```

## openspec/changes/decompose-scan-runtime/specs/scan-runtime-boundaries/spec.md

- Source: openspec/changes/decompose-scan-runtime/specs/scan-runtime-boundaries/spec.md
- Lines: 1-71
- SHA256: 34bf0dc7061e10d88c63fb3c89c95d29d9eff0621c1aa7b34c9081c09f67781f

```md
## ADDED Requirements

### Requirement: 扫描运行时保持单一稳定入口
系统 MUST 继续通过现有 `RunScan` 契约执行完整扫描，并由该入口协调 run 生命周期、目标调度、单目标流水线和报告提交；内部拆分不得要求 CLI、Web Manager 或其他调用方采用新的运行协议。

#### Scenario: CLI 与 Web 启动完整扫描
- **WHEN** CLI 直接调用或 Web Manager 异步调用完整扫描
- **THEN** 两者 MUST 继续传入现有扫描选项并获得与重构前相同的成功、失败或取消结果

### Requirement: Run 生命周期语义保持不变
系统 MUST 保持 artifact 目录创建、`running` 记录写入、完成/失败/取消状态更新、错误消息和最终 JSON 报告提交的现有顺序与条件。

#### Scenario: 扫描成功完成
- **WHEN** 所有必需阶段完成且报告写入成功
- **THEN** run MUST 以现有字段记录为 `completed`，并在配置路径写出等价 JSON 报告和过程文件

#### Scenario: 扫描普通失败
- **WHEN** 扫描在报告完成前返回普通错误
- **THEN** run MUST 标记为 `failed` 并保存现有错误消息语义，不得误记为完成

#### Scenario: 扫描被取消
- **WHEN** context 在执行过程中被取消
- **THEN** 运行时 MUST 返回可由 `errors.Is` 识别的取消错误，并把 run 标记为 `canceled`

### Requirement: 多目标调度与失败汇总保持不变
系统 MUST 使用现有 HostWorkers 规则调度目标，保证每个已分发目标最多执行一次，并保持部分失败、全部失败和取消的现有决策优先级。

#### Scenario: Worker 数量边界
- **WHEN** HostWorkers 小于等于零、超过目标数或位于有效范围内
- **THEN** 实际 worker 数 MUST 分别按现有规则收敛为可执行下限、目标数上限或请求值

#### Scenario: 部分目标失败
- **WHEN** 至少一个目标成功且另一个目标返回普通错误
- **THEN** 运行时 MUST 为失败目标记录现有错误事件，并用成功目标的结果继续生成报告

#### Scenario: 全部目标失败
- **WHEN** 所有已扫描目标均返回普通错误
- **THEN** 运行时 MUST 返回现有 `all targets failed` 包装语义，且不得把 run 标记为完成

#### Scenario: 调度期间取消
- **WHEN** context 在目标分发或结果收集期间取消
- **THEN** 调度 MUST 停止继续分发，并优先返回取消结果而不是普通目标失败汇总

### Requirement: 单目标工具流水线保持显式且兼容
系统 MUST 保持当前 RustScan、Nmap、HTTPX、NSE 与 nuclei 的启用条件、调用顺序、参数、错误处理和数据累积方式，不得通过本次拆分改为动态阶段注册。

#### Scenario: 完整单目标扫描
- **WHEN** 所有工具均按当前配置启用且目标可达
- **THEN** 运行时 MUST 按现有顺序调用工具，并生成等价 fingerprint、finding、事件与 artifact

#### Scenario: 无存活主机
- **WHEN** 现有 Nmap 存活探测返回空目标集
- **THEN** 运行时 MUST 发出当前跳过提示、不得启动后续端口扫描，并按现有规则完成报告

### Requirement: 事件、心跳和产物契约保持不变
系统 MUST 保持现有事件级别、阶段名、关键消息、Nmap 心跳条件，以及 artifact 目录和文件命名；内部文件拆分不得改变调用方可观察内容。

#### Scenario: 长时间 Nmap 执行
- **WHEN** Nmap 阶段运行时间达到当前心跳间隔
- **THEN** 系统 MUST 继续按现有阶段和消息语义发出心跳，完成或取消后停止心跳

#### Scenario: 代表性扫描产物
- **WHEN** 一次扫描产生 RustScan、Nmap、HTTPX、NSE 或 nuclei 输出
- **THEN** 对应原始输出 MUST 继续写入相同 run artifact 目录并使用现有文件名

### Requirement: 拆分不引入推测性运行框架
实现 MUST 继续复用现有 `tools.Runner` 和具体流程函数，不得为固定阶段新增插件注册表、依赖注入容器、每阶段单实现接口或生产依赖。

#### Scenario: 重构完成后的依赖与入口
- **WHEN** 该 change 完成
- **THEN** `go.mod`/`go.sum` MUST 不因本次重构变化，且完整扫描仍只有一个公开运行入口

```
