# Web Console Nmap XML 导入入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Web Console 增加一个 Nmap XML 导入入口，用户上传 XML 后同步导入为完成态 run 并跳转到 run 详情页。

**Architecture:** 4 层轻量增强——app 层 `ImportNmapOptions` 加 `XMLData []byte` 让 Web 复用现有导入编排；web 层新增两个路由（GET 表单 / POST 处理）和一个表单模板；base.html 侧边栏加入口。表单走整页 multipart POST + 303 重定向，失败重渲染表单显示错误。

**Tech Stack:** Go 1.26 标准库（`net/http` ServeMux、`mime/multipart`）、`html/template`（embed）、纯原生 fetch（无需新依赖）。

**Design Doc:** `docs/superpowers/specs/2026-07-09-web-nmap-xml-import-design.md`

## 代码现实（已确认）

| 现有资产 | 位置 | 复用方式 |
|---------|------|---------|
| `app.ImportNmap(ctx, store, ImportNmapOptions)` | `internal/app/import.go:31` | 加 `XMLData` 字段后 Web 直接调 |
| 路由注册 | `internal/web/server.go:107-120` | 仿照加两行 `mux.HandleFunc` |
| POST handler 样板 `scanCreate` | `internal/web/server.go:449-593` | 仿照结构（方法守卫→解析→调 app→重定向） |
| 表单渲染样板 `renderScanForm` | `internal/web/server.go:312-323` | 仿照加载项目列表 + render |
| multipart 上传样板 `parseProjectRequest` | `internal/web/server.go:895-903` | 复用 `ParseMultipartForm(8<<20)` |
| multipart 表单模板 `project_form.html` | `internal/web/templates/project_form.html` | 仿照 `enctype` + `input type=file` |
| `render(w, file, data)` | `internal/web/server.go:1186-1193` | 直接调 |
| `managedReportPath(dbPath, projectID, runID)` | `internal/web/server.go:73-79` | 直接复用生成报告路径 |
| `newID(prefix, now)` | `internal/web/server.go:1195-1197` | 直接复用生成 runID |
| `s.store.ListProjects()` | `internal/web/server.go:313` | 复用加载项目下拉框 |
| multipart 测试样板 | `internal/web/server_test.go:73-100` | 仿照 `CreateFormFile` |
| 导航 + 高亮脚本 | `internal/web/templates/base.html:21-79` | 加一项 nav-item + 一条高亮分支 |

**关键**：`ImportNmap` 当前 `internal/app/import.go:34` 是 `opts.XMLPath == ""` 校验，内部 `import.go:38` 用 `os.ReadFile(opts.XMLPath)`。改为支持 `XMLData` 后，`XMLPath` 优先（CLI 零影响）。

## Global Constraints

- 保持单二进制交付，不引入新依赖。
- 导入是同步的，不走 Manager 异步机制（与 spec 一致）。
- 所有用户可见文案使用中文（与 `.comet/config.yaml` 的 `language: zh-CN` 一致）。
- multipart 上传上限 8 MiB（复用 `parseProjectRequest` 的 `8 << 20`）。
- app 层改动向后兼容：`XMLPath` 优先，现有 CLI 和测试零影响。
- 每个任务完成后跑相关测试；全部完成后跑 `go test ./...`。
- 无 schema 变更（数据库列在 enhance-nmap-xml-import change 已加好）。

---

## Task 1: app 层支持 XMLData []byte

**Files:**
- Modify: `internal/app/import.go:19-26`（ImportNmapOptions 加字段）、`internal/app/import.go:33-40`（读数据逻辑）、`internal/app/import.go:35`（校验放宽）
- Test: `internal/app/import_test.go`（新增测试）

**Interfaces:**
- Produces: `ImportNmapOptions.XMLData []byte`（新字段，Web handler 会用它）；`ImportNmap` 仍返回 `(string, error)`，签名不变。

- [ ] **Step 1: 写失败测试 — ImportNmap 接受 XMLData**

在 `internal/app/import_test.go` 末尾新增测试函数：

```go
func TestImportNmapAcceptsXMLData(t *testing.T) {
	scanStore := newScanStore(t)

	runID, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{
		XMLData: []byte(importFixtureXML),
		Now:     func() time.Time { return time.Unix(1700000000, 0) },
	})
	if err != nil {
		t.Fatalf("ImportNmap returned error: %v", err)
	}

	fps, err := scanStore.ListFingerprints(runID)
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 2 {
		t.Fatalf("expected two fingerprints (tcp+udp), got %d", len(fps))
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/app/... -run TestImportNmapAcceptsXMLData -v`
Expected: FAIL（当前 ImportNmap 用 XMLPath 为空校验，XMLData 会被当作"缺 xml"拒绝）

