---
change: decompose-delivery-adapters
design-doc: docs/superpowers/specs/2026-07-12-decompose-delivery-adapters-design.md
base-ref: 141de6a359bd5c0b4be0e5d16400c07904784dde
---

# Decompose Delivery Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变 CLI、HTTP、报告、数据库或扫描行为的前提下，把 CLI、Web Handler 和报告交付逻辑按真实职责拆到同 package 的聚焦文件中。

**Architecture:** 这是纯同包机械重组：先用现有特征测试锁定契约，再按一组符号一次移动并运行最窄测试。`cmd/anchorscan/main.go` 保留根分派，`internal/web/server.go` 保留唯一装配与路由表，报告 Handler 只协调过滤、分页、导出和视图纯函数。

**Tech Stack:** Go 1.26、标准库 `flag`/`net/http`/`html/template`、现有 `modernc.org/sqlite`、Go `testing`、Node 内置 test runner、Make。

## Global Constraints

- **硬前置：** 只有 `decompose-scan-runtime` 已验证并归档后才能开始 Task 1；否则停止，不创建临时适配层。
- 从实际开始实施时的 `decompose-scan-runtime` 归档提交创建工作分支；本计划的 `base-ref` 仅用于核对规划基线。
- 保持 `main`、`cmd/anchorscan` 和 `internal/web` 的现有 package，不新增跨包 API。
- 保持 CLI 参数、帮助、stdout、stderr、错误优先级、HTTP 路由顺序、状态码、重定向、表单字段、模板数据和响应正文不变。
- 不修改数据库 Schema/Migration、JSON/HTML 报告格式、扫描顺序、并发、取消、事件或产物。
- 不新增依赖、路由框架、Command/Service/Repository/Presenter/ViewModel 接口或注册表。
- 模板、CSS、JavaScript 保持不动；它们属于后续 `modularize-web-presentation`。
- 每次只移动一组现有符号；除 import、格式化和测试文件归属外，不顺便重命名或清理代码。

---

## Target File Map

### CLI

| 文件 | 最终职责 |
| --- | --- |
| `cmd/anchorscan/main.go` | `cliDeps`、`main`、`Execute`、`run`、根/版本帮助，以及被多个命令共用的 `ensureParentDir`、`isHelpRequest` |
| `cmd/anchorscan/scan_command.go` | `runScan`、`logScan`、`logPreflight`、`printScanHelp` |
| `cmd/anchorscan/tool_command.go` | `runTool`、`applyToolExtraArgs`、`splitCSV`、`printToolHelp` |
| `cmd/anchorscan/report_command.go` | `runReport`、`runImportNmap`、`printReportHelp`、`printImportNmapHelp` |
| `cmd/anchorscan/admin_command.go` | `runDoctor`、`runWeb`、`runCancel`、`runTools`、`checkToolPath` 及对应帮助 |

不创建 `command` 接口或第五个 helper 文件；两个真正跨命令的小 helper 留在根文件即可。

### Web

| 文件 | 移入的现有符号 |
| --- | --- |
| `internal/web/server.go` | `ServerOptions`、`server`、managed path helpers、`NewServer`、完整且顺序不变的路由注册、`ServeHTTP`、`Close`、`render`、`newID` |
| `internal/web/projects.go` | `home`、`projects`、`projectNew`、`projectDetail`、`parseProjectRequest` |
| `internal/web/scans.go` | `renderProjectScanForm`、`scanCreate`、`mergedTargetsInput`、`joinNonEmpty`、`loadProjectForScan`、`defaultProjectProfile`、`coalesce` |
| `internal/web/tools.go` | `toolPageData`、`manualTool`、`toolPreset`、`toolNew`、`toolPage`、`toolCreate`、`isManualTool`、`manualToolByName`、`manualTools`、`applyToolExtraArgs`、`splitCSV` |
| `internal/web/imports.go` | `importNmapForm`、`renderImportForm`、`importNmapRun` |
| `internal/web/runs.go` | `runDetail`、`runAPI`、`runs` |
| `internal/web/config.go` | `configPageData`、`configPage`、`configPorts`、`normalizePortCSV` |
| `internal/web/report_handler.go` | `reportDetail` |

