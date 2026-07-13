---
change: modularize-web-presentation
design-doc: docs/superpowers/specs/2026-07-12-modularize-web-presentation-design.md
base-ref: f61b7109ba37a0d85466fbd023c22568723308b1
---

# Web Presentation Modularity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变页面、静态资源入口或报告字节输出的前提下，将静态报告正文移入内嵌模板，并只抽取已证明独立的前端叶子行为。

**Architecture:** 保留 `report.WriteHTML`、`/static/app.js` 和 `/static/style.css` 作为稳定边界。报告使用 Go 标准库 `embed.FS`；前端仍使用按模板顺序加载的经典脚本，不增加模块系统、构建链或依赖。

**Tech Stack:** Go 1.26 标准库（`embed`、`html/template`、`crypto/sha256`）、原生 JavaScript、Node `assert`/`vm`、现有 Go HTTP 测试。

## Global Constraints

- 只有在 `decompose-scan-runtime` 与 `decompose-delivery-adapters` 已验证并归档、UI 已稳定后才能开始；任一条件不满足即停止。
- 不改变 Web 路由、HTTP 状态、表单字段、模板数据、DOM 标识、数据属性、视觉结果或交互语义。
- 保留 `/static/app.js`、`/static/style.css`、`report.WriteHTML(path string, scanReport ScanReport) error` 和单二进制交付。
- 不引入 React、Vue、ES modules、bundler、transpiler、代码生成器、第三方依赖或全局模板缓存。
- 默认不拆；无法证明独立加载、独立测试、无隐含共享状态和视觉等价的 JS/CSS 必须留在原文件。

---

### Task 1: 前置门槛与迁移基线

**Files:**
- Inspect only: `openspec/changes/archive/`
- Inspect only: `internal/report/`
- Inspect only: `internal/web/static/`
- Inspect only: `internal/web/templates/`

**Interfaces:**
- Consumes: 已归档的前两个 change 与稳定 UI 工作树。
- Produces: 可比较的测试、包和固定视口基线；本任务不修改仓库。

- [x] **Step 1: 确认前两个 change 已归档**

Run:

```bash
rtk find openspec/changes/archive -maxdepth 1 -type d | rtk rg '/[0-9]{4}-[0-9]{2}-[0-9]{2}-decompose-(scan-runtime|delivery-adapters)$'
```

Expected: 恰好分别出现一个 `decompose-scan-runtime` 和 `decompose-delivery-adapters` 归档目录；缺少任何一个就停止，不执行后续任务。

- [x] **Step 2: 确认表现层相关路径无未提交改动**

Run:

```bash
rtk git status --short -- internal/report internal/web/static internal/web/templates
```

Expected: 无输出。若有输出，先让变更所有者完成或撤出重叠工作，不覆盖它。

- [x] **Step 3: 记录自动化基线**

Run:

```bash
rtk go test ./...
rtk node --test internal/web/static/app.test.mjs
rtk make package
```

Expected: Go 测试全部通过；Node 进程退出码为 0；`make package` 成功生成当前平台目录和 `.tar.gz`。

- [x] **Step 4: 记录视觉基线**

Run the existing server with a disposable database:

```bash
rtk go run ./cmd/anchorscan web --listen 127.0.0.1:19000 --config config/default.yaml.example --db /tmp/anchorscan-modularize-web.sqlite
```

Expected: 服务监听选定的本地地址。使用含现有 run/report 数据的稳定扫描库，在同一浏览器、1440x960 视口保存 `/runs`、一个现有 `/tools/<name>`、`/runs/<run-id>` 和 `/reports/<run-id>` 的截图；不修改仓库数据。

### Task 2: 锁定静态 HTML 报告字节输出

**Files:**
- Modify: `internal/report/html_test.go`
- Test: `internal/report/html_test.go`