- [ ] **Step 3: 给 ImportNmapOptions 加 XMLData 字段**

修改 `internal/app/import.go` 的 `ImportNmapOptions` 结构体（在 XMLPath 后加 XMLData）：

```go
type ImportNmapOptions struct {
	XMLPath   string
	XMLData   []byte
	RunID     string
	ProjectID string
	JSONPath  string
	HTMLPath  string
	Now       func() time.Time
}
```

- [ ] **Step 4: 修改 ImportNmap 校验和读数据逻辑**

修改 `internal/app/import.go` 的 `ImportNmap` 函数开头两处：

校验放宽（原 `if opts.XMLPath == ""`）：
```go
	if opts.XMLPath == "" && len(opts.XMLData) == 0 {
		return "", errors.New("--xml is required")
	}
```

读数据逻辑（原 `data, err := os.ReadFile(opts.XMLPath)`），改为按字段优先级：
```go
	var data []byte
	if opts.XMLPath != "" {
		data, err = os.ReadFile(opts.XMLPath)
		if err != nil {
			return "", err
		}
	} else {
		data = opts.XMLData
	}
```

注意：原 `data, err := os.ReadFile(opts.XMLPath)` 这行下面紧跟的 `if err != nil { return "", err }` 要合并进上面的 `if` 分支里。删除原来独立的 `if err != nil` 块。

- [ ] **Step 5: 运行测试验证通过**

Run: `go test ./internal/app/... -run Import -v`
Expected: 全部 PASS（包括新测试和原有 XMLPath 路径测试）

- [ ] **Step 6: 提交**

```bash
git add internal/app/import.go internal/app/import_test.go
git commit -m "feat: ImportNmap accepts in-memory XMLData for web upload"
```

---

## Task 2: Web 路由注册 + GET 表单 handler

**Files:**
- Modify: `internal/web/server.go:107-120`（路由注册）、新增 handler 方法
- Create: `internal/web/templates/import_nmap.html`（表单页）

**Interfaces:**
- Consumes: Task 1 的 `ImportNmapOptions.XMLData`、`app.ImportNmap`
- Produces: `GET /import/nmap` 渲染表单页（供 Task 3 的 POST handler 重渲染复用）；`renderImportForm(w, errMsg)` 方法

- [ ] **Step 1: 写失败测试 — GET /import/nmap 渲染表单**

在 `internal/web/server_test.go` 末尾新增测试：

```go
func TestImportNmapFormRenders(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/import/nmap", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "导入 Nmap XML") || !strings.Contains(body, `name="xml_file"`) {
		t.Fatalf("expected import form, got: %s", body)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/web/... -run TestImportNmapFormRenders -v`
Expected: FAIL（404，路由未注册）

- [ ] **Step 3: 注册路由**

在 `internal/web/server.go` 的 `NewServer` 路由块（`mux.HandleFunc("/config", s.configPage)` 前或后），加两行：

```go
	mux.HandleFunc("/import/nmap", s.importNmapForm)
	mux.HandleFunc("/import/nmap/run", s.importNmapRun)
```

- [ ] **Step 4: 写 importNmapForm handler 和 renderImportForm 辅助方法**

在 `internal/web/server.go` 里（建议放在 `scanCreate` 函数 `server.go:593` 之后），新增：

```go
func (s *server) importNmapForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.renderImportForm(w, "")
}

// renderImportForm renders the Nmap XML import page with the project list and
// an optional error message shown in a top banner.
func (s *server) renderImportForm(w http.ResponseWriter, errMsg string) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/import_nmap.html", map[string]any{
		"Projects": projects,
		"Error":    errMsg,
	})
}
```

- [ ] **Step 5: 创建 import_nmap.html 模板**

创建 `internal/web/templates/import_nmap.html`（仿 project_form.html 的 multipart 样式）：