### Report helpers

| 文件 | 移入的现有符号 |
| --- | --- |
| `internal/web/report_filters.go` | `reportFilters`、`supportedSeverities`、filter/match helpers、`containsValue`、`parseSeverityFilters` |
| `internal/web/report_pagination.go` | `reportPageSize`、`reportPage`、`reportPageSizes`、parse/paginate/URL helpers |
| `internal/web/report_exports.go` | `exportAssetsTXT`、`exportAssetsCSV`、`exportFindingsCSV` |
| `internal/web/report_views.go` | `runMetaSummaryLimit`、`hostAssetView`、`runMetaView`、`newRunMetaView`、`summarizeRunValue`、`groupFingerprintsByHost`、`appendUnique` |

测试文件镜像上述职责；共享 `serverSequenceRunner`、`writeFile`、`writeExecutable`、`closeServer` 只保留一份在 `server_test.go`。

---

### Task 1: Verify the predecessor and capture the compatibility baseline

**Files:**
- Inspect: `openspec/changes/archive/*-decompose-scan-runtime/`
- Inspect: `internal/app/scan.go`
- Inspect: `cmd/anchorscan/main.go`
- Inspect: `internal/web/server.go`
- Inspect: `go.mod`
- Inspect: `go.sum`

**Interfaces:**
- Consumes: 已归档 change 中仍由 `app.PrepareScan(PrepareScanRequest) (PreparedScan, error)` 与 `app.RunScan(context.Context, tools.Runner, *store.Store, app.ScanOptions) error` 提供的稳定入口。
- Produces: 一个全绿、可与每次机械移动比较的基线；不产生代码。

- [x] **Step 1: Confirm the predecessor is archived**

Run: `rtk find openspec/changes/archive -maxdepth 1 -type d -name '*-decompose-scan-runtime'`

Expected: 恰好输出一个归档目录。没有输出或存在 active `openspec/changes/decompose-scan-runtime/` 时立即停止并回到前序 change。

- [x] **Step 2: Confirm the shared scan boundary is the one actually consumed**

Run: `rtk codegraph explore "callers of app.PrepareScan and app.RunScan in cmd/anchorscan and internal/web"`

Expected: CLI `runScan` 与 Web `scanCreate` 使用共享准备入口；没有各自复制的 target/port/profile 准备流程。

- [x] **Step 3: Record the green baseline**

Run: `rtk go test ./...`

Expected: PASS，所有 Go package 返回 `ok` 或 `[no test files]`。

Run: `rtk node --test internal/web/static/app.test.mjs`

Expected: PASS，失败数为 0。

Run: `rtk make package`

Expected: exit 0，并在 `dist/` 生成当前平台 tarball。

- [x] **Step 4: Confirm dependency files are untouched**

Run: `rtk git diff --exit-code -- go.mod go.sum`

Expected: exit 0、无输出。

### Task 2: Add the smallest missing characterization checks

**Files:**
- Modify: `cmd/anchorscan/main_test.go`
- Modify: `internal/web/reports_test.go`

**Interfaces:**
- Consumes: 当前 `run`、`parseSeverityFilters`、`paginateFingerprints`、`exportAssetsTXT`、`newRunMetaView`。
- Produces: 根错误输出、过滤规范化、分页查询保持、TXT 输出、Unicode 摘要的稳定契约。

- [x] **Step 1: Add the root dispatch contract test to `main_test.go`**

