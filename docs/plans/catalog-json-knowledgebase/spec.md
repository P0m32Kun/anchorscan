# Spec — 知识库 catalog.json 消费链路

**Status:** ready-for-agent

本文定义 AnchorScan 消费 Pentest-Playbook handbook-v3 生成的 `catalog.json` 的设计：在 `internal/knowledgebase/` 新增 JSON 加载路径，与既有 Markdown 解析路径并存，产出完全等价的 `Catalog`。

## 1. 背景与目标

### 1.1 当前死锁

Pentest-Playbook 的 handbook-v3 条目知识库（188 条目，frontmatter + 生成器）已建成，`build_handbook.py --check` 通过与 v2 手册的内容级比对和 anchorscan-catalog v1 契约校验。但 AnchorScan 的知识库加载器 `internal/knowledgebase/parse.go` 用正则解析 v2 手册的 Markdown 格式（`### 标题（severity）` + HTML 注释 YAML 元数据），只认 `<!-- anchorscan-catalog version: 1 -->` 标记的单一 Markdown 文件。

后果：

- v3 的 `dist/catalog.json`（机器消费版产物）无人消费；
- v3 不敢正式替换 v2——切换后 AnchorScan 知识库加载失效；
- AnchorScan 不敢动知识库解析——v2 是唯一输入源，正则解析牵一发动全身。

### 1.2 目标

1. AnchorScan 支持直接消费 `catalog.json`，产出与 Markdown 路径**逐字段等价**的 `Catalog`。
2. 双格式并存：`knowledge_base.path` 按文件扩展名分派，`.json` 走新加载器，其余走既有 Markdown 解析，向后兼容。
3. 提供**平价验收测试**：同一份知识库分别经两条路径加载，断言 `Catalog` 条目集合完全一致。
4. 为后续切换铺路：本工作完成后，操作者可将 `knowledge_base.path` 指向 `catalog.json`；Markdown 路径保留至 v2 手册退役。

### 1.3 非目标

- 不改动 Pentest-Playbook 侧（`catalog.json` 产物已完整，无需变更）。
- 不改动知识库 Web 视图（`/kb` 页面）、报告生成、命令执行的调用方——`Catalog`/`Entry` 公开接口不变。
- 不实现知识库热加载；加载仍发生在 `NewServer` 启动时。
- 不处理 v3 条目中 `漏洞描述`/`修复建议` 之外的扩展知识小节（如 `漏洞原理`）；JSON 加载器忽略它们，与 Markdown 路径行为一致（Markdown 路径的固定三章节契约本就不解析扩展小节）。

## 2. 现状分析

### 2.1 AnchorScan 侧

加载入口（`internal/web/server.go:81-83`）：

```go
catalog := knowledgebase.Load(opts.ConfigPath, "")
if cfg, err := config.Load(opts.ConfigPath); err == nil {
    catalog = knowledgebase.Load(opts.ConfigPath, cfg.KnowledgeBase.Path)
}
```

`Load(configPath, configuredPath)` 解析单个 Markdown 文件；`configuredPath` 为空时返回 `StatusDisabled`。相对路径相对于配置文件目录解析。

`Entry` 模型（`internal/knowledgebase/catalog.go`）：

| 字段 | 类型 | 来源（Markdown 路径） |
|---|---|---|
| `ID` | string | `anchorscan-entry` 元数据 `id` |
| `Name` | string | `###` 标题 |
| `Severity` | enum | 标题中的中文标签 → `parseSeverity` |
| `Aliases` | []string | 元数据 `aliases`（v2 手册实际全部为空） |
| `Match` | MatchKeys | 元数据 `match.nuclei/nse/manual-review/cve` |
| `Description` | string | `#### 漏洞描述` 节 |
| `Commands` | Commands | `##### Nuclei/Nmap NSE/MSF` 代码块，经 `validNuclei/validNmapNSE/validMSF` 白名单校验 |
| `Remediation` | string | `#### 修复建议` 节 |

诊断语义：条目级问题（缺章节、命令非法等）→ `StatusDegraded` + `Diagnostic`，条目视情况保留；文件级问题（缺标记、无有效条目、重复 ID）→ `StatusUnavailable`；正常 → `StatusReady`。

### 2.2 catalog.json 产物结构

`build_handbook.py` 的 `build()` 生成：

```json
{
  "version": 1,
  "source": "handbook-v3",
  "entry_count": 188,
  "entries": [ ... ]
}
```

每条目字段（已在真实产物上核实，188 条目）：

| catalog.json 字段 | 说明 | 覆盖情况 |
|---|---|---|
| `id` | 条目 ID | 全部 |
| `title` | 标题 | 全部 |
| `severity` | `严重/高危/中危/低危` | 全部，四种取值 |
| `order` | 章内排序 | 全部 |
| `status` | 条目状态 | 全部 |
| `tags` | 标签数组 | 全部（多为空） |
| `chapter` / `chapter_name` | 所属章节 | 全部 |
| `match.nuclei/nse/manual-review/cve` | 匹配键 | 全部（129 条有 nuclei 键） |
| `verify` | 验证命令结构 | 147 条；41 条无 verify |
| `sections` | 正文字节映射（不含 `验证命令`） | 全部 |
| `sources` | 来源 | 全部（多为空） |

