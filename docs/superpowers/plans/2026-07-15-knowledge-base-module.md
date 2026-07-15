---
change: add-knowledge-base-module
design-doc: docs/superpowers/specs/2026-07-15-knowledge-base-module-design.md
base-ref: 264e77b8432e5cda5adbdaca87732a240294ae95
---

# 知识库模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从本地 Pentest-Playbook 手册建立启动时加载的只读知识库，并提供配置、浏览、搜索和确定性匹配。

**Architecture:** `internal/knowledgebase` 是唯一的 Markdown/YAML 解析、校验和索引入口；它始终返回带状态与诊断的具体 Catalog，不让手册错误阻断 Web 服务。Web 只在启动时装配 Catalog，并以 `/kb` 页面安全展示纯文本；当前变更不触及 finding、数据库、报告聚合或命令目标替换。

**Tech Stack:** Go、标准库、既有 `gopkg.in/yaml.v3`、`html/template`、`net/http`、Go testing。

## Global Constraints

- 只支持 `anchorscan-catalog` v1，正文固定为“漏洞描述 / 验证命令 / 修复建议”三节；不引入 Result 或“结果说明”。
- 不新增依赖、数据库、外部进程、远程同步、热刷新、镜像或手册快照。
- `knowledge_base.path` 为空为 `disabled`；任何手册错误都不可阻断既有扫描、报告或 Web 启动。
- 相对手册路径以配置文件目录解析；Catalog 只在 `NewServer` 启动时加载一次。
- 命令仅保存通过 v1 基础契约的原始模板，不执行、不替换目标；解析 token 时复用 `config.SplitArgs`。
- 自动测试只使用仓库内 fixture；真实手册 smoke test 仅在收尾人工运行，记录外部 commit/blob 和统计，不把条目数作为阈值。

---

### Task 1: 建立只读 Catalog 模型、索引与匹配

**Files:**
- Create: `internal/knowledgebase/catalog.go`
- Create: `internal/knowledgebase/catalog_test.go`

**Interfaces:**
- Produces: `Catalog`、`Entry`、`Commands`、`Observation`、`MatchResult`、`Status`、`Diagnostic`。
- Consumes later: `Load(configPath, configuredPath string) *Catalog`（Task 2 实现）、`Search`、`Entry`、`Match`（Task 4 调用）。

- [x] **Step 1: 写出索引、排序、复制隔离与匹配的失败测试**

```go
func TestCatalogSearchAndCopies(t *testing.T) {
    c := catalogFromEntries([]Entry{{ID: "smb-signing", Name: "SMB 签名未启用", Severity: Medium, Aliases: []string{"SMB signing"}, Match: MatchKeys{CVEs: []string{"CVE-2024-0001"}}}})
    got := c.Search("sign")
    got[0].Aliases[0] = "changed"
    again, ok := c.Entry("smb-signing")
    if !ok || again.Aliases[0] != "SMB signing" { t.Fatal("Catalog leaked mutable slice") }
}

func TestCatalogMatchNeverChoosesAmbiguousCandidate(t *testing.T) {
    c := catalogFromEntries([]Entry{{ID: "a", Match: MatchKeys{ToolIDs: []string{"shared"}}}, {ID: "b", Match: MatchKeys{ToolIDs: []string{"shared"}}}})
    got := c.Match(Observation{Tool: ToolNuclei, ToolID: "shared"})
    if got.Status != MatchAmbiguous || len(got.Candidates) != 2 { t.Fatalf("got %#v", got) }
}
```

- [x] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/knowledgebase -run 'TestCatalog(SearchAndCopies|MatchNeverChoosesAmbiguousCandidate)' -count=1`

Expected: FAIL，因为 package、模型或 `catalogFromEntries` 尚不存在。

- [x] **Step 3: 实现最小不可变模型与精确匹配**

```go
type Entry struct { ID, Name string; Severity Severity; Aliases []string; Match MatchKeys; Description string; Commands Commands; Remediation string }
type Observation struct { Tool Tool; ToolID string; CVEs []string; Name string }

