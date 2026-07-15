## 背景

Pentest-Playbook 的手册已经完成 anchorscan-catalog v1 基础结构迁移：文件包含唯一版本声明；每个漏洞使用带四级风险后缀的 `###` 标题、严格 YAML 元数据块、固定的漏洞描述/验证命令/结果说明/修复建议章节，以及可选的 `Nuclei`、`Nmap NSE`、`MSF` 命令段。真实命令仍须按本文的工具目标操作数契约再校验一次，未通过前不进入检测命令实现。

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
    Result      string
    Remediation string
}

type Commands struct {
    Nuclei     string
    NmapNSE    string
    Metasploit string
}

type MatchKeys struct {
    Nuclei       []string `yaml:"nuclei"`
    NSE          []string `yaml:"nse"`
    ManualReview []string `yaml:"manual-review"`
    CVE          []string `yaml:"cve"`
}
```

严重性只允许 `critical`、`high`、`medium`、`low` 四种内部值。标题中的 `（严重）/（高危）/（中危）/（低危）` 在加载时转换，风险后缀只用于展示，不参与名称匹配。

### 3. 严格解析全局契约，局部隔离条目错误

解析器只识别 v1 约定的漏洞标题、元数据注释、固定 `####` 章节和固定 `#####` 工具标题；章节标题等其他 Markdown 内容直接忽略。元数据使用项目已有的 `gopkg.in/yaml.v3` 严格解码，并拒绝未知字段、YAML anchor、alias 和复杂对象。

v1 的实现契约固定为：

- 全文恰好一个多行 `<!-- anchorscan-catalog` 注释，且唯一字段为整数 `version: 1`。
- 漏洞标题严格匹配 `^### .+（严重|高危|中危|低危）$`；信息类内容不得成为漏洞条目。
- 标题后立即跟一个 `<!-- anchorscan-entry` YAML 注释，不允许插入其他正文。
- `id` 必填、全局唯一并匹配 `^[a-z0-9]+(?:-[a-z0-9]+)*$`；`aliases` 必填且为字符串列表，可为空。
- `match` 必填且只包含 `nuclei`、`nse`、`manual-review`、`cve` 四个字符串列表；CVE 使用大写规范形式。
- 每个条目恰好按顺序包含 `漏洞描述`、`验证命令`、`结果说明`、`修复建议` 四个 `####` 章节。
- `验证命令` 下只允许按顺序出现可选的 `Nuclei`、`Nmap NSE`、`MSF`；每种工具至多一个代码块，代码块只表达一条逻辑命令或一个 MSF 模块序列。

错误分级如下：

| 条件 | 处理 |
|---|---|
| 路径为空 | Catalog 为 `disabled` |
| 文件不可读、版本声明缺失/重复、版本不支持 | Catalog 为 `unavailable` |
| 稳定 ID 重复 | Catalog 为 `unavailable`，避免报告身份不稳定 |
| 没有任何有效条目 | Catalog 为 `unavailable` |
| 核心元数据、标题风险或必填正文无效 | 跳过该条目，Catalog 为 `degraded` |
| 可选命令段无效 | 仅丢弃该工具命令并记录诊断；条目仍可用于报告，Catalog 为 `degraded` |
| 所有条目有效 | Catalog 为 `ready` |

占位符只允许 `{{host}}`、`{{port}}`、`{{url}}`，并使用以下可确定转换的目标形态：

- Nuclei 必须有且只有一个单目标 `-u` 操作数：HTTP/HTTPS 检测使用完整的 `{{url}}`；非 HTTP 网络检测使用 `{{host}}:{{port}}`。目标占位符不得出现在其他参数中。
- Nmap NSE 必须有且只有一个位置目标 `{{host}}`，并通过 `-p {{port}}` 绑定实际 finding 端口。
- MSF 必须只有一个模块序列，使用 `set RHOSTS {{host}}`；模块具有端口选项时必须使用 `set RPORT {{port}}`。安全模块与动作白名单由检测命令能力继续验证。

输出参数、旧占位符、残留示例目标和复杂 shell 控制结构不属于 v1；发现时该命令不可用，不自动迁移或清理。

诊断至少包含状态、条目 ID（若已知）、工具类型（若适用）、手册行号和简短原因，供配置页、知识库页、检测命令页及测试使用，不把解析日志当作唯一反馈。

### 4. 人工搜索与漏洞匹配使用不同语义

`Search` 面向知识浏览，对名称、别名、稳定 ID 和 CVE 做大小写无关的规范化包含搜索，去重后稳定排序。它可以返回多个条目，不影响报告匹配的严格性。

`Match` 接收不依赖 `report.Finding` 的中性观察值：

```go
type Observation struct {
    Tool   Tool
    ToolID string
    CVEs   []string
    Name   string
}
```

`Tool` 是有限集合 `nuclei`、`nse`、`manual-review`、`unknown`。调用方负责通过一个共享适配器把具体 finding 转成 Observation；未知来源只能继续使用 CVE 或名称证据。只从约定的标识字段提取工具 ID 或严格 CVE token，不读取或解析自由文本 `Output`。

匹配按以下证据优先级进行：

1. 来源对应的精确工具 ID（Nuclei、NSE、manual-review）；
2. 精确 CVE；
3. 规范化后的名称或别名。

高优先级键若命中多个条目，可用后续证据缩小候选集。只有最终候选唯一时返回 `matched`；无候选返回 `unmatched`；多个候选返回 `ambiguous`。索引保存全部候选，任何阶段都不返回“第一个命中”。

### 5. 知识库页面只做安全展示

新增：

```text
GET /kb        列表与搜索，q 为空时列出全部
GET /kb/{id}   条目详情
```

列表展示名称、四级风险、CVE 和主要匹配标识；详情展示描述、通用检测命令、结果说明和修复建议。命令保留 v1 占位符，目标替换属于检测命令 change。

页面使用现有模板转义和复制交互，不增加 Markdown 渲染器或前端框架。`disabled`、`unavailable`、`degraded` 分别显示可操作提示；主导航入口始终可见。

### 6. 后续能力只依赖 Catalog

```text
add-knowledge-base-module
      ├── add-vulnerability-aggregate-report
      └── add-vulnerability-detection-commands
```

聚合报告通过 `Match` 获取规范条目；检测命令通过匹配结果和固定 `Commands` 字段获取模板。两者均不得重新读取或解析手册，也不互相依赖内部视图模型。

## 风险 / 取舍

- 手册更新需要重启才能生效；这是启动快照的明确语义，不增加缓存失效和并发复杂度。
- 严格匹配会暴露缺少别名或重复匹配键；应修正手册元数据，不启用相似度猜测。
- 历史扫描页面使用当前进程启动时的 Catalog；首版不保存知识快照。需要审计级可复现导出时再单独设计 catalog hash 或快照。
- 自动化测试只使用小型合成 v1 fixture，不复制完整外部手册；真实手册仅作人工兼容性 smoke test。

## 迁移计划

配置默认禁用，无数据库迁移。实现顺序为知识库、聚合报告、检测命令。只支持 v1，不保留旧解析路径。

## 待解决问题

无。