**注意**：catalog.json 不投影 `aliases`。已核实 v2 手册全部条目 `aliases: []`，v3 schema 也未声明 `aliases`，当前无信息损失。若未来 v3 schema 引入非空 aliases，须同步加入 catalog 投影与本加载器。

`verify` 三种形态（真实产物核实：nuclei 125 / nmap 19 / msf 3）：

```json
{"tool": "nuclei", "template": "network/exposures/exposed-redis.yaml", "target": "host:port"}
{"tool": "nmap", "script": "telnet-brute", "args": "userdb=...,passdb=...", "flags": ["-sV"]}
{"tool": "msf", "module": "auxiliary/scanner/ssh/ssh_enumusers", "action": "run"}
```

### 2.3 命令构造的唯一真相源

`build_handbook.py:render_command` 是 verify → 命令字符串的**唯一构造实现**，白名单规则内建：

- **nuclei**：`nuclei -t <template> -u {{url}}`（`target: url`）或 `nuclei -t <template> -u {{host}}:{{port}}`（`target: host:port`）；template 须匹配 `(RBKD-templates/)?[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*\.ya?ml`。
- **nmap**：`nmap [flags...] -p {{port}} --script <script> [--script-args <args>] {{host}}`；flags 每项匹配 `-[A-Za-z0-9]+`；script/args 禁含 ` <>&;|#`。
- **msf**：四行 `use <module>\nset RHOSTS {{host}}\nset RPORT {{port}}\n<action>`；`run` 仅允许 `auxiliary/scanner/`，`check` 仅允许 `exploit/`；module 匹配 `(auxiliary/scanner/|exploit/)[A-Za-z0-9._/-]+`。

生成的 v3 Markdown 已通过 `--check` 验证与 v2 内容一致、且通过 `validate_anchorscan_v1` 契约校验——即这些构造规则产出的命令与 AnchorScan 现有解析器接受的命令集合完全重合。

## 3. 设计

### 3.1 总体方案

在 `internal/knowledgebase/` 新增 `catalog_json.go`：

```go
// LoadJSON 从 catalog.json 加载知识库。path 语义与 Load 相同。
func LoadJSON(configPath, configuredPath string) *Catalog
```

`Load` 改为按扩展名分派：

```go
func Load(configPath, configuredPath string) *Catalog {
    // ...既有的空路径/路径解析/读文件逻辑...
    if strings.HasSuffix(strings.ToLower(path), ".json") {
        return parseCatalogJSON(string(data))
    }
    return parseCatalog(string(data)) // 既有 Markdown 路径
}
```

调用方（`server.go`）零改动；`knowledge_base.path` 指向 `catalog.json` 即启用新路径。

### 3.2 字段映射

| catalog.json | Entry | 转换规则 |
|---|---|---|
| `id` | `ID` | 直取；空或重复 → 见 §3.4 |
| `title` | `Name` | 直取 |
| `severity` | `Severity` | 复用 `parseSeverity`（中文标签→枚举）；四种以外取值 → 条目级诊断，跳过该条目 |
| `match.nuclei` | `Match.NucleiIDs` | 直取；缺省视为空数组 |
| `match.nse` | `Match.NSEIDs` | 同上 |
| `match.cve` | `Match.CVEs` | 同上 |
| `match.manual-review` | `Match.Names` | 同上 |
| `sections["漏洞描述"]` | `Description` | 直取；缺失或为空 → 条目级诊断，跳过该条目 |
| `sections["修复建议"]` | `Remediation` | 直取；缺失或为空 → 条目级诊断，跳过该条目 |
| `verify` | `Commands` | 按 §3.3 构造；无 verify → 三个命令字段均为空（与 Markdown 路径无命令块的条目等价） |
| —（catalog 无 aliases） | `Aliases` | 置空切片 |
| `chapter/order/status/tags/sources` | — | 忽略（`Entry` 无对应字段；KB 视图未来需要时再扩展） |
| `sections` 其他键 | — | 忽略（见 §1.3 非目标） |

### 3.3 命令构造（Go 侧 `renderCommand`）

Go 实现与 `build_handbook.py:render_command` 逐条对齐的构造与白名单校验。任何一条校验失败 → 条目级诊断（degraded），该条目保留但对应命令字段置空；**绝不**输出未通过白名单的命令字符串。

| tool | 构造 | 校验 |
|---|---|---|
| `nuclei` | `nuclei -t <template> -u {{url}}` 或 `... {{host}}:{{port}}` | template 正则同上；target 仅允许 `url`/`host:port` |
| `nmap` | `nmap [flags] -p {{port}} --script <script> [--script-args <args>] {{host}}` | script/args 禁字符 ` <>&;|#`；flags 每项 `-[A-Za-z0-9]+` |
| `msf` | 四行 use/set/set/action | module 前缀白名单 + action↔模块类型匹配，规则同上 |
| 其他 | — | 未知 tool → 条目级诊断，无命令 |

