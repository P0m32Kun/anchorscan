# scan-runtime-boundaries Specification

## Purpose
TBD - created by archiving change decompose-scan-runtime. Update Purpose after archive.
## Requirements
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