func (c *Catalog) Match(o Observation) MatchResult {
    candidates := c.candidates(o.Tool, o.ToolID, o.CVEs, o.Name)
    switch len(candidates) { case 0: return MatchResult{Status: MatchUnmatched}; case 1: return MatchResult{Status: MatchMatched, Entry: copyEntry(candidates[0])}; default: return MatchResult{Status: MatchAmbiguous, Candidates: copyEntries(candidates)} }
}
```

实现固定 severity/tool 枚举、诊断、私有多键索引、`Status`/`Diagnostics`/`Search`/`Entry`/`Match`。搜索以 ID、名称、别名、CVE 的小写包含匹配，按风险、名称、ID 稳定排序；工具 ID、CVE、名称/别名按优先级逐步缩小候选，绝不从自由文本推断，也绝不取第一个。

- [x] **Step 4: 运行包测试**

Run: `go test ./internal/knowledgebase -count=1`

Expected: PASS。

- [x] **Step 5: 提交模型层**

Run: `git add internal/knowledgebase/catalog.go internal/knowledgebase/catalog_test.go && git commit -m "feat: add immutable knowledge catalog"`

### Task 2: 严格解析 v1 手册和命令契约

**Files:**
- Create: `internal/knowledgebase/parse.go`
- Create: `internal/knowledgebase/parse_test.go`
- Modify: `internal/knowledgebase/catalog.go`

**Interfaces:**
- Consumes: Task 1 的模型和 `config.SplitArgs`。
- Produces: `Load(configPath, configuredPath string) *Catalog`。

- [ ] **Step 1: 用内联 Markdown fixture 写失败测试**

````go
const validHandbook = "<!-- anchorscan-catalog\nversion: 1\n-->\n\n### SMB 签名未启用（中危）\n\n<!-- anchorscan-entry\nid: smb-signing\naliases: [SMB signing]\nmatch:\n  names: [SMB signing]\n-->\n\n#### 漏洞描述\n\n描述。\n\n#### 验证命令\n\n##### Nuclei\n```bash\nnuclei -t smb.yaml -u {{host}}:{{port}}\n```\n\n#### 修复建议\n\n启用签名。\n"

func TestLoadAcceptsThreeRequiredSections(t *testing.T) { /* 写入 config 旁临时文件；断言 ready、Description 与 Remediation */ }
func TestLoadRejectsVersionAndDuplicateIDs(t *testing.T) { /* 断言 unavailable */ }
func TestLoadDegradesInvalidEntryAndOptionalCommand(t *testing.T) { /* 断言有效条目保留、无效命令清空 */ }
````

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/knowledgebase -run 'TestLoad' -count=1`

Expected: FAIL，`Load` 尚未实现。

- [ ] **Step 3: 实现路径解析和严格解析器**

```go
func Load(configPath, configuredPath string) *Catalog {
    if strings.TrimSpace(configuredPath) == "" { return disabledCatalog() }
    path := configuredPath
    if !filepath.IsAbs(path) { path = filepath.Join(filepath.Dir(configPath), path) }
    source, err := os.ReadFile(filepath.Clean(path))
    if err != nil { return unavailableCatalog(Diagnostic{Reason: err.Error()}) }
    return parseCatalog(path, string(source))
}
```

按行处理唯一目录注释、`### 名称（风险）`、标题后的空白行和紧邻 entry 注释。用 `yaml.v3.Node` 拒绝 unknown field、anchor、alias、复杂节点和不合法枚举；仅提取按序且唯一的三个 `####` 标题。全局错误（不可读、目录、重复 ID、零有效条目）为 unavailable；核心条目错误跳过条目并 degraded；可选命令错误只清空该命令并 degraded。Nuclei、Nmap NSE、MSF 均先用 `config.SplitArgs`，拒绝 shell/输出/未知占位符后校验指定位置操作数。

- [ ] **Step 4: 运行解析与全包测试**

Run: `go test ./internal/knowledgebase -count=1`

Expected: PASS，覆盖 disabled、unavailable、degraded、ready 四种状态。

- [ ] **Step 5: 提交解析层**

Run: `git add internal/knowledgebase && git commit -m "feat: parse knowledge base handbook"`

### Task 3: 接入 YAML 配置和配置页面

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config/default.yaml.example`
- Modify: `internal/web/config.go`
- Modify: `internal/web/config_test.go`
- Modify: `internal/web/templates/config.html`

**Interfaces:**
- Produces: `Config.KnowledgeBase.Path` 和带 `saved=1` 的配置重定向。
- Consumes later: Task 4 的 `NewServer` 使用该路径加载 Catalog。

- [ ] **Step 1: 写配置保存与重启提示失败测试**

```go
form := strings.NewReader("rustscan=&nmap=&httpx=&nuclei=&ports=top1000&profile=normal&knowledge_base_path=../playbook/handbook.md")
req := httptest.NewRequest(http.MethodPost, "/config", form)
// 断言 303 Location 为 /config?saved=1；重新 Load 后 cfg.KnowledgeBase.Path 保持输入。
// GET /config?saved=1 断言含“重启 AnchorScan 后生效”。
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/web -run TestConfigPage -count=1`

Expected: FAIL，因为配置字段、表单字段和提示尚不存在。

- [ ] **Step 3: 最小接入配置**

```go
type Config struct {
    Tools ToolPaths `yaml:"tools"`
    KnowledgeBase struct { Path string `yaml:"path"` } `yaml:"knowledge_base"`
    // 保留既有 Scan 与 Profiles
}
```

在示例 YAML 添加 `knowledge_base: { path: "" }`（多行表示）。基础表单读取/保存 `knowledge_base_path`，成功后重定向 `/config?saved=1`；页面在路径输入框旁说明“重启后生效”，仅当 query `saved=1` 显示成功提示。raw YAML 流程保持原样。

- [ ] **Step 4: 运行配置回归**

Run: `go test ./internal/config ./internal/web -run 'TestConfigPage|Test.*Config' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交配置接入**