```go
func TestRunUnknownCommandPreservesStderrAndError(t *testing.T) {
	var stderr bytes.Buffer
	err := run([]string{"missing"}, &bytes.Buffer{}, &stderr, cliDeps{})
	if err == nil || err.Error() != "unknown command" {
		t.Fatalf("error = %v", err)
	}
	if stderr.String() != "unknown command: missing\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
```

- [x] **Step 2: Add pure report characterization tests to `reports_test.go`**

Add imports `net/url`, `strings`, and `github.com/P0m32Kun/anchorscan/internal/store`, then add:

```go
func TestParseSeverityFiltersNormalizesAndDeduplicates(t *testing.T) {
	got := parseSeverityFilters(url.Values{"severity": {"HIGH,unknown", "high", "critical"}})
	want := []string{"high", "critical"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("severities = %#v", got)
	}
}

func TestPaginateFingerprintsClampsPageAndPreservesFilters(t *testing.T) {
	items := make([]fingerprint.ServiceFingerprint, 11)
	page := paginateFingerprints(items, 99, url.Values{"q": {"redis"}}, "assets_page", "assets_size", 10)
	if page.Page != 2 || page.TotalPages != 2 || len(page.Items.([]fingerprint.ServiceFingerprint)) != 1 {
		t.Fatalf("page = %#v", page)
	}
	if page.PrevURL != "?assets_page=1&q=redis" || page.NextURL != "?assets_page=3&q=redis" {
		t.Fatalf("urls = %q %q", page.PrevURL, page.NextURL)
	}
}

func TestExportAssetsTXTKeepsCurrentLineFormat(t *testing.T) {
	got := exportAssetsTXT([]fingerprint.ServiceFingerprint{{IP: "192.0.2.1", Port: 443}}, "ip_port")
	if got != "192.0.2.1:443\n" {
		t.Fatalf("export = %q", got)
	}
}

func TestNewRunMetaViewSummarizesByRune(t *testing.T) {
	value := strings.Repeat("界", runMetaSummaryLimit+1)
	got := newRunMetaView(store.ScanRun{Target: value})
	if got.FullTarget != value || got.Target != strings.Repeat("界", runMetaSummaryLimit)+"..." {
		t.Fatalf("view = %#v", got)
	}
}
```

- [x] **Step 3: Verify the tests pass before moving code**

Run: `rtk go test ./cmd/anchorscan ./internal/web`

Expected: PASS。它们是特征测试，不是新功能测试，因此在重构前就必须通过。

- [x] **Step 4: Commit the compatibility checks**

```bash
rtk git add cmd/anchorscan/main_test.go internal/web/reports_test.go
rtk git commit -m "test: lock delivery adapter contracts"
```

### Task 3: Split the CLI by command responsibility

**Files:**
- Create: `cmd/anchorscan/scan_command.go`
- Create: `cmd/anchorscan/tool_command.go`
- Create: `cmd/anchorscan/report_command.go`
- Create: `cmd/anchorscan/admin_command.go`
- Modify: `cmd/anchorscan/main.go`

**Interfaces:**
- Consumes: `run` 调用的现有未导出函数签名；`cliDeps` 保持 `newRunner func() tools.Runner`、`openStore func(string) (*store.Store, error)`、`now func() time.Time`。
- Produces: `runScan`、`runTool`、`runReport`、`runImportNmap`、`runDoctor`、`runWeb`、`runCancel`、`runTools` 的原签名和行为。

- [x] **Step 1: Move the scan symbols byte-for-byte**

Move `runScan`, `logScan`, `logPreflight`, and `printScanHelp` to `scan_command.go`. Do not reorder statements. The resulting import set is:

```go
import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/report"
)
```

Run: `rtk gofmt -w cmd/anchorscan/main.go cmd/anchorscan/scan_command.go`

Run: `rtk go test ./cmd/anchorscan -run 'TestExecuteScan'`

Expected: PASS。

- [x] **Step 2: Move the tool symbols byte-for-byte**