构造产物的字符串必须与 Markdown 路径命令块逐字节一致（含 msf 的换行符位置），这是平价测试的断言基础。

### 3.4 错误处理与诊断语义

复用现有 `Status`/`Diagnostic` 模型：

| 情形 | 状态 | 说明 |
|---|---|---|
| 文件不存在 / 读失败 | `StatusUnavailable` | 与 Markdown 路径一致 |
| JSON 语法错误、`version != 1`、`entries` 非数组 | `StatusUnavailable` | 文件级诊断 |
| 重复条目 ID | `StatusUnavailable` | 与 Markdown 路径一致 |
| 条目缺 `id`/`title`/severity 非法/缺 `漏洞描述`/缺 `修复建议` | `StatusDegraded` | 条目级诊断，跳过该条目，继续解析其余 |
| 条目 verify 校验失败 | `StatusDegraded` | 条目保留，命令字段置空 |
| 全部条目被跳过（0 有效条目） | `StatusUnavailable` | 与 Markdown 路径"没有有效漏洞条目"一致 |
| 正常（可有 degraded 诊断） | `StatusReady`/`StatusDegraded` | 同现有语义 |

### 3.5 分发方式

发布归档当前不内嵌手册（`Makefile package` 只带 config/docs/docx-render），`knowledge_base.path` 由操作者自行配置——catalog.json 沿用同一模式，操作者将路径指向 Pentest-Playbook 的 `handbook-v3/dist/catalog.json`（或其副本）。本次不涉及发布打包变更；后续若决定归档内嵌知识库，另行立项。

## 4. 验收标准

1. **平价测试**（核心验收）：测试 fixture 使用真实 `catalog.json` 与其对应的真实 v3 生成 Markdown 手册（或从两者提取的对等子集），分别经 `parseCatalogJSON` 与 `parseCatalog` 加载，断言：
   - 两条路径的有效条目集合完全相同（按 ID 对齐）；
   - 每条目的 `ID/Name/Severity/Aliases/Match/Description/Remediation` 逐字段相等；
   - 每条目的 `Commands.Nuclei/NmapNSE/Metasploit` 逐字节相等；
   - 诊断条目的集合一致（Markdown 路径因格式问题降级的条目，JSON 路径同样降级——当前真实产物两条路径均无诊断，此断言退化为"双方均 ready"）。
2. **契约测试**：手工构造的 catalog.json fixture 覆盖 §3.4 每种诊断情形，断言状态码与诊断内容。
3. **分派测试**：`Load` 对 `.json` 扩展名走 JSON 路径、对 `.md` 走 Markdown 路径、对无扩展名文件走 Markdown 路径（向后兼容）。
4. `go test ./internal/knowledgebase/ ./internal/report/ ./internal/web/` 全绿；`make pr-check` 通过。
5. 手工验证：将运行实例的 `knowledge_base.path` 指向真实 `catalog.json`，`/kb` 页面条目列表/详情、报告页漏洞 enrich、候选命令生成与指向 v2 手册时表现一致。

## 5. 分阶段迁移

| 阶段 | 内容 | 完成判据 |
|---|---|---|
| 1（本 spec） | AnchorScan 双格式支持 + 平价测试 | §4 全部通过 |
| 2 | 操作者默认配置指向 catalog.json；更新 `docs/` 中知识库配置说明 | 新部署默认消费 JSON |
| 3（后续，Playbook 侧主导） | v3 正式替换 v2 成为唯一维护源；v2 手册停更 | Playbook 完成切换 |
| 4（远期） | AnchorScan 移除 Markdown 解析路径 | 确认无操作者依赖 v2 格式后另行立项 |

## 6. 风险与对策

| 风险 | 等级 | 对策 |
|---|---|---|
| Go/Python 两处命令构造规则漂移 | 高 | 平价测试直接钉死双路径输出一致；Python 侧 render_command 变更会导致 v3 手册变化、进而打破平价测试，形成双向告警 |
| catalog.json 被手工编辑成非法 verify | 中 | §3.3 白名单校验在 Go 侧兜底，非法命令不进入 Entry |
| v3 schema 演进（新增 aliases、verify 新形态）未同步 | 中 | catalog.json `version` 字段门控；加载器只认 `version: 1`，schema 变更须升版本号并同步本加载器 |
| 41 条无 verify 条目在报告中缺候选命令 | 低 | 与现状一致（v2 中这些条目同样无命令块），非回归 |

## 7. 实施 ticket 拆分（建议）

按 `docs/agents/issue-tracker.md` 契约，实施时创建：

1. `01-json-catalog-loader` — `catalog_json.go`：JSON 解析、字段映射、renderCommand + 白名单、诊断语义；单元测试覆盖 §3.4。
2. `02-load-dispatch` — `Load` 扩展名分派；分派测试。
3. `03-parity-test` — 真实产物平价测试（fixture 取自 Pentest-Playbook dist，测试内嵌或构建期拷贝，不运行时依赖外部仓库）。
4. `04-docs-and-manual-verify` — 更新知识库配置文档；按 §4.5 手工验证并记录结果。

实施顺序即编号顺序；每个 ticket 独立可验证。fixed point 为 `main` 分支 `41580aa`。