```html
{{define "content"}}
<section class="page-header">
  <div>
    <p class="eyebrow">数据导入</p>
    <h2>导入 Nmap XML</h2>
  </div>
  <a class="button button-secondary" href="/runs">返回扫描历史</a>
</section>

{{if .Error}}
<section class="panel" style="border-color: var(--danger, #dc2626); background: rgba(220,38,38,0.08);">
  <p style="color: var(--danger, #dc2626); margin: 0; font-weight: 600;">导入失败：{{.Error}}</p>
</section>
{{end}}

<section class="panel">
  <form class="form-grid" method="post" action="/import/nmap/run" enctype="multipart/form-data">
    <label class="full-width">
      <span>Nmap XML 文件 *</span>
      <div style="font-size: 0.75rem; color: var(--muted); margin-bottom: 0.5rem; margin-top: 0.25rem;">上传已有的 Nmap XML 扫描结果，系统会保留协议（TCP/UDP）、CPE 和 NSE 脚本输出。</div>
      <input type="file" name="xml_file" accept=".xml" required>
    </label>

    <label class="full-width">
      <span>关联项目（可选）</span>
      <select name="project_id">
        <option value="">不关联项目</option>
        {{range .Projects}}
        <option value="{{.ID}}">{{.Name}}</option>
        {{end}}
      </select>
    </label>

    <div class="form-actions full-width" style="margin-top: 1.5rem;">
      <button class="button button-primary" type="submit">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" style="width: 1.2rem; height: 1.2rem;">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
        </svg>
        <span>导入</span>
      </button>
    </div>
  </form>
</section>
{{end}}
```

- [ ] **Step 6: 运行测试验证通过**

Run: `go test ./internal/web/... -run TestImportNmapFormRenders -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/web/server.go internal/web/templates/import_nmap.html internal/web/server_test.go
git commit -m "feat: add /import/nmap form page to web console"
```

---

## Task 3: POST 导入 handler（成功 + 失败重渲染）

**Files:**
- Modify: `internal/web/server.go`（新增 `importNmapRun` handler）
- Test: `internal/web/server_test.go`（新增 POST 测试）

**Interfaces:**
- Consumes: Task 1 的 `ImportNmap(ctx, store, ImportNmapOptions{XMLData...})`、Task 2 的 `renderImportForm`、`managedReportPath`、`newID`

- [ ] **Step 1: 写失败测试 — 成功上传重定向到 run**

在 `internal/web/server_test.go` 末尾新增测试：

```go
func TestImportNmapRunRedirectsToRun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "scan.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte(`<nmaprun>
  <host>
    <address addr="10.0.0.53"/>
    <ports>
      <port protocol="tcp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
    </ports>
  </host>
</nmaprun>`)); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", res.Code)
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/runs/") {
		t.Fatalf("expected redirect to /runs/, got %q", location)
	}

	// 验证 DB 有完成态 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("expected one completed run, got %d err=%v", len(runs), err)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/web/... -run TestImportNmapRunRedirectsToRun -v`
Expected: FAIL（404 或 405，POST handler 未实现）

- [ ] **Step 3: 实现 importNmapRun handler**

在 `internal/web/server.go` 的 `importNmapForm` 方法之后，新增 `importNmapRun`：

```go
func (s *server) importNmapRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		s.renderImportForm(w, "文件过大或格式错误")
		return
	}

	file, _, err := r.FormFile("xml_file")
	if err != nil {
		s.renderImportForm(w, "请选择要导入的 Nmap XML 文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		s.renderImportForm(w, err.Error())
		return
	}

	projectID := r.FormValue("project_id")
	runID := newID("run", s.opts.Now())
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)

	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = app.ImportNmap(context.Background(), s.store, app.ImportNmapOptions{
		XMLData:   data,
		RunID:     runID,
		ProjectID: projectID,
		JSONPath:  jsonPath,
	})
	if err != nil {
		s.renderImportForm(w, err.Error())
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
}
```