Move `runTool`, `applyToolExtraArgs`, `splitCSV`, and `printToolHelp` to `tool_command.go`; keep its calls to package-local `logScan` and `ensureParentDir` unchanged.

Run: `rtk gofmt -w cmd/anchorscan/main.go cmd/anchorscan/tool_command.go`

Run: `rtk go test ./cmd/anchorscan -run 'TestExecuteTool'`

Expected: PASS。

- [x] **Step 3: Move report and import symbols byte-for-byte**

Move `runReport`, `runImportNmap`, `printReportHelp`, and `printImportNmapHelp` to `report_command.go`.

Run: `rtk gofmt -w cmd/anchorscan/main.go cmd/anchorscan/report_command.go`

Run: `rtk go test ./cmd/anchorscan -run 'TestExecute(Report|ImportNmap)'`

Expected: PASS。

- [x] **Step 4: Move administration symbols byte-for-byte**

Move `runDoctor`, `runWeb`, `runCancel`, `runTools`, `checkToolPath`, `printDoctorHelp`, `printWebHelp`, `printToolsHelp`, and `printToolsCheckHelp` to `admin_command.go`.

Keep `cliDeps`, `main`, `Execute`, `run`, `ensureParentDir`, `isHelpRequest`, `printVersion`, and `printRootHelp` in `main.go`. Remove only imports that the compiler reports unused.

Run: `rtk gofmt -w cmd/anchorscan/*.go`

Run: `rtk go test ./cmd/anchorscan`

Expected: PASS，包括 root help、unknown command、version、doctor、web、cancel 和 tools checks。

- [x] **Step 5: Commit the production split**

```bash
rtk git add cmd/anchorscan/main.go cmd/anchorscan/scan_command.go cmd/anchorscan/tool_command.go cmd/anchorscan/report_command.go cmd/anchorscan/admin_command.go
rtk git commit -m "refactor: split CLI command adapters"
```

### Task 4: Mirror the CLI split in tests

**Files:**
- Create: `cmd/anchorscan/scan_command_test.go`
- Create: `cmd/anchorscan/tool_command_test.go`
- Create: `cmd/anchorscan/report_command_test.go`
- Create: `cmd/anchorscan/admin_command_test.go`
- Modify: `cmd/anchorscan/main_test.go`

**Interfaces:**
- Consumes: 同 package 测试可直接调用 Task 3 的未导出函数；共享 runner 和文件 helper 不复制。
- Produces: 每个命令组可用 `rtk go test -run` 独立验证。

- [x] **Step 1: Record the exact baseline before editing**

Run:

```bash
git status --short -- cmd/anchorscan
go test ./cmd/anchorscan -list '^Test' | sed '/^ok[[:space:]]/d' | sort > /tmp/anchorscan-tests.before
wc -l /tmp/anchorscan-tests.before
go test ./cmd/anchorscan
```

Expected: `cmd/anchorscan` 无未提交修改；测试清单为 23 行；package PASS。计划文档本身可以尚未提交。若任一条件不满足，停止，不继续移动。

- [x] **Step 2: Move tests by exact name, without an extraction script**

Use `apply_patch` to move complete `func Test...` declarations. Do not use line-number slicing, independent prefix predicates, regex extraction, or a temporary AST splitter. Do not alter any test body or assertion.

Create `scan_command_test.go` with exactly these eight tests:

```text
TestExecuteScanHelpShowsFlags
TestExecuteScanReturnsPortErrorBeforeProfileError
TestExecuteScanDoesNotOpenStoreWhenSharedPreflightFails
TestExecuteScanStoresArtifactDirUnderSelectedRoot
TestExecuteScanPrintsPreflightSummary
TestExecuteScanStopsOnPreflightError
TestExecuteScanPassesProfileAndToolArgs
TestExecuteScanWritesJSONAndHTML
```

Create `tool_command_test.go` with exactly these three tests:

