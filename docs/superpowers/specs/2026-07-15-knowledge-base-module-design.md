---
comet_change: add-knowledge-base-module
role: technical-design
canonical_spec: openspec
---

# AnchorScan 知识库模块技术设计

## 目标与边界

AnchorScan 从配置指定的 Pentest-Playbook Markdown 手册构建进程内只读 Catalog，为知识库浏览、后续漏洞聚合和检测命令生成提供同一份条目、命令与匹配结果。

本变更只实现配置、加载、校验、索引、搜索、匹配和知识库页面。它不修改数据库或 finding，不实现漏洞聚合、目标替换、命令执行、远程同步、热刷新或手册快照。

手册正文契约固定为三个章节：`漏洞描述`、`验证命令`、`修复建议`。模型和页面均不存在 `Result` 或“结果说明”字段。

## 方案选择

### 采用：原生 Go 只读 Catalog

使用现有 Go 标准库和 `gopkg.in/yaml.v3` 直接解析 Markdown 中的 v1 注释和固定章节。Catalog 在 Web 服务启动时构建一次，保存于 `server`，由页面和后续能力直接查询。

该方案没有额外运行时、临时文件或第二份持久数据，错误可以直接定位到手册行号。

### 不采用：调用 Python 校验器后再次解析

调用外部仓库的 Python 校验器只能判断手册是否有效，仍需在 Go 中再次解析条目；这会产生两套路径、部署依赖和错误语义。

### 不采用：导入 JSON 或 SQLite 快照

中间快照会引入同步、失效和迁移问题，也违反手册作为唯一持久知识源的边界。审计级快照如果未来确有需求，应作为独立变更设计。

## 文件与职责

首版按职责保留少量文件，不建立单实现接口或分层门面：

- `internal/knowledgebase/catalog.go`：公开模型、状态、只读 Catalog、索引、搜索和匹配。
- `internal/knowledgebase/parse.go`：路径解析、Markdown/YAML 解析、章节提取、命令校验和诊断。
- `internal/knowledgebase/catalog_test.go`：搜索、索引、匹配和复制隔离测试。
- `internal/knowledgebase/parse_test.go`：三章节 fixture、四种状态和严格契约测试。
- `internal/web/knowledgebase.go`：`/kb` 与 `/kb/{id}` handler 和页面 view model。
- `internal/web/templates/knowledgebase.html`、`knowledgebase_detail.html`：安全展示纯文本内容和通用命令。

现有文件只做必要接线：

- `internal/config/config.go` 增加 `KnowledgeBase.Path`。
- `internal/web/server.go` 启动时构建 Catalog、保存指针并注册路由。
- `internal/web/config.go` 和 `templates/config.html` 保存手册路径并显示重启提示。
- `templates/base.html` 增加知识库导航。
- `config/default.yaml` 增加默认空路径。

## Catalog 接口

Catalog 使用具体类型，不增加 Service、Repository 或工厂：

```go
type Catalog struct { /* private immutable state */ }

func Load(configPath, configuredPath string) *Catalog
func (c *Catalog) Status() Status
func (c *Catalog) Diagnostics() []Diagnostic
func (c *Catalog) Search(query string) []Entry
func (c *Catalog) Entry(id string) (Entry, bool)
func (c *Catalog) Match(observation Observation) MatchResult
```

`Load` 始终返回 Catalog：空路径返回 `disabled`；路径、版本或全局身份错误返回 `unavailable`；局部条目或可选命令错误返回 `degraded`；全部有效返回 `ready`。因此 Web 装配不需要用异常控制降级。

Catalog 的索引字段私有，查询结果对切片字段做复制。调用方不能改变 Catalog 内部条目或候选顺序。

## 数据模型

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

type Commands struct {
    Nuclei     string
    NmapNSE    string
    Metasploit string
}

type Observation struct {
    Tool   Tool
    ToolID string
    CVEs   []string
    Name   string
}
```

`Severity` 只允许 `critical`、`high`、`medium`、`low`；`Tool` 只允许 `nuclei`、`nse`、`manual-review`、`unknown`。无效枚举不被静默转换。

`Diagnostic` 至少保存 Catalog 状态、条目 ID、工具、行号和原因。页面展示有限摘要，测试直接断言结构化诊断。

## 加载和解析流程

1. `knowledge_base.path` 为空时直接生成 disabled Catalog。
2. 绝对路径直接清理；相对路径以配置文件目录为基准解析为绝对路径。
3. 读取 Markdown 并按行保存原始行号。
4. 校验全文恰好一个 `anchorscan-catalog` 注释，且只包含整数 `version: 1`。
5. 识别带四级风险后缀的 `###` 漏洞标题；标题后允许空白行，下一段必须是 `anchorscan-entry` 注释。
6. 使用 `yaml.v3` Node 和 KnownFields 严格解码元数据，拒绝未知字段、anchor、alias、复杂值、非法 ID 和非规范 CVE。
7. 在当前条目边界内提取且只提取按序的三个 `####` 章节。
8. 在验证命令章节中识别可选的 `Nuclei`、`Nmap NSE`、`MSF` 段；每种工具至多一个代码块。
9. 校验条目并累计诊断，再统一检查重复 ID 和有效条目数量。
10. 仅用有效条目构建不可变索引。

