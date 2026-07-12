# Comet Design Handoff

- Change: decompose-delivery-adapters
- Phase: design
- Mode: compact
- Context hash: d09a6cd1e7f053f9fbb9bf0359c3574392e21590f6d406bd0378e41623c777b6

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/decompose-delivery-adapters/proposal.md

- Source: openspec/changes/decompose-delivery-adapters/proposal.md
- Lines: 1-30
- SHA256: b7a5b34b12e5a0d6c71563c1937074a35a0052cbcdfd2d1bf4339a5103b665ba

```md
## Why（为什么）

CLI 与 Web 交付层目前把命令分派、HTTP 路由、业务输入适配、报告查询和视图数据组装集中在少数大文件中，导致任一职责变化都扩大阅读、测试和回归范围。完成共享扫描用例与扫描运行时拆分后，应按真实修改原因重组交付适配器，使后续改动能落在明确边界内，同时严格保持现有外部契约。

## What Changes（变更内容）

- 按现有 CLI 命令拆分 `cmd/anchorscan/main.go`，保留一个薄入口负责启动和分派，命令实现各自持有参数解析、校验和输出适配职责。
- 按 Web 资源和职责拆分 `internal/web/server.go`，集中保留服务器装配与路由注册，把项目、扫描、工具、报告等处理逻辑放到对应适配器文件。
- 将报告筛选、分页、导出和视图数据组装拆成边界清晰的现有 `internal/web` 组件，避免 Handler 同时承担查询规则和展示转换。
- 按相同职责重组测试，使每组测试与其交付适配器相邻且可独立运行。
- 保持 CLI 参数、帮助文本、输出和退出行为，以及 Web 路由、状态码、表单字段、模板数据和 DOM 行为不变。
- 保持数据库 Schema、Migration、已有数据、JSON/HTML 报告格式和扫描行为不变。
- 该变更依赖 `unify-scan-use-case` 与 `decompose-scan-runtime` 先完成，以已有共享用例和运行时边界为拆分基础。
- 不新增路由框架、Service/Repository 抽象或第三方依赖；不按文件行数机械拆分。

## Capabilities（能力）

### New Capabilities（新增能力）

- `delivery-adapter-modularity`：规定 CLI、Web 与报告交付适配器的职责边界、兼容约束和可独立验证要求。

### Modified Capabilities（修改能力）

- 无。

## Impact（影响）

- 主要影响 `cmd/anchorscan`、`internal/web` 及其测试文件；必要时仅调整这些适配器引用既有应用层入口的方式。
- 不改变公开 CLI/HTTP 契约、数据库与报告格式，也不改变扫描流程、并发、取消、事件或产物。
- 不增加生产或开发依赖；继续使用 Go 标准库、现有模板和现有测试工具。

```

## openspec/changes/decompose-delivery-adapters/design.md

- Source: openspec/changes/decompose-delivery-adapters/design.md
- Lines: 1-104
- SHA256: 2afa7f368a207b5345100828cb6d3667101a40339b9825205501d678dac3c4b2

[TRUNCATED]

```md
## Context（背景）

当前 CLI 入口同时承担进程启动、命令分派、参数解析、输入校验、应用调用和输出适配；Web 服务器文件同时承担服务器装配、路由注册、多个资源的 Handler 与部分辅助逻辑；报告交付路径又把筛选、分页、导出和模板视图数据组装交织在一起。这些代码大多仍属于正确的包，问题是包内修改原因没有形成清晰文件边界。

本变更只重组交付适配器。它排在 `unify-scan-use-case` 和 `decompose-scan-runtime` 之后：CLI 与 Web 应调用前序变更建立的共享扫描用例和稳定运行时边界，而不是再次复制扫描准备或下沉运行逻辑。

约束是严格兼容：CLI、HTTP、模板数据、DOM、数据库、报告和扫描行为不得变化；不得新增依赖、路由框架或只有单一实现的抽象。

```text
CLI main ──> 命令适配器 ──┐
                         ├──> 共享应用用例 ──> 扫描运行时 / Store / Report
HTTP server ─> 资源 Handler ┘
                         └──> 筛选 / 分页 / 导出 / 视图数据
