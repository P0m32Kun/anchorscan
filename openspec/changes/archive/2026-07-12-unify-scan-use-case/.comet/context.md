# Comet Design Handoff

- Change: unify-scan-use-case
- Phase: design
- Mode: compact
- Context hash: a34eac23929d16b07e460a19455f1fe14a321d4419278901e939665266986058

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/unify-scan-use-case/proposal.md

- Source: openspec/changes/unify-scan-use-case/proposal.md
- Lines: 1-27
- SHA256: 9f4a765d59a91e2b5a8815d548ec25d6c6f10c41b78778c38b40a01626c54197

```md
## Why

CLI 与 Web 当前分别完成配置和规则加载、目标与端口解析、扫描档位覆盖、预检参数构造以及 `app.ScanOptions` 组装。同一业务入口存在两套实现，修改一侧时容易产生扫描计划和错误行为漂移，也让后续扫描能力迭代必须重复修改。

## What Changes

- 在现有应用层建立一个共享的扫描准备用例，将原始扫描请求标准化为可执行扫描计划及预检结果。
- CLI 与 Web 仅保留协议适配职责：采集各自输入、提供入口特有默认值、调用共享用例并呈现结果。
- 收敛扫描准备过程中工具路径、额外参数和预检数据的所有权，消除 `preflight` 对应用层类型的反向依赖，但不新增通用 `common` 包或推测性抽象。
- 用共享契约测试固定 CLI 与 Web 对等输入的标准化结果和失败语义。
- 严格保持 CLI、Web、数据库、报告以及扫描执行行为兼容。

## Capabilities

### New Capabilities

- `shared-scan-preparation`: 规定所有扫描入口必须通过同一准备边界生成标准化扫描计划，并保持入口间一致的校验和预检语义。

### Modified Capabilities

- 无。现有用户可见需求不变，本次新增的是约束既有行为一致性的内部架构能力。

## Impact

- 主要影响 `cmd/anchorscan`、`internal/web`、`internal/app`，以及必要的 `internal/config`、`internal/ports`、`internal/target`、`internal/preflight` 类型归属和相关测试。
- 不修改 CLI 参数或输出、Web 路由或表单、SQLite schema、JSON/HTML 格式、工具执行顺序和运行时语义。
- 不新增生产依赖、单实现接口、Repository/Service 层或面向未来的扩展框架。

```

## openspec/changes/unify-scan-use-case/design.md

- Source: openspec/changes/unify-scan-use-case/design.md
- Lines: 1-73
- SHA256: 80f186eaf75c49423b8ba0458665c6a949af07e70700dac851f6e71a85e4db2a