目录声明错误、重复 ID、文件不可读或零有效条目属于全局错误。核心条目错误跳过整条；可选命令错误只清空对应命令，保留描述、修复建议和匹配身份。

## 命令契约

命令校验复用 `config.SplitArgs`，不引入 shell。先拒绝 `<`、`>`、`&`、`;`、`|`、`#`、输出参数、重定向、后台执行、命令替换、未知占位符和示例目标，再校验工具形状：

- Nuclei 恰好为 `nuclei -t <模板> -u <目标>` 五个令牌；目标只能是 `{{url}}` 或 `{{host}}:{{port}}`。
- Nmap 只有一个 `-p {{port}}` 和一个位置目标 `{{host}}`，`--script` 操作数中不得出现目标占位符。
- MSF 只解析一个模块序列并确认占位符位置；完整模块/动作白名单仍由后续检测命令变更执行。

知识库只保存通过基础契约的原始通用命令，不替换目标，也不执行命令。

## 搜索与匹配

搜索面向人：对稳定 ID、名称、别名和 CVE 做大小写无关的包含匹配，按风险、名称、ID 的固定规则排序。空查询返回全部有效条目。

匹配面向程序：按来源工具 ID、CVE、规范化名称/别名依次缩小候选集。高优先级证据命中多个条目时继续使用后续证据；最终唯一才返回 matched，零个返回 unmatched，多个返回 ambiguous。任何路径都不从 finding 输出文本推断身份，也不按插入顺序选第一个。

## Web 装配与页面

`NewServer` 读取配置并调用 `knowledgebase.Load`。配置文件本身无法读取时构建 unavailable Catalog，但仍完成 Store、Manager 和 HTTP 路由装配。

`GET /kb?q=` 展示状态、诊断摘要和稳定排序的搜索结果；`GET /kb/{id}` 展示名称、风险、描述、通用命令和修复建议。无效 ID 返回 404。

页面使用 `html/template` 自动转义，不渲染 Markdown HTML。命令放入纯文本代码块并复用现有复制交互。导航始终显示知识库入口，disabled、unavailable、degraded 都提供可操作提示。

配置表单增加 `knowledge_base_path`。基础表单保存仍调用 `config.SaveWithBackup`，成功后重定向到 `/config?saved=1`；页面提示配置已保存、重启 AnchorScan 后生效。当前 server 中的 Catalog 不变化。

## 测试策略

### 包级测试

- 最小三章节 fixture 加载为 ready。
- 目录版本、重复 ID、零有效条目产生 unavailable。
- 非法核心条目被跳过；非法可选命令只禁用该命令并产生 degraded。
- 标题和元数据之间允许空白行但拒绝其他正文。
- YAML 未知字段、anchor、alias、复杂值和非法枚举被拒绝。
- Search 覆盖包含匹配、去重和稳定排序。
- Match 覆盖工具 ID、CVE、名称优先级、候选缩小、unmatched 和 ambiguous。
- 返回 Entry、Diagnostics、Search 结果后修改切片不会影响 Catalog。

### Web 与配置测试

- 基础配置表单保存并重新加载 `knowledge_base.path`。
- 相对路径按配置文件目录解析。
- 保存后页面显示重启提示，当前 Catalog 不热刷新。
- `/kb`、搜索和详情输出正确；条目内容经过模板转义。
- 四种 Catalog 状态均返回正常页面，不阻断现有扫描和报告 handler。

### 回归与真实手册验证

- 每个任务运行对应包测试，最终运行 `go test ./...`。
- 自动化测试只使用仓库内小 fixture，不读取外部 Pentest-Playbook。
- 最终人工 smoke test 使用 Pentest-Playbook 合并提交 `eb4ee8c` 的手册，并记录文件 blob；手册条目数只作为输出统计，不作为通过阈值。

## 风险控制

- 解析器与外部校验器可能漂移：用真实手册 smoke test 和三章节/命令契约 fixture 捕获，而不在运行时调用外部脚本。
- 手册降级可能隐藏部分条目：知识库页必须显示诊断数量和首批可定位错误。
- 历史 run 使用当前启动 Catalog：这是首版明确语义，不宣称审计级知识快照。
- 大手册搜索首版线性扫描；当前规模足够，只有测量到瓶颈后才增加搜索索引。