```

## Goals / Non-Goals（目标与非目标）

**Goals（目标）：**

- 让 CLI 入口、各命令适配器、Web 装配层、资源 Handler 和报告交付职责各自只有清晰的修改原因。
- 保持现有包边界和调用方向，复用前序 Change 的共享应用入口。
- 让测试结构镜像职责边界，并用兼容性测试约束移动过程。
- 以最少文件完成重组：只有职责确实独立变化时才拆文件。

**Non-Goals（非目标）：**

- 不新增或修改用户功能、CLI/HTTP API、页面视觉与交互。
- 不调整扫描顺序、工具参数、并发、取消、事件或产物。
- 不修改数据库 Schema、Migration、数据或 JSON/HTML 报告格式。
- 不创建新的通用 `delivery`、`service`、`repository`、`command` 框架或接口层。
- 不引入 React、Vue、前端构建器、路由库或其他依赖。
- 不以文件行数、函数数量或测试数量作为拆分标准。

## Decisions（设计决策）

### 1. 保持现有 Go package，仅按职责拆文件

CLI 继续位于 `cmd/anchorscan` 的 `main` package，Web 与报告交付逻辑继续位于 `internal/web`。重组主要移动现有未导出函数、方法和测试，不新增跨包 API。

**理由：** 当前问题是包内职责集中，并非包职责错误。同包拆分没有新的依赖方向、构造协议或导出面，改动和回滚都最小。

**未采用：** 新建 CLI/Web 框架包或为每个 Handler/Command 建接口。它们只有一个实现，会增加导航和装配成本而不提供现实收益。

### 2. CLI 使用薄入口加命令职责文件

`main.go` 只保留进程入口、顶层命令选择和必要的全局装配。每个有独立参数与输出契约的命令持有自己的 `FlagSet`、解析、校验、应用层调用和结果输出；仅在确有相同变更原因时合并小命令。共享扫描准备必须来自 `unify-scan-use-case`，不能在命令文件中复制。

**理由：** 命令是现有 CLI 最稳定的用户契约和测试边界，同 package 的函数分派已经足够，无需 Command 接口或注册表。

**未采用：** 按“解析器、校验器、输出器”等技术步骤横向拆文件。一次命令变更通常会跨越这些步骤，横向拆分反而增加跳转。

### 3. Web 保留单一服务器装配点，Handler 按资源归属

服务器装配和现有路由注册保留一个事实源；项目、扫描、工具、报告等 Handler 按资源职责移动到同 package 文件。Handler 负责 HTTP 边界工作：解析请求、调用既有应用/存储能力、选择状态码或重定向并构造响应；扫描准备和运行规则不得回流到 Handler。

**理由：** 单一路由表便于审查兼容性，资源文件则缩小具体功能的修改范围。现有 `net/http` 能完整覆盖需求。

**未采用：** 每个 Handler 自注册、反射注册或第三方路由框架。这会隐藏路由顺序和契约，并增加无必要依赖。

### 4. 报告按筛选、分页、导出、视图数据四种修改原因分离

在 `internal/web` 内让筛选条件规范化、分页计算、导出响应和模板视图数据组装成为可分别测试的职责。报告 Handler 只协调这些职责；Store 查询和 `internal/report` 的格式生成仍由现有所有者负责。若两个很小的辅助函数始终共同变化，可留在同一职责文件中。

**理由：** 这四类逻辑受不同输入和兼容契约驱动，分开后可以针对边界做特征测试，同时避免创建新的领域层。

**未采用：** 通用查询 DSL、分页框架或 Presenter/ViewModel 接口。当前只有本地 SQLite 与一套页面/导出路径，现有具体类型更直接。

### 5. 先锁定行为，再做机械移动

实施按职责逐组进行：先补足或确认特征测试，再移动代码并立即运行该职责测试；每组保持可编译。测试文件按命令、资源和报告职责重组，但公共契约测试保留跨边界覆盖。只在移动后为修复编译所需时调整可见性或命名，不顺带改变行为。

**理由：** 这是无数据迁移的内部重构，小步移动和现有测试是最低风险手段，也能在失败时直接定位到最近职责。

**未采用：** 一次性重写 CLI/Web 或同时更换测试框架。两者会把结构变化与行为变化混在同一差异中。

### 6. 与相邻 Change 保持单向依赖

本变更实施前必须确认 `unify-scan-use-case` 和 `decompose-scan-runtime` 已落地；本变更只消费它们的稳定边界。模板、CSS、JavaScript 和静态报告资源的组织留给后续 `modularize-web-presentation`，本变更只保证现有模板数据与 DOM 契约不变。

**理由：** 顺序执行避免多个 Change 同时移动同一责任，也避免交付层在缺少共享入口时制造临时抽象。

```