确认 `internal/web/server.go` 顶部 import 已包含 `io`、`os`、`context`、`path/filepath`、`github.com/P0m32Kun/anchorscan/internal/app`（这些在文件里大多已存在，因为 scanCreate 用了同样的包；若缺少则补上）。

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/web/... -run TestImportNmapRunRedirectsToRun -v`
Expected: PASS

- [ ] **Step 5: 写失败测试 — 空 XML 重渲染表单**

在 `internal/web/server_test.go` 末尾新增：

```go
func TestImportNmapRunEmptyFileRendersFormError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "empty.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte("")); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 (form re-render), got %d", res.Code)
	}
	pageBody := res.Body.String()
	if !strings.Contains(pageBody, "empty XML file") {
		t.Fatalf("expected error banner, got: %s", pageBody)
	}

	// 验证 DB 无新增 run
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}
```

- [ ] **Step 6: 运行测试验证通过**

Run: `go test ./internal/web/... -run TestImportNmapRunEmptyFile -v`
Expected: PASS（handler 已在 Step 3 实现，空文件走 ImportNmap 的 `empty XML file` 错误 → renderImportForm）

- [ ] **Step 7: 写测试 — 非 nmaprun XML 重渲染表单**

在 `internal/web/server_test.go` 末尾新增（复用上面的结构，只换 XML 内容和断言文案）：

```go
func TestImportNmapRunNonNmaprunRendersFormError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088", Now: func() time.Time { return time.Unix(10, 0) }})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("xml_file", "foo.xml")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write([]byte(`<foo><bar/></foo>`)); err != nil {
		t.Fatalf("file write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import/nmap/run", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %s", res.Body.String())
	}
}
```

- [ ] **Step 8: 运行测试验证通过**

Run: `go test ./internal/web/... -run TestImportNmapRun -v`
Expected: 全部 PASS（成功重定向 + 空文件 + 非 nmaprun）

- [ ] **Step 9: 提交**

```bash
git add internal/web/server.go internal/web/server_test.go
git commit -m "feat: web import-nmap POST handler with error re-render"
```

---

## Task 4: 导航入口（base.html）

**Files:**
- Modify: `internal/web/templates/base.html:39`（导航项）、`base.html:72-78`（高亮脚本）

- [ ] **Step 1: 写失败测试 — 导航含导入入口**

在 `internal/web/server_test.go` 末尾新增（复用 TestHomePageRenders 的 server 启动模式）：

```go
func TestNavIncludesImportNmapEntry(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, err := NewServer(ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath, Listen: "127.0.0.1:8088"})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	body := res.Body.String()
	if !strings.Contains(body, `href="/import/nmap"`) || !strings.Contains(body, "导入 Nmap XML") {
		t.Fatalf("expected import nav entry, got: %s", body)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/web/... -run TestNavIncludesImportNmap -v`
Expected: FAIL（导航无导入入口）

- [ ] **Step 3: 在 base.html 侧边栏加导航项**

在 `internal/web/templates/base.html` 的"扫描历史"导航项（`id="nav-runs"` 那个 `<a>`，约 39 行 `</a>` 之后）之后，插入：

```html
      <a href="/import/nmap" class="nav-item" id="nav-import">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
        </svg>
        <span>导入 Nmap XML</span>
      </a>
```

- [ ] **Step 4: 在高亮脚本加 /import 分支**

在 `internal/web/templates/base.html` 的高亮脚本里（约 74 行 `} else if (path.startsWith("/tools"))` 之前），加一条分支：

```javascript
      } else if (path.startsWith("/import")) {
        document.getElementById("nav-import").classList.add("active");
```

- [ ] **Step 5: 运行测试验证通过**

Run: `go test ./internal/web/... -run TestNavIncludesImportNmap -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/web/templates/base.html internal/web/server_test.go
git commit -m "feat: add import-nmap entry to web console sidebar"
```

---

## Task 5: 全量验证与回归

- [ ] **Step 1: 全量编译与测试**

Run: `go build ./... && go test ./...`
Expected: 全部 ok，0 失败。确认现有 XMLPath 路径测试（app 和 cmd）未受影响。

- [ ] **Step 2: 实跑 Web 导入**

```bash
go build -o /tmp/anchorscan ./cmd/anchorscan
/tmp/anchorscan web --db /tmp/import-web.sqlite &
sleep 1
# 用 curl 上传 fixture
curl -s -o /dev/null -w "%{http_code} %{redirect_url}" \
  -F "xml_file=@/tmp/import-sample.xml" \
  http://127.0.0.1:8088/import/nmap/run
```
Expected: `303 /runs/run-<timestamp>`（如果 /tmp/import-sample.xml 不存在，用 enhance-nmap-xml-import 的 fixture 内容重建，或直接跳过实跑靠测试覆盖）

- [ ] **Step 3: 提交（如有改动）**

如全量验证发现需要调整，提交修复；否则此步无操作。

---

## 任务依赖与执行顺序

```
Task 1（app 层 XMLData）是基础，先做。
Task 2（GET 表单 + 模板）依赖 Task 1 的字段已存在（编译需要），但逻辑独立。
Task 3（POST handler）依赖 Task 1（ImportNmap XMLData）+ Task 2（renderImportForm）。
Task 4（导航）独立，可与 Task 2/3 并行。
Task 5（全量验证）最后做。
```

**推荐 TDD 顺序：** 每个任务先写测试（红），再实现（绿），再提交。
