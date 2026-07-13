# Comet Design Handoff

- Change: modularize-web-presentation
- Phase: design
- Mode: compact
- Context hash: 12b7cf1265f3b63b8bbc51ed526a6ef515cf684a1129a79082be18f44ad7b350

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/modularize-web-presentation/proposal.md

- Source: openspec/changes/modularize-web-presentation/proposal.md
- Lines: 1-28
- SHA256: 0e566b83a1a2d165aa626509238491817cfa88f54d005626f34471675aaef473

```md
## Why

Web 页面模板、`app.js`、`style.css`、Node 测试以及静态 HTML 报告的职责集中在少数大文件中，导致彼此无关的界面维护和报告维护容易互相干扰。当前 UI 工作稳定后再进行一次纯结构整理，可以在不改变任何外部行为的前提下缩小后续修改的影响范围。

## What Changes

- 按“修改原因”而不是文件行数整理现有 Web 模板、JavaScript、CSS 与对应 Node 测试；只有职责和验证边界明确时才拆分文件。
- 保留现有 Web 资源入口、DOM 契约和加载方式，确保页面视觉、交互、路由、表单与模板数据行为不变。
- 将 `internal/report/html.go` 中的大型 HTML 字符串迁移为包内嵌模板，复用项目已有的 `embed.FS` 与 Go 标准库模板模式。
- 为结构迁移补充最小回归验证，覆盖静态资源行为和相同输入下的 HTML 报告输出。
- 不引入 React、Vue、打包器、代码生成器或新依赖；不包含任何视觉重设计。
- 本 change 在当前未提交的 UI 工作稳定后最后实施，避免结构迁移与界面设计同时改动相同文件。

## Capabilities

### New Capabilities

- `web-presentation-modularity`: 约束 Web 表现层资源和静态报告模板按独立修改原因组织，并在重组过程中保持现有外部表现与输出兼容。

### Modified Capabilities

- 无。

## Impact

- 主要影响 `internal/web/templates/`、`internal/web/static/app.js`、`internal/web/static/style.css`、相关 Node 测试，以及 `internal/report/html.go` 与新增的包内模板文件。
- `report.WriteHTML` 的函数签名、调用方、Web 路由、HTTP 行为、JSON 格式、HTML 报告内容和现有资源 URL 均不改变。
- 不改变数据库、扫描流程、构建方式或生产依赖；实施前需要先确认当前 UI 工作已经稳定并具备可比较基线。

```

## openspec/changes/modularize-web-presentation/design.md

- Source: openspec/changes/modularize-web-presentation/design.md
- Lines: 1-89
- SHA256: 2f5c9ba1ec6d8a5ae3bdab23f5874de08e931505c1a8285dd34a91288fb0c14a

[TRUNCATED]