Full source: openspec/changes/decompose-delivery-adapters/design.md

## openspec/changes/decompose-delivery-adapters/tasks.md

- Source: openspec/changes/decompose-delivery-adapters/tasks.md
- Lines: 1-33
- SHA256: a026014aa4d6ebf7c0b8a81f2d581da78fc775d7154d83756d9eb01cea539ec6

```md
## 1. 前置边界与兼容基线

- [ ] 1.1 确认 `unify-scan-use-case` 与 `decompose-scan-runtime` 已完成，记录 CLI/Web 实际调用的共享用例和运行时入口；若边界尚未稳定则暂停本 Change。
- [ ] 1.2 运行现有 Go、Web 静态资源和打包测试，保存重组前的通过基线，并确认 `go.mod`、`go.sum`、数据库和报告格式不在变更范围内。
- [ ] 1.3 补足 CLI 帮助、成功与错误输出，Web 路由/状态码/表单/模板数据，以及报告筛选/分页/导出的最小特征测试，使后续移动产生漂移时测试会失败。

## 2. 按命令重组 CLI 适配器

- [ ] 2.1 将 `cmd/anchorscan/main.go` 收敛为进程启动、顶层命令分派和必要装配，并保持现有分派顺序与默认行为。
- [ ] 2.2 按现有命令的独立修改原因移动参数解析、校验、应用调用和输出适配；小型且共同变化的命令可同文件，不新增 Command 接口或注册框架。
- [ ] 2.3 移动扫描命令时保持其只调用前序 Change 提供的共享扫描用例，不重新引入 CLI 私有的扫描准备逻辑。
- [ ] 2.4 按命令职责重组 CLI 测试，并验证参数、帮助、标准输出、标准错误和退出行为与基线一致。

## 3. 按资源重组 Web 适配器

- [ ] 3.1 保留单一服务器装配与路由注册事实源，将非装配、非路由职责从 `internal/web/server.go` 移出且不改变注册顺序。
- [ ] 3.2 按项目、扫描、工具、报告等现有资源归属移动 Handler 及其私有 HTTP 辅助逻辑，保持所有代码位于 `internal/web` 且不新增路由或 Service/Repository 抽象。
- [ ] 3.3 移动扫描 Handler 时保持其只处理 HTTP 输入输出并调用共享扫描用例，确认扫描准备与运行规则没有回流到 Web 层。
- [ ] 3.4 按资源职责重组 Web Handler 测试，并逐资源验证路由、方法、状态码、重定向、表单字段、模板数据和响应正文不变。

## 4. 拆分报告交付职责

- [ ] 4.1 将报告筛选条件规范化和分页计算整理为独立职责，以现有边界输入测试锁定默认值、排序、总数和页码行为。
- [ ] 4.2 将报告导出响应和模板视图数据组装整理为独立职责，继续复用现有 Store 查询与 `internal/report` 格式生成能力。
- [ ] 4.3 收敛报告 Handler 为请求解析、职责协调和响应返回，并验证 JSON/HTML 内容、错误行为、数据库查询语义和 DOM 所需数据不变。
- [ ] 4.4 按筛选、分页、导出和视图数据职责重组报告测试，保留所有既有断言和跨职责兼容场景。

## 5. 全量验证与范围审计

- [ ] 5.1 对所有移动后的 Go 文件运行格式化与职责审查，确认文件按修改原因组织、没有行数阈值、单实现接口、通用框架或新增依赖。
- [ ] 5.2 运行 `go test ./...` 与 `node --test internal/web/static/app.test.mjs`，修复任何职责重组造成的兼容回归。
- [ ] 5.3 运行 `make package` 及项目现有相关端到端/冒烟验证，确认 CLI、HTTP、扫描事件与产物、数据库和报告输出保持等价。
- [ ] 5.4 检查最终差异只包含本 Change 的交付适配器与测试重组，不包含模板/CSS/JavaScript 视觉改造或其他相邻 Change 的工作。

```