```md
## Context

`cmd/anchorscan.runScan` 与 `internal/web.(*server).scanCreate` 都负责加载主配置、NSE/tag 规则、解析目标与端口、应用扫描档位覆盖、构造预检参数，并把同一组值再次复制到 `app.ScanOptions`。Web 还叠加项目默认值与排除项，CLI 则负责输出路径、同步执行和可选 HTML 报告。两者真正不同的是入口协议和启动方式，而不是扫描准备规则。

当前 `internal/preflight` 为复用 `ToolPaths` 和 `ToolExtraArgs` 反向导入 `internal/app`。如果共享准备用例归属 `app`，这条依赖会形成环，因此需要用现有低层包承接最小的工具值类型，而不是新建通用层。

## Goals / Non-Goals

**Goals:**

- 让 CLI 和 Web 通过一个应用层边界完成扫描请求标准化、规则加载、预检和 `ScanOptions` 构造。
- 保留入口特有的参数采集、默认路径、项目读取、启动方式、错误呈现和后处理。
- 用已有包和简单值结构消除反向依赖，确保依赖方向由入口流向应用层和基础能力包。
- 以特征测试固定当前行为，再迁移调用方。

**Non-Goals:**

- 不把 CLI 与 Web 合并成同一种交付协议，也不让应用层依赖 HTTP、flag 或 `store.Project`。
- 不重写配置、端口、目标或预检实现；单个错误的文本和入口呈现保持不变，复合无效输入允许通过共享边界统一首错顺序。
- 不引入 Builder、Factory、依赖注入容器、通用 Service/Repository 接口或新的 `common` 包。
- 不调整扫描运行时；该职责属于后续 `decompose-scan-runtime` change。

## Decisions

### 1. 在 `internal/app` 建立单一扫描准备函数

新增一个普通请求值和一个普通结果值，由单个函数协调现有 `config`、`target`、`ports` 与 `preflight` 能力。结果包含可直接交给 `RunScan` 或 `Manager.Start` 的 `ScanOptions`，并保留结构化预检结果供入口按原方式展示。

选择 `app` 是因为该逻辑协调多个领域能力并形成可执行用例；把它放进 `config`、`preflight` 或 Web 都会让低层包承担上层编排。备选方案是只抽取若干小 helper，但它仍会在两个入口保留组装顺序和字段复制，不能消除漂移。另一个备选是新建 scan service/builder 包，当前只有一个用例和两个调用方，不值得增加抽象层。

### 2. 入口先处理协议特有策略，再交给共享边界

CLI 继续解析 flag、生成默认 run/report 路径、打开 store、同步执行、生成可选 HTML 并打印结果。Web 继续验证 HTTP 方法和项目必填、读取项目、选择表单与项目默认值、使用 `Manager.Start` 异步启动并重定向。共享边界接收这些已选定的原始值，以及 Web 实际存在的排除项，不感知 flag、HTTP 或项目持久化模型。

这避免为了消除少量入口差异而制造“万能请求”。只有配置加载、规范化、排除规则、预检和执行选项组装进入共享边界。

### 3. 保留两类失败通道

配置/规则读取失败以及目标、端口、档位解析失败继续作为普通 `error` 返回；预检警告和错误继续保留为 `preflight.Result`。CLI 仍记录预检详情并以 `preflight failed` 结束，Web 仍以 400 重绘项目扫描表单。共享函数不写 HTTP 响应、不打印日志，也不启动扫描。

这种分离保留现有交付层语义，并避免把结构化预检信息压扁成字符串错误。

### 4. 复用现有配置工具值类型解除反向依赖

`internal/config` 已有与运行时额外参数同构的 `ToolArgs`，工具路径则是 `Config.Tools` 的匿名结构。将该匿名结构命名为 `config.ToolPaths`，复用 `config.ToolArgs`，并在 `app` 保留 `ToolPaths` / `ToolExtraArgs` 类型别名。`preflight` 直接依赖 `config`，从而解除 `preflight -> app` 的反向依赖。

该方案比新增 `tools` 类型少一套映射，也避免 `config` 反向依赖执行层。不得复制等价结构或新增只服务本次重构的包。

### 5. 先用共享契约测试固定行为，再迁移两个入口

测试以相同的配置和原始扫描输入调用共享边界，比较目标、端口、工具、Profile、Workers、额外参数和规则等标准化字段，并覆盖 Web 项目排除项及预检错误。CLI/Web 现有测试继续固定各自的帮助、输出、HTTP 状态和跳转行为。

不会为每个一行映射函数创建测试；测试集中保护会发生漂移的边界和用户可见契约。

## Risks / Trade-offs

- [风险] 统一准备顺序会改变复合无效输入的首个错误优先级。→ delta spec 明确允许统一顺序，并用测试固定单项错误文本与入口协议。
- [风险] Web 项目默认值和排除逻辑被过度泛化。→ 项目读取与默认值优先级留在 Web，仅把实际扫描规范化规则放入共享边界。
- [风险] 工具类型移动扩大编译影响。→ 复用现有 `config` 类型并保留必要类型别名，不顺带重构单工具运行。
- [取舍] CLI 与 Web 仍保留少量相似的入口代码。→ 这些代码表达真实协议差异；消除它们需要更大的抽象且收益不足。

## Migration Plan

1. 用测试记录现有 CLI/Web 对等输入、项目排除和预检失败行为。
2. 建立共享准备边界及最小依赖调整，先由测试直接验证。
3. 迁移 CLI，再迁移 Web；每一步均保持旧入口测试通过。
4. 删除已无调用方的重复组装和排除 helper，不保留兼容包装层之外的双路径。

本次是仓库内部重构，无数据迁移或部署开关；回滚方式是整体回退该 change，不维护新旧两套运行路径。

## Resolved Questions

- 工具值类型采用 `config.ToolPaths` 和现有 `config.ToolArgs`，`app` 保留类型别名；不新增 `internal/tools` 等价类型。

```

## openspec/changes/unify-scan-use-case/tasks.md

- Source: openspec/changes/unify-scan-use-case/tasks.md
- Lines: 1-21
- SHA256: 027cdd9cab1fbe11e9db344893e9edc1cefbe6fa3cdaaf5e533376b2f1840fe0

```md
## 1. 固定现有扫描准备契约

- [ ] 1.1 为共享准备边界补充失败优先的测试，覆盖配置默认值、显式覆盖、目标/端口解析、项目排除、规则加载和预检诊断。
- [ ] 1.2 补充 CLI 与 Web 特征测试，固定当前必填校验、错误信息、预检呈现、HTTP 状态和扫描未启动条件。

## 2. 建立最小共享边界

- [ ] 2.1 解除 `internal/preflight` 对 `internal/app` 工具值类型的反向依赖，并用现有低层包或类型别名保持改动最小。
- [ ] 2.2 在 `internal/app` 实现单一扫描准备函数，复用现有 config、target、ports 和 preflight 逻辑生成结构化结果与 `ScanOptions`。

## 3. 迁移交付入口

- [ ] 3.1 迁移 CLI `scan` 命令调用共享准备边界，保留 flag、默认路径、日志、同步执行、HTML 后处理和标准输出行为。
- [ ] 3.2 迁移 Web 扫描创建调用共享准备边界，保留项目必填、默认值优先级、表单重绘、Manager 启动和重定向行为。
- [ ] 3.3 删除已无调用方的重复组装与排除 helper，确认不存在第二条扫描准备路径。

## 4. 兼容性验证

- [ ] 4.1 运行 `go test ./...`，并比较 CLI/Web 对等输入生成的标准化扫描计划。
- [ ] 4.2 运行现有扫描端到端或冒烟检查和 `make package`，确认 CLI、Web、数据库、报告、工具顺序、事件、取消及产物契约均未变化。
- [ ] 4.3 确认 `go.mod`/`go.sum` 和前端依赖未变化，且现有用户 UI 设计文件未被修改。

```