**Interfaces:**
- Consumes: `WriteHTML(path string, scanReport ScanReport) error`。
- Produces: `TestWriteHTMLStableBytes`，固定同一 `ScanReport` 的完整输出 SHA-256。

- [x] **Step 1: 添加固定输入的失败哈希测试**

在 `html_test.go` 复用现有 Tomcat fixture，新增 `crypto/sha256` 与 `fmt` import，并新增：

```go
func TestWriteHTMLStableBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	input := Build(
		[]fingerprint.ServiceFingerprint{{
			IP: "192.168.1.10", Port: 8080, Service: "http", Product: "tomcat",
			IsWeb: true, URL: "http://192.168.1.10:8080",
		}},
		[]Finding{{
			IP: "192.168.1.10", Port: 8080, Source: "nuclei",
			ID: "tomcat-default-login", Severity: "high", Summary: "Tomcat Default Login",
			Target: "http://192.168.1.10:8080",
			Output: `{"matched-at":"http://192.168.1.10:8080"}`,
		}},
	)
	if err := WriteHTML(path, input); err != nil {
		t.Fatalf("WriteHTML returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	const want = "0000000000000000000000000000000000000000000000000000000000000000"
	if got != want {
		t.Fatalf("HTML SHA-256 mismatch: got %s want %s", got, want)
	}
}
```

- [x] **Step 2: 运行测试并捕获旧实现的实际哈希**

Run:

```bash
rtk go test ./internal/report -run TestWriteHTMLStableBytes -count=1 -v
```

Expected: FAIL，消息中的 `got` 是 64 位小写十六进制 SHA-256。全零值是明确的 red-test 哨兵；把 `want` 精确替换为该次失败输出的 `got`。它必须取自前两个 change 归档后的稳定实现，不能沿用设计阶段较早的输出。

- [x] **Step 3: 验证基线测试通过**

Run:

```bash
rtk go test ./internal/report -run 'TestWriteHTML(StableBytes|IncludesFindingSummary)' -count=1 -v
```

Expected: 两个测试均 PASS。

- [x] **Step 4: 提交报告基线**

```bash
rtk git add internal/report/html_test.go
rtk git commit -m "test: lock static report html bytes"
```

### Task 3: 将报告正文机械迁移为内嵌模板

**Files:**
- Create: `internal/report/templates/report.html`
- Modify: `internal/report/html.go`
- Test: `internal/report/html_test.go`

**Interfaces:**
- Consumes: Task 2 固定的 SHA-256。
- Produces: 私有 `reportTemplates embed.FS`；`WriteHTML` 签名、解析时机、建文件时机与错误返回不变。

- [x] **Step 1: 原样移动模板正文**

用 `apply_patch` 将 `const htmlTemplate = ` 反引号之间的全部字节移动到 `internal/report/templates/report.html`。文件第一个字节必须是 `<`，最后字节必须是 `>`；不要运行 HTML formatter，不修正文案、空白、重复 CSS 或换行。

- [x] **Step 2: 用标准库从内嵌文件解析**

将 `internal/report/html.go` 收敛为：

```go
package report

import (
	"embed"
	"html/template"
	"os"
)

//go:embed templates/report.html
var reportTemplates embed.FS

func WriteHTML(path string, scanReport ScanReport) error {
	tpl, err := template.ParseFS(reportTemplates, "templates/report.html")
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return tpl.ExecuteTemplate(file, "report.html", scanReport)
}
```

- [x] **Step 3: 证明迁移逐字节等价**

Run:

```bash
rtk gofmt -w internal/report/html.go internal/report/html_test.go
rtk go test ./internal/report -count=1 -v
```

Expected: `TestWriteHTMLStableBytes` 和所有 report 测试 PASS；不得更新 Task 2 的哈希。

- [x] **Step 4: 验证无运行时模板依赖**

Run:

```bash
rtk make package
rtk find dist -type f -name anchorscan -maxdepth 3
```

Expected: 打包成功，包目录中只有二进制而没有 `templates/report.html`；该二进制仍能经现有 CLI/report 路径生成 HTML。