## openspec/changes/decompose-delivery-adapters/specs/delivery-adapter-modularity/spec.md

- Source: openspec/changes/decompose-delivery-adapters/specs/delivery-adapter-modularity/spec.md
- Lines: 1-48
- SHA256: 35f0f0d54f9283350fd8dc131b8709dc51eb714b4a59c52ca8c9bfb9cd396baf

```md
## ADDED Requirements

### Requirement: CLI 适配器按命令职责组织
系统 SHALL 保留一个仅负责进程启动与命令分派的 CLI 入口，并 SHALL 将各现有命令的参数解析、输入校验、应用层调用和输出适配放入对应命令职责中。文件边界 MUST 依据修改原因确定，不得以行数作为拆分条件。

#### Scenario: 修改单个命令适配器
- **WHEN** 维护者调整某个现有 CLI 命令的参数适配或输出转换
- **THEN** 该改动可定位到该命令职责，且无需修改其他命令的私有适配逻辑

#### Scenario: CLI 外部契约保持不变
- **WHEN** 用户以相同参数执行重构前后同一 CLI 命令
- **THEN** 参数名称、默认值、帮助文本、标准输出、标准错误和退出行为保持一致

### Requirement: Web 适配器按资源和职责组织
系统 SHALL 让服务器装配层负责依赖装配与现有路由注册，并 SHALL 将项目、扫描、工具和报告等 HTTP 处理逻辑放入对应资源职责。重组 MUST 继续使用现有标准库路由方式，不得引入新路由框架或 Service/Repository 抽象。

#### Scenario: 修改单个 Web 资源
- **WHEN** 维护者调整某类资源的请求解析或响应适配
- **THEN** 该改动可定位到对应资源职责，且服务器装配层无需吸收该资源的处理细节

#### Scenario: HTTP 契约保持不变
- **WHEN** 客户端向重构前后相同路由提交相同方法、查询参数和表单字段
- **THEN** 路由匹配、HTTP 状态码、重定向、响应正文和模板数据保持一致

### Requirement: 报告交付职责彼此分离
系统 SHALL 在现有 `internal/web` 边界内分别表达报告筛选、分页、导出和视图数据组装职责，Handler SHALL 只协调这些既有职责并返回响应。拆分 MUST 保持数据库查询语义、排序、页码边界、导出内容和报告格式不变。

#### Scenario: 浏览筛选后的报告列表
- **WHEN** 用户以相同筛选条件、排序和页码浏览报告
- **THEN** 重构前后的结果集合、顺序、总数、分页边界和视图数据相同

#### Scenario: 导出报告
- **WHEN** 用户以相同输入导出 JSON 或 HTML 报告
- **THEN** 文件格式、字段、内容语义和错误行为保持一致

### Requirement: 测试结构镜像交付职责
系统 SHALL 按 CLI 命令、Web 资源和报告职责组织对应测试，使每个交付边界可独立验证，同时 MUST 保留跨边界兼容测试覆盖公开契约。

#### Scenario: 验证职责内改动
- **WHEN** 某个交付适配器发生内部重组
- **THEN** 对应职责测试可直接验证该边界，且现有全量 Go、Web 静态资源及端到端测试仍可执行

### Requirement: 重组不改变底层系统行为
交付适配器重组 MUST NOT 改变数据库 Schema、Migration、已有数据、扫描顺序、工具参数、并发、取消、事件、产物或 DOM 行为，并 MUST NOT 增加生产或开发依赖。

#### Scenario: 执行兼容性回归
- **WHEN** 在相同配置和输入下运行重构前后的扫描与交付流程
- **THEN** 数据库状态、扫描事件、扫描产物、报告输出和页面交互保持等价，且依赖清单不新增条目

```