```text
TestExecuteToolHelpShowsTools
TestExecuteToolNucleiRejectsMissingTagsAndTemplate
TestExecuteToolNmapAliveWritesRunOutput
```

Create `report_command_test.go` with exactly these five tests:

```text
TestExecuteReportWritesHTMLFromStoredRun
TestExecuteImportNmapWritesRunAndReports
TestExecuteImportNmapRejectsEmptyXML
TestExecuteImportNmapRejectsNonNmaprun
TestExecuteImportNmapHelpShowsFlags
```

Create `admin_command_test.go` with exactly these four tests:

```text
TestExecuteToolsCheckReportsConfiguredTools
TestExecuteDoctorPrintsChecks
TestExecuteWebHelpShowsListen
TestExecuteCancelPostsToServer
```

`TestExecuteToolsCheckReportsConfiguredTools` belongs only to `admin_command_test.go`. Although its name starts with `TestExecuteTool`, it must never also be classified as a tool-command test.

Use these import blocks after the moves; do not retain imports only needed by a test moved to another file:

```go
// cmd/anchorscan/scan_command_test.go
import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

// cmd/anchorscan/tool_command_test.go
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/tools"
)

// cmd/anchorscan/report_command_test.go
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// cmd/anchorscan/admin_command_test.go
import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)
```

Keep exactly these three tests in `main_test.go`:

```text
TestExecuteRootHelpShowsCommands
TestRunUnknownCommandPreservesStderrAndError
TestVersionCommandPrintsVersion
```

Also keep `sampleFingerprint`, `fakeRunner`, `recordingRunner`, `failRunner`, `writeFile`, and `writeExecutable` in `main_test.go`; package-level declarations remain visible to all five test files.

Its final imports are:

```go
import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/version"
)
```

- [x] **Step 3: Format and compile-check imports**

Run:

```bash
gofmt -w cmd/anchorscan/main_test.go cmd/anchorscan/scan_command_test.go cmd/anchorscan/tool_command_test.go cmd/anchorscan/report_command_test.go cmd/anchorscan/admin_command_test.go
go test ./cmd/anchorscan
```

Expected: the first `go test` may report only unused or missing imports. Adjust imports in the named file, run `gofmt` again, and repeat until PASS. Do not move helpers merely to avoid an import edit.

- [x] **Step 4: Prove that no test was lost or duplicated**

Run:

```bash
go test ./cmd/anchorscan -list '^Test' | sed '/^ok[[:space:]]/d' | sort > /tmp/anchorscan-tests.after
diff -u /tmp/anchorscan-tests.before /tmp/anchorscan-tests.after
test "$(wc -l < /tmp/anchorscan-tests.after | tr -d ' ')" = 23
```

Expected: `diff` has no output and both commands exit 0. This is the hard gate for the split.

Then run each responsibility independently:

```bash
go test ./cmd/anchorscan -run '^TestExecuteScan'
go test ./cmd/anchorscan -run '^TestExecuteTool(Help|Nuclei|Nmap)'
go test ./cmd/anchorscan -run '^TestExecute(Report|ImportNmap)'
go test ./cmd/anchorscan -run '^TestExecute(ToolsCheck|Doctor|Web|Cancel)'
go test ./cmd/anchorscan -run '^(TestExecuteRootHelp|TestRunUnknownCommand|TestVersionCommand)'
go test ./cmd/anchorscan
```

Expected: all six commands PASS. The tool regex deliberately excludes `TestExecuteToolsCheckReportsConfiguredTools`; the admin regex includes it exactly once.

- [x] **Step 5: Review only the mechanical split**

Run:

```bash
git diff --check
git diff --stat
git diff -- cmd/anchorscan/main_test.go cmd/anchorscan/scan_command_test.go cmd/anchorscan/tool_command_test.go cmd/anchorscan/report_command_test.go cmd/anchorscan/admin_command_test.go
```

Expected: only the five test files changed; test bodies and assertions are byte-for-byte equivalent apart from file position and `gofmt`-managed whitespace/imports. No production file changes.