```md
## Context

当前 Web 表现层已经使用 `embed.FS` 打包 `templates/*.html` 与 `static/*`，页面由 Go `html/template` 渲染；`app.js` 同时承担运行状态刷新、单工具表单、复制、报告分布图、进度步骤和筛选交互，`style.css` 也覆盖全局外壳及多个页面。Node 回归测试通过 `vm` 直接执行 `app.js`，因此脚本拆分必须同时保持浏览器加载顺序和测试执行顺序。

静态报告的另一条路径位于 `internal/report/html.go`：约 700 行 HTML 以 Go 原始字符串存在，`report.WriteHTML` 每次解析该字符串后写入文件。项目已有的 Web 模板嵌入方式足以解决这个职责混合问题，不需要新模板引擎或资源工具链。

本 change 是纯结构迁移，并受两个硬约束：外部表现严格兼容；当前 UI 工作稳定后才能最后实施。不存在数据迁移或对扫描流程的修改。

## Goals / Non-Goals

**Goals:**

- 让模板、JavaScript、CSS、Node 测试和静态报告模板各自具有清晰且可验证的修改原因。
- 保留 `/static/app.js`、`/static/style.css` 等现有入口，以及页面 DOM、视觉、交互和服务端契约。
- 使用现有 Go 标准库嵌入模式从 `html.go` 移出报告正文，并证明相同输入的 HTML 输出逐字节一致。
- 让生产资源仍由 Go 二进制直接提供，不增加构建步骤和运行时文件依赖。

**Non-Goals:**

- 不进行视觉、文案、交互、可访问性结构或信息架构重设计。
- 不修改 Web 路由、HTTP 状态、表单字段、模板数据、JSON 格式、数据库或扫描行为。
- 不引入 React、Vue、ES 模块迁移、打包器、转译器、代码生成器或第三方依赖。
- 不创建组件框架、通用前端基础层或仅有一个使用方的抽象。
- 不按行数设定拆分目标，也不要求所有大文件都必须变成多个文件。

## Decisions

### 1. 保留稳定入口，只抽取职责明确的叶子资源

`/static/app.js` 与 `/static/style.css` 继续作为现有稳定入口。实施时先基于已经稳定的 UI 快照盘点代码块；只有同时满足以下条件才抽取到同级叶子资源：

1. 该代码块对应一个明确页面或行为；
2. 不依赖隐含的跨块局部状态；
3. 可以由独立 Node 测试或页面回归验证；
4. 抽取后无需新增运行时、命名空间框架或构建工具。

页面专用叶子脚本或样式由对应现有模板显式加载，并保持原有执行顺序和 CSS 级联顺序；共享代码仍留在现有入口。生产叶子资源保持在 `internal/web/static/` 的平级目录中，以直接复用当前 `//go:embed static/*` 和静态文件服务。没有清晰边界的代码保持原位。

现有模板已经按页面划分，因此不再建立 partial/component 层次。`base.html` 仅在某段外壳行为确实被所有页面共享且可独立验证时才抽取；否则保留当前结构。Node 测试只按实际抽取的生产职责镜像拆分，`app.test.mjs` 继续作为现有测试命令的入口；测试公共设置仅在至少两个测试文件重复使用时才提取一个最小 helper。

备选方案：

- **整文件保持不变，仅加注释分区**：改动最小，但不能隔离已经明确独立的页面行为，无法达到本 change 的主要目标。
- **全面改为 ES modules 或引入 bundler**：模块边界更强，但会改变加载语义、测试方式和构建链，超出纯结构迁移范围。
- **一次性按固定目录模板切碎所有资源**：文件数量增加但职责未必更清晰，违反按修改原因拆分的约束。

### 2. 报告模板使用包内 `embed.FS`，保持 `WriteHTML` 边界

将 `htmlTemplate` 的正文原样迁移到 `internal/report/templates/report.html`，在 `report` 包中通过 `//go:embed templates/report.html` 声明私有 `embed.FS`。`WriteHTML(path, scanReport)` 保持签名、建文件时机、错误返回和调用方不变，并在函数内使用 `html/template.ParseFS` 与 `ExecuteTemplate` 渲染内嵌模板。

模板正文迁移必须是机械移动：不格式化、不修正文案、不调整空白。迁移前使用固定 `ScanReport` 生成输出并记录 SHA-256 期望值；迁移后的最小 Go 测试对同一输入校验哈希，从而以较小测试数据证明逐字节兼容。模板继续在调用时解析，不增加全局缓存或 `template.Must` 初始化状态，因为当前没有性能问题需要解决。

备选方案：

- **继续保留 Go 原始字符串**：运行最简单，但报告结构与 Go 控制代码仍混在同一文件。
- **运行时从磁盘读取模板**：源码分离，但发布物新增外部文件依赖，破坏单二进制行为。
- **引入第三方模板或资源生成器**：没有现有需求，增加依赖和构建复杂度。
- **包初始化时预解析并缓存模板**：可能减少重复解析，但会改变错误发生时机；没有测量证据时不引入。

### 3. 兼容性以迁移前基线和现有入口测试为准

实施开始前必须确认当前 UI 工作已稳定，且 `internal/web/templates/`、`internal/web/static/`、`internal/report/` 不存在未合并的重叠改动。随后先运行现有 Go、Node 和打包检查，记录代表性页面在固定浏览器与视口下的基线，并在旧实现上生成报告输出哈希。

每次只迁移一个独立职责并立即验证：Node 测试按页面实际加载顺序执行脚本；Go Web 测试验证路由、状态、关键 DOM 标识和模板数据；相同环境的页面截图不得出现预期外差异；报告测试校验固定输入的字节哈希。任何差异先回退当前抽取单元，不把兼容修复与下一单元混在一起。

备选方案是依赖人工浏览或只运行全量测试。前者不可重复，后者难以定位结构迁移引入的顺序或级联差异，因此采用小步基线验证。

## Risks / Trade-offs

- **[当前 UI 工作仍在变化，产生冲突或基线漂移]** → 将“UI 已稳定且相关目录无重叠改动”设为实施前置门槛，本 change 保持最后执行。
- **[经典脚本拆分改变全局函数可见性或执行顺序]** → 不迁移到 ES modules；模板显式按原顺序加载，Node 测试使用同一顺序。
- **[CSS 拆分改变级联结果或加载时机]** → 只机械移动完整规则块，保持选择器、声明和先后顺序，并比较固定环境截图。
- **[报告模板迁移改变首尾换行或模板空白]** → 原样移动正文，在修改前记录固定输出哈希并在修改后校验。
- **[为了“模块化”制造过多文件和测试辅助层]** → 每个抽取必须对应独立修改原因；默认保留原位，公共 helper 至少有两个真实使用方。
- **[多文件静态请求略有增加]** → 仅抽取确有边界的页面资源；本地单用户控制台不为未测量的性能问题引入打包器。

## Migration Plan

1. 等待当前 UI 工作稳定，确认相关目录没有重叠的未完成修改，并跑通现有基线检查。
2. 在旧实现上增加固定 `ScanReport` 输出哈希回归，再原样迁移报告模板到包内 `embed.FS`；验证 `WriteHTML` 调用方和打包产物。

```

Full source: openspec/changes/modularize-web-presentation/design.md

## openspec/changes/modularize-web-presentation/tasks.md

- Source: openspec/changes/modularize-web-presentation/tasks.md
- Lines: 1-26
- SHA256: f5e869666279f16b4b9178acd382a8e9f91eb929365110cfce9e4d2353b4bca1

```md
## 1. 稳定性门槛与迁移基线

- [ ] 1.1 确认当前 UI 工作已经稳定，且 `internal/web/templates/`、`internal/web/static/`、`internal/report/` 没有重叠的未完成改动；条件不满足时停止实施
- [ ] 1.2 在修改前运行 `go test ./...`、`node --test internal/web/static/app.test.mjs` 和 `make package`，确认现有基线通过
- [ ] 1.3 使用固定 `ScanReport` 在旧实现上记录 HTML 输出 SHA-256，并保存代表性 Web 页面在固定浏览器与视口下的视觉及关键 DOM 基线

## 2. 静态报告模板迁移

- [ ] 2.1 添加固定报告输入的字节哈希回归测试，使模板内容或首尾空白变化能够被检测
- [ ] 2.2 将 `htmlTemplate` 正文不做格式化地迁移到 `internal/report/templates/report.html`，并用私有 `embed.FS`、`ParseFS` 和 `ExecuteTemplate` 保持 `WriteHTML` 边界
- [ ] 2.3 验证固定输入的 HTML 哈希不变、无外部模板文件时仍可生成报告，并确认现有 `WriteHTML` 调用方无需修改

## 3. Web 表现资源按职责整理

- [ ] 3.1 基于稳定 UI 快照确认具有独立页面或行为、独立验证边界且无隐含共享状态的抽取清单；不满足条件的代码明确保留原位
- [ ] 3.2 将确认独立的 JavaScript 行为抽取为平级叶子资源，保留 `/static/app.js` 入口，并由对应模板按原执行顺序加载
- [ ] 3.3 让 Node 测试布局镜像实际 JavaScript 职责，保留 `app.test.mjs` 测试入口，并仅在至少两个测试文件复用时提取最小测试 helper
- [ ] 3.4 仅将与确认职责对应的完整 CSS 规则块抽取为平级叶子资源，保留 `/static/style.css` 入口以及原选择器、声明和级联顺序
- [ ] 3.5 更新相关现有模板的叶子资源引用，并验证路由、表单字段、模板数据、静态资源入口和关键 DOM 契约未改变

## 4. 兼容性验证

- [ ] 4.1 运行 `go test ./...`，确认报告、Web Handler、模板渲染及其他 Go 行为全部通过
- [ ] 4.2 运行 `node --test internal/web/static/app.test.mjs` 和 `make package`，确认脚本加载顺序、交互回归与单二进制打包通过
- [ ] 4.3 在固定浏览器与视口下执行相关 Web 冒烟流程并比较基线，确认页面视觉、DOM、交互、路由、HTTP 状态以及 JSON/HTML 输出没有预期外差异
- [ ] 4.4 审查最终依赖和文件边界，确认未增加框架、打包器或第三方依赖，且每个新增文件都对应一个真实修改原因

```

## openspec/changes/modularize-web-presentation/specs/web-presentation-modularity/spec.md

- Source: openspec/changes/modularize-web-presentation/specs/web-presentation-modularity/spec.md
- Lines: 1-34
- SHA256: ed37cf0ea07bd84bd3681f9a8370822e940bb26a6768b5e4f99c63204df54006

```md
## ADDED Requirements

### Requirement: 表现层源码按独立修改原因组织
系统 MUST 仅在模板、JavaScript、CSS 或 Node 测试存在可独立描述和验证的修改原因时拆分对应文件，并 MUST 让测试边界与被拆分的职责一致；系统 MUST NOT 仅因文件行数拆分源码。

#### Scenario: 整理具有独立职责的前端行为
- **WHEN** 一个前端行为可以独立于其他页面行为进行修改和验证
- **THEN** 该行为及其最小必要测试被组织到职责明确的资源中，修改它不要求同步编辑无关职责的资源

#### Scenario: 保留没有独立边界的代码
- **WHEN** 一段模板、样式或脚本没有独立的修改原因或验证边界
- **THEN** 系统保留其现有归属，不为缩短文件而创建额外层次或文件

### Requirement: Web 表现契约保持兼容
表现层整理后，系统 MUST 保持现有 Web 路由、HTTP 状态、表单字段、模板数据、静态资源入口、DOM 标识与数据属性、视觉结果和用户交互语义不变，并 MUST 继续通过现有浏览器行为对应的 Node 测试。

#### Scenario: 访问现有 Web 页面
- **WHEN** 用户以与变更前相同的请求访问任一现有 Web 页面
- **THEN** 系统返回相同语义的页面结构和视觉结果，原有控件、表单与交互继续工作

#### Scenario: 加载现有静态资源入口
- **WHEN** 页面或测试加载现有 JavaScript 与 CSS 资源入口
- **THEN** 入口仍可直接使用且无需新增打包、转译或运行时依赖

### Requirement: 静态 HTML 报告使用内嵌模板
系统 MUST 使用 Go 标准库 `embed.FS` 将静态 HTML 报告模板编译进二进制，并 MUST 保持 `report.WriteHTML` 的调用契约以及相同 `ScanReport` 输入生成的 HTML 字节不变；JSON 报告不受此变更影响。

#### Scenario: 在没有外部模板文件的环境生成报告
- **WHEN** 已构建的 AnchorScan 二进制在运行目录中不存在报告模板文件时调用 `report.WriteHTML`
- **THEN** 系统仍可使用编译进二进制的模板生成完整 HTML 报告

#### Scenario: 比较迁移前后的报告
- **WHEN** 使用同一份固定 `ScanReport` 数据分别通过迁移前后的实现生成 HTML
- **THEN** 两份 HTML 输出逐字节相同，且现有调用方不需要修改调用参数或处理方式

```