- [x] **Step 5: 提交模板迁移**

```bash
rtk git add internal/report/html.go internal/report/html_test.go internal/report/templates/report.html
rtk git commit -m "refactor: embed static report template"
```

### Task 4: 抽取运行状态叶子脚本

**Files:**
- Create: `internal/web/static/run-status.js`
- Modify: `internal/web/static/app.js`
- Modify: `internal/web/static/app.test.mjs`
- Modify: `internal/web/templates/run.html`
- Modify: `internal/web/runs_test.go`

**Interfaces:**
- Produces: 经典脚本全局函数 `formatEventTime(value)`、`refreshEvents()`、`refreshRunStatus()`、`updateStepper(events, runStatus)`；只由 run 页面加载。

- [x] **Step 1: 先让测试表达真实加载顺序**

在 `app.test.mjs` 增加 `runStatusSource = fs.readFileSync(new URL('./run-status.js', import.meta.url), 'utf8')`，并让时间、轮询和 stepper 测试在同一 `vm` context 中先执行 `source`、再执行 `runStatusSource`。在 `runs_test.go` 将 run 页面断言扩展为同时包含且按序出现：

```html
<script src="/static/app.js"></script>
<script src="/static/run-status.js"></script>
```

Run:

```bash
rtk go test ./internal/web -run TestRunPage -count=1
rtk node --test internal/web/static/app.test.mjs
```

Expected: FAIL，因为文件与模板引用尚不存在。

- [x] **Step 2: 机械移动完整运行状态闭包**

移动 `beijingTimeFormatter`、四个接口函数、两个 `setInterval`/立即调用以及 `updateStepper` 到 `run-status.js`；不要重命名、改延迟、改 DOM ID、改日志转义或改执行顺序。`app.js` 不保留兼容代理。`run.html` 在设置 `window.anchorRunID`/`window.anchorRunStatus` 后依次加载 `app.js`、`run-status.js`。

- [x] **Step 3: 验证并提交**

```bash
rtk node --test internal/web/static/app.test.mjs
rtk go test ./internal/web -run 'TestRunPage|TestStatic' -count=1
rtk git add internal/web/static/app.js internal/web/static/run-status.js internal/web/static/app.test.mjs internal/web/templates/run.html internal/web/runs_test.go
rtk git commit -m "refactor: isolate run status presentation"
```

Expected: Node 与 Go 定向测试全部通过。

### Task 5: 抽取工具表单与报告分布叶子脚本

**Files:**
- Create: `internal/web/static/tool-form.js`
- Create: `internal/web/static/report-ui.js`
- Modify: `internal/web/static/app.js`
- Modify: `internal/web/static/app.test.mjs`
- Modify: `internal/web/templates/tool_page.html`
- Modify: `internal/web/templates/report.html`
- Modify: `internal/web/tools_test.go`
- Modify: `internal/web/report_handler_test.go`

**Interfaces:**
- Produces: `setupToolForm()`/`pollToolOutput()` 只在工具页加载；`renderVulnDistribution()` 只在报告页加载。
- Consumes: `app.js` 的共享复制、preset 和端口插入行为；不创建命名空间或 helper 文件。

- [x] **Step 1: 添加叶子脚本加载与行为测试**

让 `app.test.mjs` 分别读取 `tool-form.js`、`report-ui.js`。工具上下文断言加载脚本后向 `[data-tool-form]` 注册一次 `submit`，报告上下文继续执行现有两组分布图断言，并按浏览器顺序先执行 `app.js` 后执行对应叶子脚本。`tools_test.go` 断言：工具页为 `app.js` 后 `tool-form.js`；`report_handler_test.go` 断言：报告页为 `app.js` 后 `report-ui.js`。

Run:

```bash
rtk node --test internal/web/static/app.test.mjs
rtk go test ./internal/web -run 'TestTool|TestReportPage' -count=1
```