Run: `git add internal/config/config.go config/default.yaml.example internal/web/config.go internal/web/config_test.go internal/web/templates/config.html && git commit -m "feat: configure knowledge base path"`

### Task 4: 装配 Catalog 和知识库浏览页面

**Files:**
- Create: `internal/web/knowledgebase.go`
- Create: `internal/web/templates/knowledgebase.html`
- Create: `internal/web/templates/knowledgebase_detail.html`
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`
- Modify: `internal/web/templates/base.html`

**Interfaces:**
- Consumes: `knowledgebase.Load`、`Catalog.Search`、`Catalog.Entry`。
- Produces: `GET /kb?q=`、`GET /kb/{id}`；手册失败时仍可用的 Web 服务。

- [ ] **Step 1: 写 handler 失败测试**

```go
func TestKnowledgeBaseRoutes(t *testing.T) {
    h := newServerWithHandbook(t, validHandbook)
    for _, tc := range []struct{ path, want string }{{"/kb?q=SMB", "SMB 签名未启用"}, {"/kb/smb-signing", "启用签名"}} {
        res := httptest.NewRecorder(); h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, tc.path, nil))
        if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), tc.want) { t.Fatalf("%s: %d %s", tc.path, res.Code, res.Body.String()) }
    }
}
func TestKnowledgeBaseDetailEscapesEntryText(t *testing.T) { /* fixture 名称含 <script>；断言页面不含可执行标签 */ }
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/web -run TestKnowledgeBase -count=1`

Expected: FAIL，因为路由和模板不存在。

- [ ] **Step 3: 在启动时加载一次并实现页面**

```go
cfg, err := config.Load(opts.ConfigPath)
catalog := knowledgebase.Load(opts.ConfigPath, "")
if err == nil { catalog = knowledgebase.Load(opts.ConfigPath, cfg.KnowledgeBase.Path) }
s := &server{opts: opts, store: scanStore, manager: app.NewManager(opts.Runner, scanStore), catalog: catalog}
mux.HandleFunc("/kb/", s.knowledgeBaseDetail)
mux.HandleFunc("/kb", s.knowledgeBaseList)
```

在 handler 限制为 GET；`/kb/{id}` 无条目返回 404。列表展示状态、有限诊断摘要、搜索表单和条目链接；详情展示名称、风险、描述、非空通用命令与修复建议。通过现有 `render` 和 `html/template` 自动转义，命令仅放置在 `<pre><code>`，不渲染 Markdown HTML。导航新增 `/kb` 并在当前路径激活。

- [ ] **Step 4: 验证降级与既有路由**

Run: `go test ./internal/web -run 'Test(KnowledgeBase|ConfigPage|Server)' -count=1`

Expected: PASS；disabled、unavailable、degraded 均返回可解释的正常页面，未配置手册时主页和配置页不受影响。

- [ ] **Step 5: 提交 Web 页面**

Run: `git add internal/web && git commit -m "feat: add knowledge base pages"`

### Task 5: 完整验证与真实手册 smoke test

**Files:**
- Modify: `openspec/changes/add-knowledge-base-module/tasks.md`

- [ ] **Step 1: 执行完整自动化验证**

Run: `go test ./...`

Expected: PASS。

- [ ] **Step 2: 对已合并手册做一次手工 smoke test**

Run: `go test ./internal/knowledgebase -run TestManualHandbookSmoke -count=1 -handbook /Users/kun/DEV/Pentest-Playbook/handbook/内网渗透测试手册_v2.md`

Expected: PASS；若测试项目不接受 flag，则使用一个临时、未提交的 Go 小程序调用 `knowledgebase.Load` 并输出状态、诊断数、条目数。记录 Pentest-Playbook commit `eb4ee8c` 与文件 blob，不断言条目数。

- [ ] **Step 3: 勾选实际完成的 OpenSpec 任务并提交验证记录**

Run: `git add openspec/changes/add-knowledge-base-module/tasks.md && git commit -m "docs: record knowledge base verification"`

## 自检

- Spec 覆盖：Task 1 覆盖 Catalog、搜索、匹配；Task 2 覆盖 v1 解析、状态和命令；Task 3 覆盖路径配置与重启语义；Task 4 覆盖浏览、诊断与安全展示；Task 5 覆盖回归和真实手册验证。
- 无占位任务：每个任务给出文件、依赖接口、失败测试、命令和预期结果。
- 类型一致性：Web 只依赖 `Load`、`Catalog.Search`、`Catalog.Entry`；不越界访问 Catalog 私有索引。
