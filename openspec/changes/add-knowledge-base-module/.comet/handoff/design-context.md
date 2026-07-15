# Comet Design Handoff

- Change: add-knowledge-base-module
- Phase: design
- Mode: compact
- Context hash: 389c54fd69af57ada25d69e9110930136ace81e02d751d84a9fc3eb2cc570cca

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/add-knowledge-base-module/proposal.md

- Source: openspec/changes/add-knowledge-base-module/proposal.md
- Lines: 1-31
- SHA256: 0d68c297f655bf564aa869779016c3fb66df85aee891300bc104af4fa3858c36

```md
## 为什么

Pentest-Playbook 已完成 anchorscan-catalog v1 的基础结构迁移，用于维护漏洞描述、修复建议和通用检测命令；在 AnchorScan 实现前还需要让现有命令通过本文收紧后的工具目标操作数校验。若报告聚合和检测命令分别解析手册，配置、格式校验和匹配规则会重复并逐渐分叉。

新增一个只读知识库 Catalog，让知识库浏览、报告聚合和检测命令共享同一次启动加载及同一套确定性匹配规则。手册继续由独立仓库维护，AnchorScan 不保存第二份内容。

## 变更内容

- 新增具体、只读的 `internal/knowledgebase.Catalog`，服务启动时从配置路径加载一次 anchorscan-catalog v1 手册并构建内存索引。
- 在现有 YAML 配置和前端配置页中增加 `knowledge_base.path`；保存后提示重启生效，不提供热刷新。
- 严格解析目录版本、条目元数据、三个固定正文章节和固定命令段；使用项目已有的 `yaml.v3`，不增加依赖。
- 提供面向人的包含搜索，以及面向扫描结果的确定性匹配；匹配结果显式区分成功、未匹配和歧义，不选择“第一个命中”。
- 新增 `/kb` 列表/搜索页和 `/kb/{id}` 详情页，展示手册内容及保留占位符的通用检测命令。
- 手册未配置、整体不可用或局部异常时可见降级，不阻断扫描和现有技术报告。

## 能力

### 新增能力

- `vulnerability-knowledge-base`：定义本地 v1 手册的配置、启动加载、校验、查询浏览和确定性 finding 匹配行为。

### 修改能力

- 无。报告聚合与检测命令分别由后续 change 接入 Catalog。

## 影响

- 影响配置模型、配置页、Web 服务装配、导航和新增知识库页面。
- 新增 `internal/knowledgebase` 包，但不预设多层门面或单实现接口。
- 不修改数据库和 finding 持久化格式，不远程拉取、不写回、不嵌入完整手册。
- `add-vulnerability-aggregate-report` 和 `add-vulnerability-detection-commands` 必须删除各自的手册解析与匹配设计，统一依赖本能力。

```

## openspec/changes/add-knowledge-base-module/design.md

- Source: openspec/changes/add-knowledge-base-module/design.md
- Lines: 1-195
- SHA256: 2d14342728a87c1a330f501eaff5f0abe580c0eb39e95ab1c936639750ff3bd4

[TRUNCATED]

```md
## 背景

Pentest-Playbook 的手册已经完成 anchorscan-catalog v1 基础结构迁移：文件包含唯一版本声明；每个漏洞使用带四级风险后缀的 `###` 标题、严格 YAML 元数据块、固定的漏洞描述/验证命令/修复建议章节，以及可选的 `Nuclei`、`Nmap NSE`、`MSF` 命令段。真实命令仍须按本文的工具目标操作数契约再校验一次，未通过前不进入检测命令实现。

知识库浏览、按漏洞聚合报告和检测命令都需要消费这些条目。解析、校验和匹配必须只有一个实现；各页面只负责自己的展示或目标转换。

## 目标 / 非目标

**目标：**

- 将手册加载、校验、索引、搜索和确定性匹配收敛到单一只读 Catalog。
- 允许用户像配置工具路径一样，在 YAML 和前端配置页设置手册路径。
- 提供独立于扫描结果的知识库浏览与搜索。
- 用可诊断的状态隔离手册错误，不阻断现有扫描功能。
- 为报告聚合和检测命令提供稳定、无歧义的共享契约。

**非目标：**

- 不修改数据库或 finding 持久化格式。
- 不实现报告聚合、目标替换或命令执行。
- 不监听文件、不热刷新、不按请求重新解析手册。
- 不远程拉取、写回、镜像或嵌入完整手册。
- 不兼容 anchorscan-catalog v1 之前的标题、占位符或命令格式。
- 不使用模糊相似度或 AI 推断漏洞身份。

## 架构

```text
config.yaml ── knowledge_base.path ──┐
                                     v
Pentest-Playbook ── startup load ── Catalog
                                     ├── /kb 浏览与搜索
                                     ├── 聚合报告匹配
                                     └── 检测命令取原始模板
```