Expected: FAIL，因为两个叶子文件和模板引用尚不存在。

- [x] **Step 2: 逐个机械移动候选**

先移动 `setupToolForm`、`pollToolOutput` 和唯一的 `setupToolForm()` 调用到 `tool-form.js`，验证后再移动 `renderVulnDistribution` 及其 DOMContentLoaded 调用到 `report-ui.js`。保持经典脚本、函数名、fetch URL、轮询间隔和异常文案不变；原 `app.js` 中其他报告过滤、复制、popover、preset 与共享代码不动。

- [x] **Step 3: 更新模板并验证稳定入口**

`tool_page.html` 依次加载 `app.js`、`tool-form.js`；`report.html` 依次加载 `app.js`、`report-ui.js`，之后仍执行原内联 `toggleFindingDetails`。运行：

```bash
rtk node --test internal/web/static/app.test.mjs
rtk go test ./internal/web -run 'TestTool|TestReportPage|TestRunPageLoadsStatusPolling' -count=1
```

Expected: 全部 PASS；`GET /static/app.js` 仍返回 200，原共享行为测试不变。

- [x] **Step 4: 提交叶子脚本**

```bash
rtk git add internal/web/static/app.js internal/web/static/app.test.mjs internal/web/static/tool-form.js internal/web/static/report-ui.js internal/web/templates/tool_page.html internal/web/templates/report.html internal/web/tools_test.go internal/web/report_handler_test.go
rtk git commit -m "refactor: isolate page-specific web behaviors"
```

### Task 6: CSS 门槛与最终兼容性验证

**Files:**
- Verify unchanged: `internal/web/static/style.css`
- Verify: `internal/web/templates/*.html`
- Verify: `go.mod`
- Verify: `go.sum`

**Interfaces:**
- Produces: 兼容性证据；不新增 CSS 文件。

- [x] **Step 1: 明确不拆当前 CSS**

当前工具、报告和 run 规则与通用 terminal、filter、severity、responsive 规则交错且共同使用，不能完整移动连续规则块而不改变级联。因此本 change 不修改 `style.css`、不新增叶子 CSS；这是“默认不拆”门槛的预期结果。

Run:

```bash
rtk git diff f61b7109ba37a0d85466fbd023c22568723308b1 -- internal/web/static/style.css
```

Expected: 无输出。若前两个 change 已合理修改该文件，则改为与 Task 1 记录的实施起点提交比较，结果仍必须无输出。

- [x] **Step 2: 运行完整自动化验证**

```bash
rtk go test ./...
rtk node --test internal/web/static/app.test.mjs
rtk make test
rtk make package
```

Expected: 所有命令退出码为 0，包仍由 Go 二进制直接包含模板与静态资源。

- [x] **Step 3: 比较固定视口页面**

用 Task 1 同一数据库、浏览器和 1440x960 视口重拍相同 URL。Expected: 像素比较无预期外差异；run 状态刷新、工具提交/输出、报告分布/筛选/复制均可操作。出现差异时只回退最后一个叶子抽取，不添加覆盖 CSS 或兼容层。

- [x] **Step 4: 检查边界与依赖**

```bash
rtk git diff --check
rtk git diff --name-only f61b7109ba37a0d85466fbd023c22568723308b1 -- go.mod go.sum internal/web/static/style.css
rtk rg -n 'type="module"|import\(|React|Vue' internal/web/static internal/web/templates
rtk git status --short
```

Expected: `git diff --check` 无错误；`go.mod`、`go.sum`、`style.css` 无本 change 差异；搜索无新增模块/框架用法；状态只包含本计划范围内尚未提交的 Comet/文档状态。

- [ ] **Step 5: 仅在存在验证记录时提交收尾**

若仓库约定保存验证报告，更新现有 Comet verify 产物后提交；否则不创建额外总结文档。不要把 `dist/` 或 `/tmp` 基线加入 Git。