- [x] **Step 6: Commit the test split**

```bash
rtk git add cmd/anchorscan/*_test.go
rtk git commit -m "test: group CLI tests by command"
```

### Task 5: Reduce `server.go` to assembly and move resource handlers

**Files:**
- Create: `internal/web/projects.go`
- Create: `internal/web/scans.go`
- Create: `internal/web/tools.go`
- Create: `internal/web/imports.go`
- Create: `internal/web/runs.go`
- Create: `internal/web/config.go`
- Create: `internal/web/report_handler.go`
- Modify: `internal/web/server.go`

**Interfaces:**
- Consumes: `server` fields `opts ServerOptions`, `store *store.Store`, `manager *app.Manager`, `mux *http.ServeMux`。
- Produces: 原有 Handler 方法名和签名；`NewServer(ServerOptions) (http.Handler, error)` 与路由注册顺序完全不变。

- [x] **Step 1: Freeze the route table in place**

Do not edit the following registration order while moving dependent methods:

```go
mux.Handle("/static/", http.FileServerFS(assets))
mux.HandleFunc("/projects", s.projects)
mux.HandleFunc("/projects/new", s.projectNew)
mux.HandleFunc("/projects/", s.projectDetail)
mux.HandleFunc("/scan", s.scanCreate)
mux.HandleFunc("/tools/new", s.toolNew)
mux.HandleFunc("/tools/", s.toolPage)
mux.HandleFunc("/tools", s.toolCreate)
mux.HandleFunc("/runs", s.runs)
mux.HandleFunc("/runs/", s.runDetail)
mux.HandleFunc("/api/runs/", s.runAPI)
mux.HandleFunc("/reports/", s.reportDetail)
mux.HandleFunc("/config", s.configPage)
mux.HandleFunc("/config/ports", s.configPorts)
mux.HandleFunc("/import/nmap", s.importNmapForm)
mux.HandleFunc("/import/nmap/run", s.importNmapRun)
mux.HandleFunc("/", s.home)
```

- [x] **Step 2: Move project and scan symbols**

Move the exact symbol groups from the Target File Map to `projects.go` and `scans.go`; do not split `projectDetail` internally even though it dispatches subpaths.

Run: `rtk gofmt -w internal/web/server.go internal/web/projects.go internal/web/scans.go`

Run: `rtk go test ./internal/web -run 'Test(Home|CreateProject|NewScan|ScanCreate|DeleteProject)'`

Expected: PASS。

- [x] **Step 3: Move tool and import symbols**

Move the exact tool and import groups from the Target File Map. Keep tool presets as concrete slices/functions; do not introduce a registry.

Run: `rtk gofmt -w internal/web/server.go internal/web/tools.go internal/web/imports.go`

Run: `rtk go test ./internal/web -run 'Test(Tool|ImportNmap|NavIncludesImport)'`

Expected: PASS。

- [x] **Step 4: Move run, config, and report Handler symbols**

Move the exact groups to `runs.go`, `config.go`, and `report_handler.go`. Keep all response branches in `reportDetail` in their current order; pure report helpers move only in Task 7.

Run: `rtk gofmt -w internal/web/*.go`

Run: `rtk go test ./internal/web -run 'Test(Run|Runs|DeleteScanRun|Config|Report)'`

Expected: PASS。

- [x] **Step 5: Verify `server.go` owns only shared assembly**

Run: `rtk rg -n '^func \(s \*server\)' internal/web/server.go`

Expected: 只列出 `ServeHTTP` 和 `Close`；`NewServer`、managed path helpers、`render`、`newID` 仍可作为普通函数存在。

- [x] **Step 6: Commit the Handler split**

```bash
rtk git add internal/web/server.go internal/web/projects.go internal/web/scans.go internal/web/tools.go internal/web/imports.go internal/web/runs.go internal/web/config.go internal/web/report_handler.go
rtk git commit -m "refactor: split web resource handlers"
```