## openspec/changes/unify-scan-use-case/specs/shared-scan-preparation/spec.md

- Source: openspec/changes/unify-scan-use-case/specs/shared-scan-preparation/spec.md
- Lines: 1-53
- SHA256: dae2aac357fe33829e1c0f9e6d30d14ff44784e15bd112fb55f719f1e0ac2e64

```md
## ADDED Requirements

### Requirement: 扫描入口共享同一准备边界
系统 MUST 让 CLI 与 Web 的完整扫描入口通过同一个应用层准备边界完成配置与规则加载、目标和端口规范化、扫描档位解析、预检以及执行选项构造，不得在两个入口分别维护等价流程。

#### Scenario: 对等输入生成一致扫描计划
- **WHEN** CLI 与 Web 使用相同配置，并提交语义相同的目标、端口、Profile、Workers 和工具额外参数
- **THEN** 两个入口生成的目标、端口、工具路径、Profile、Workers、额外参数、NSE 规则和 tag 规则 MUST 一致，入口特有的 run ID、项目 ID 和输出路径除外

#### Scenario: 配置默认值一致生效
- **WHEN** 某入口未覆盖端口、Profile、Workers 或工具额外参数
- **THEN** 共享准备边界 MUST 使用同一份配置解析规则补全对应扫描计划字段

#### Scenario: 复合无效输入采用统一首错顺序
- **WHEN** 一个扫描请求同时包含多个准备阶段错误
- **THEN** 共享准备边界 MUST 按同一固定顺序返回首个普通错误，且每个单项错误的消息以及 CLI 与 Web 的呈现协议 MUST 保持不变

### Requirement: 项目扫描规则在共享准备中保持语义
系统 MUST 在 Web 入口选择好表单值与项目默认值之后，通过共享准备边界应用项目目标和端口排除规则；CLI 未提供排除项时 MUST 保持现有行为。

#### Scenario: Web 项目默认值和排除项
- **WHEN** Web 扫描使用项目默认目标或端口，并配置了目标或端口排除项
- **THEN** 最终扫描计划 MUST 与当前先选择默认值、再解析并应用排除项的结果一致

#### Scenario: 端口排除保持当前预检与执行字段
- **WHEN** 项目端口排除规则改变了最终执行端口
- **THEN** 预检摘要 MUST 继续使用排除前的已选端口串，而执行选项 MUST 使用解析并排除后的最终端口串

#### Scenario: CLI 不应用项目规则
- **WHEN** CLI 发起普通扫描且没有项目排除输入
- **THEN** 共享准备边界 MUST 保留全部已解析目标和端口

### Requirement: 预检诊断保持结构化且阻止无效扫描
系统 MUST 从共享准备边界返回完整的预检摘要、警告和错误；存在预检错误时 CLI 与 Web 均不得启动扫描，并 MUST 按各自现有协议呈现诊断。

#### Scenario: CLI 预检失败
- **WHEN** CLI 扫描准备产生一个或多个预检错误
- **THEN** CLI MUST 继续输出现有预检日志并返回现有 `preflight failed` 失败语义，且不得调用扫描运行时

#### Scenario: Web 预检失败
- **WHEN** Web 扫描准备产生一个或多个预检错误
- **THEN** Web MUST 返回当前使用的 HTTP 400，并用完整预检结果重绘项目扫描表单，且不得启动 Manager run

### Requirement: 共享准备重构严格保持外部契约
系统 MUST 保持现有 CLI 参数、帮助与输出，Web 路由、状态码、表单与模板数据，数据库 schema，JSON/HTML 报告格式，以及扫描工具顺序、并发、取消、事件和产物行为。

#### Scenario: 成功扫描经共享准备后执行
- **WHEN** 有效的 CLI 或 Web 请求通过共享准备并进入现有扫描运行时
- **THEN** 相同请求产生的工具参数、运行阶段、事件、持久化记录和报告内容 MUST 与重构前一致

#### Scenario: 不新增运行依赖
- **WHEN** 该 change 完成
- **THEN** 项目生产依赖列表 MUST 保持不变，且实现不得要求新的服务、框架或构建步骤

```