外部手册是唯一持久知识源。Catalog 只是当前进程中的不可变投影，不建立数据库、Repository、缓存层或刷新服务。

## 决策

### 1. 复用现有配置文件和配置页

配置新增一个字段：

```yaml
knowledge_base:
  path: ../Pentest-Playbook/handbook/内网渗透测试手册_v2.md
```

相对路径按配置文件所在目录解析，绝对路径直接使用。空值表示未启用。默认配置保留空路径，保证升级兼容。

前端配置页在工具路径旁增加知识库路径输入框，并通过现有配置保存与备份流程写回 YAML。保存成功后明确提示“重启 AnchorScan 后生效”；当前进程中的 Catalog 不变化。原始 YAML 编辑模式继续可用。

`web.NewServer` 启动时读取配置、构建一次 Catalog 并保存具体指针。Catalog 加载失败只改变其状态，不使 Web 服务启动失败。

### 2. 使用单一具体 Catalog，不建立 Service 门面

`internal/knowledgebase` 对外暴露一个不可变 `Catalog`，不为唯一实现增加接口、工厂或预设五文件脚手架。实现按自然职责拆分，首版只保留调用方确实需要的方法：

```go
catalog.Status()
catalog.Diagnostics()
catalog.Search(text) // 空查询返回全部条目
catalog.Entry(id)
catalog.Match(observation)
```

核心数据使用固定字段而不是任意 map：

```go
type Entry struct {
    ID          string
    Name        string
    Severity    Severity
    Aliases     []string
    Match       MatchKeys
    Description string
    Commands    Commands
    Remediation string
}

```

Full source: openspec/changes/add-knowledge-base-module/design.md

## openspec/changes/add-knowledge-base-module/tasks.md

- Source: openspec/changes/add-knowledge-base-module/tasks.md
- Lines: 1-26
- SHA256: 6830d01d7dadda00583d958292724aca36ff5c1aa6d29e59d8689032de723a7d

```md
## 1. 失败优先测试

- [ ] 1.1 用小型合成 fixture 覆盖目录注释、严格元数据 schema、四级风险、漏洞描述/验证命令/修复建议三个固定章节及工具目标操作数契约
- [ ] 1.2 覆盖版本声明错误、重复 ID、非法核心条目、非法可选命令及四种 Catalog 状态
- [ ] 1.3 覆盖包含搜索、稳定排序、有限 Tool 命名空间、工具 ID/CVE/名称优先级以及歧义不得首选
- [ ] 1.4 覆盖配置页保存 `knowledge_base.path`、相对路径解析及“保存后重启生效”提示

## 2. 配置与 Catalog

- [ ] 2.1 在现有 Config、默认配置和前端配置页增加 `knowledge_base.path`，复用保存与备份流程
- [ ] 2.2 使用已有 `yaml.v3` 实现 v1 严格加载、错误分级和可定位诊断
- [ ] 2.3 实现固定 Entry/Commands/MatchKeys 模型及不可变多键索引
- [ ] 2.4 实现 `Status`、`Diagnostics`、`Search`、`Entry` 和基于 Observation 的 `Match`

## 3. Web 装配与知识库页面

- [ ] 3.1 在 `web.NewServer` 启动时从配置构建一次 Catalog，并保持加载失败不阻断服务
- [ ] 3.2 注册 `/kb` 和 `/kb/{id}`，实现列表、包含搜索和详情处理
- [ ] 3.3 增加知识库模板与导航入口，复用现有模板转义和复制交互
- [ ] 3.4 为 disabled/unavailable/degraded 状态展示对应提示和诊断摘要

## 4. 验证

- [ ] 4.1 运行配置、Catalog、handler、模板及完整 Go 测试
- [ ] 4.2 确认未配置手册时所有现有扫描与报告流程不变
- [ ] 4.3 在进入检测命令实现前，使用当前 Pentest-Playbook 手册验证全部条目和命令均满足 v1；记录已验证 commit/blob，但不把外部仓库纳入自动化测试依赖

```

## openspec/changes/add-knowledge-base-module/specs/vulnerability-knowledge-base/spec.md

- Source: openspec/changes/add-knowledge-base-module/specs/vulnerability-knowledge-base/spec.md
- Lines: 1-93
- SHA256: c1b984d975241c689ac32fb24ccf1a3dc6777ad9a95c1b716fcd2ac73ef90976

[TRUNCATED]