### Task 6: Mirror Web resource ownership in tests

**Files:**
- Create: `internal/web/projects_test.go`
- Create: `internal/web/scans_test.go`
- Create: `internal/web/tools_test.go`
- Create: `internal/web/imports_test.go`
- Create: `internal/web/runs_test.go`
- Create: `internal/web/config_test.go`
- Create: `internal/web/report_handler_test.go`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Consumes: Task 5 Handler methods and the single shared test helper set in `server_test.go`。
- Produces: 每个 Web 资源可独立运行的测试组，断言和场景总数不减少。

- [x] **Step 1: Move existing tests by route ownership**

- `projects_test.go`: home/project creation/detail/deletion tests。
- `scans_test.go`: new scan and every `TestScanCreate*` test。
- `tools_test.go`: every `TestTool*` test。
- `imports_test.go`: every `TestImportNmap*` test。
- `runs_test.go`: runs page、run detail/status/events、scan run deletion tests。
- `config_test.go`: every `TestConfig*` test。
- `report_handler_test.go`: every `TestReport*` test。
- Keep route-wide navigation coverage and shared helpers/types in `server_test.go`; do not duplicate fixtures.

Move test functions byte-for-byte, then remove unused imports file by file.

- [x] **Step 2: Run resource groups independently**

Run: `rtk gofmt -w internal/web/*_test.go`

Run: `rtk go test ./internal/web -run 'Test(Home|CreateProject|DeleteProject|NewScan|ScanCreate)'`

Run: `rtk go test ./internal/web -run 'Test(Tool|ImportNmap|Run|Runs|DeleteScanRun|Config|Report)'`

Run: `rtk go test ./internal/web`

Expected: 全部 PASS；最终整包测试数量与 Task 1 基线一致或仅增加 Task 2 的特征测试。

- [x] **Step 3: Commit the Web test split**

```bash
rtk git add internal/web/*_test.go
rtk git commit -m "test: group web tests by resource"
```

### Task 7: Split report helpers by change reason

**Files:**
- Create: `internal/web/report_filters.go`
- Create: `internal/web/report_pagination.go`
- Create: `internal/web/report_exports.go`
- Create: `internal/web/report_views.go`
- Delete: `internal/web/reports.go`
- Create: `internal/web/report_filters_test.go`
- Create: `internal/web/report_pagination_test.go`
- Create: `internal/web/report_exports_test.go`
- Create: `internal/web/report_views_test.go`
- Delete: `internal/web/reports_test.go`

**Interfaces:**
- Consumes: `reportDetail` 使用的所有现有未导出 concrete types/functions。
- Produces: 完全相同的函数名、参数和返回类型；不引入新的 wrapper、DSL 或接口。

- [x] **Step 1: Move filter symbols and their tests**

Move `reportFilters`, `supportedSeverities`, `filterFingerprints`, `filterFindings`, `containsValue`, `findingMatchesService`, `fingerprintMatchesKeyword`, `findingMatchesKeyword`, and `parseSeverityFilters` to `report_filters.go`.

Move `TestFilter*` and `TestParseSeverityFiltersNormalizesAndDeduplicates` to `report_filters_test.go`.

Run: `rtk gofmt -w internal/web/report_filters*.go`

Run: `rtk go test ./internal/web -run 'Test(Filter|ParseSeverity)'`

Expected: PASS。

- [x] **Step 2: Move pagination symbols and its characterization test**

Move `reportPageSize` from `server.go`, plus `reportPage`, `reportPageSizes`, `parsePage`, `parseSize`, `paginateFingerprints`, `paginateFindings`, `paginateHostAssets`, `paginate`, `cloneValues`, `pageURL`, `pageSizeURLs`, and `withQuery` to `report_pagination.go`.