```md
## ADDED Requirements

### Requirement: 用户可配置知识库手册路径
系统 MUST 在现有 YAML 配置和前端配置页中提供 `knowledge_base.path`，并 MUST 在服务启动时按该路径构建一次只读 Catalog。

#### Scenario: 通过前端保存手册路径
- **WHEN** 用户在配置页填写知识库手册路径并保存
- **THEN** 系统通过现有配置保存与备份流程写回该路径，并提示必须重启 AnchorScan 才会加载新手册

#### Scenario: 使用相对路径
- **WHEN** `knowledge_base.path` 是相对路径
- **THEN** 系统以配置文件所在目录为基准解析该路径

#### Scenario: 路径未配置
- **WHEN** 服务启动时知识库路径为空
- **THEN** Catalog 状态为 `disabled`，服务继续启动且现有扫描与报告功能不受影响

#### Scenario: 保存配置但尚未重启
- **WHEN** 用户在当前进程中保存了不同的手册路径
- **THEN** 当前 Catalog 保持不变，页面明确提示重启生效而不执行热刷新

### Requirement: 系统严格加载 anchorscan-catalog v1
系统 MUST 从配置指定的本地 Markdown 文件构建内存投影，并 MUST NOT 将完整手册持久化到数据库、镜像到项目、写回源文件、远程同步或嵌入发布二进制。

#### Scenario: 手册整体有效
- **WHEN** 文件具有唯一受支持的目录版本且所有条目符合 v1
- **THEN** 系统加载固定字段、构建多键索引并将 Catalog 标记为 `ready`

#### Scenario: 目录声明无效
- **WHEN** 多行 `anchorscan-catalog` 注释缺失、重复、包含非整数版本或版本不是 1
- **THEN** 系统将整个 Catalog 标记为 `unavailable`，不尝试按旧格式解析

#### Scenario: 元数据使用非 v1 YAML
- **WHEN** 条目元数据缺少固定字段、包含未知字段、非字符串列表、YAML anchor、alias 或复杂对象
- **THEN** 系统将该核心条目标记为无效，而不是宽松猜测其含义

#### Scenario: 漏洞条目结构无效
- **WHEN** 标题不符合四级风险格式、元数据未紧邻标题，或者三个必填 `####` 章节缺失、重复或顺序错误
- **THEN** 系统跳过该条目并将 Catalog 标记为 `degraded`

#### Scenario: 全局契约无效
- **WHEN** 文件不可读或者稳定 ID 重复
- **THEN** Catalog 标记为 `unavailable` 并保留诊断，服务仍继续启动

#### Scenario: 没有有效漏洞条目
- **WHEN** 文件的目录声明有效但没有任何可用漏洞条目
- **THEN** Catalog 标记为 `unavailable`，而不是提供一个表面可用的空目录

#### Scenario: 核心条目局部无效
- **WHEN** 某条目的元数据、四级风险或必填正文不符合 v1
- **THEN** 系统跳过该条目、将 Catalog 标记为 `degraded`，其他有效条目继续可用

#### Scenario: 可选命令无效
- **WHEN** 某工具命令使用旧占位符、输出参数、复杂 shell 结构或其他非 v1 格式
- **THEN** 系统只禁用该工具命令、记录条目与工具诊断并将 Catalog 标记为 `degraded`，漏洞描述和修复建议仍可查询和匹配

#### Scenario: 命令目标操作数不能确定转换
- **WHEN** Nuclei、Nmap NSE 或 MSF 命令未按 v1 位置绑定所需的 host、port 或完整 URL
- **THEN** 系统将该工具命令标记为无效，不保留固定示例目标或猜测目标位置

### Requirement: 系统提供独立知识库浏览与搜索
系统 MUST 提供不依赖扫描结果的知识库列表、搜索和详情页面，并 MUST 安全展示手册中的纯文本内容。

#### Scenario: 使用关键词搜索
- **WHEN** 用户输入漏洞名、别名、稳定 ID 或 CVE 的完整或部分文本
- **THEN** 系统执行大小写无关的规范化包含搜索，返回去重且稳定排序的全部命中条目

#### Scenario: 查看漏洞详情
- **WHEN** 用户打开一个有效条目
- **THEN** 页面展示漏洞名、四级风险、描述、保留 v1 占位符的通用检测命令和修复建议

#### Scenario: Catalog 降级
- **WHEN** 用户在 Catalog 为 disabled、unavailable 或 degraded 时访问知识库
- **THEN** 页面显示与状态对应的配置或诊断提示，而不是错误页或无说明空白

### Requirement: 系统确定性匹配扫描观察值
系统 MUST 使用与 finding 类型解耦且 Tool 命名空间受限的 Observation，依次按工具 ID、CVE、规范化名称或别名进行精确匹配，并 MUST NOT 从自由文本 Output 推断漏洞身份。

#### Scenario: 唯一工具标识命中
- **WHEN** Observation 的来源和工具 ID 唯一对应一个条目

```

Full source: openspec/changes/add-knowledge-base-module/specs/vulnerability-knowledge-base/spec.md