Move `TestPaginateFingerprintsClampsPageAndPreservesFilters` to `report_pagination_test.go`.

Run: `rtk gofmt -w internal/web/server.go internal/web/report_pagination*.go`

Run: `rtk go test ./internal/web -run 'Test(Paginate|ReportPagePaginates)'`

Expected: PASS。

- [x] **Step 3: Move export symbols and tests**

Move `exportAssetsTXT`, `exportAssetsCSV`, and `exportFindingsCSV` to `report_exports.go`; move `TestExportAssetsTXTKeepsCurrentLineFormat` to `report_exports_test.go`. Handler-level export tests stay in `report_handler_test.go`.

Run: `rtk gofmt -w internal/web/report_exports*.go`

Run: `rtk go test ./internal/web -run 'Test(Export|ReportAssetExport|ReportExport)'`

Expected: PASS。

- [x] **Step 4: Move view symbols and tests**

Move `runMetaSummaryLimit`, `hostAssetView`, `runMetaView`, `newRunMetaView`, `summarizeRunValue`, `groupFingerprintsByHost`, and `appendUnique` to `report_views.go`; move `TestNewRunMetaViewSummarizesByRune` to `report_views_test.go`.

Delete `reports.go` and `reports_test.go` only after every symbol/test has a destination.

Run: `rtk gofmt -w internal/web/report_views*.go internal/web/report_handler.go`

Run: `rtk go test ./internal/web`

Expected: PASS，无 duplicate/undefined symbol。

- [x] **Step 5: Commit the report split**

```bash
rtk git add internal/web/server.go internal/web/report_handler.go internal/web/report_filters.go internal/web/report_pagination.go internal/web/report_exports.go internal/web/report_views.go internal/web/*report*_test.go
rtk git add -u internal/web/reports.go internal/web/reports_test.go
rtk git commit -m "refactor: separate report delivery responsibilities"
```

### Task 8: Final compatibility and scope audit

**Files:**
- Verify: `cmd/anchorscan/*.go`
- Verify: `internal/web/*.go`
- Verify unchanged: `internal/web/templates/`
- Verify unchanged: `internal/web/static/`
- Verify unchanged: `go.mod`
- Verify unchanged: `go.sum`

**Interfaces:**
- Consumes: Tasks 1-7。
- Produces: 可进入 Comet verify 的完整 change。

- [ ] **Step 1: Format and run the full test suite**

Run: `rtk gofmt -w cmd/anchorscan/*.go internal/web/*.go`

Run: `rtk go test ./...`

Expected: PASS。

Run: `rtk node --test internal/web/static/app.test.mjs`

Expected: PASS，失败数为 0。

- [ ] **Step 2: Build the distributable artifact**

Run: `rtk make package`

Expected: exit 0，当前平台二进制和 tarball 正常生成。

- [ ] **Step 3: Audit dependency and excluded-scope files**

Run: `rtk git diff --exit-code HEAD~6 -- go.mod go.sum internal/web/templates internal/web/static`

Expected: exit 0、无输出；`HEAD~6` 对应本计划规定的六个聚焦提交之前。

Run: `rtk git diff --check`

Expected: exit 0、无 whitespace error。

- [ ] **Step 4: Audit forbidden abstractions**

Run: `rtk rg -n 'type .* interface|gorilla|chi|echo|gin|presenter|repository|service' cmd/anchorscan internal/web`

Expected: 没有本 change 新增的接口、框架或抽象；已有业务文本命中需逐条与基线比较。

- [ ] **Step 5: Inspect the final change set**

Run: `rtk git status --short`

Expected: 只出现本计划列出的 CLI/Web Go 文件和测试文件；没有模板、CSS、JavaScript、数据库或 OpenSpec 的意外修改。

Run: `rtk git log --oneline --max-count=8`

Expected: 能看到兼容测试、CLI 拆分、CLI 测试、Web Handler、Web 测试、报告职责六个聚焦提交。
