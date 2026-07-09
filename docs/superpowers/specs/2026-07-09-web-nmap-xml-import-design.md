---
comet_change: web-nmap-xml-import
role: technical-design
canonical_spec: openspec
---

# Web Console Nmap XML 导入入口 技术设计

## 背景

AnchorScan 已有 `anchorscan import-nmap` CLI 命令（enhance-nmap-xml-import change），能把已有 Nmap XML 导入为完成态 run，保留 protocol、CPE、NSE script 输出与 scope。但 Web Console 目前没有导入入口，用户只能通过命令行导入。本设计把导入能力暴露到 Web Console，复用现有的 multipart 上传、模板渲染和 run 详情链路。

## 目标

- 在 Web Console 侧边栏新增"导入 Nmap XML"入口和表单页。
- 用户上传 XML 文件后同步导入，成功跳转到 run 详情页。
- 复用现有 `app.ImportNmap` 编排和 run/store/report 链路，不新建独立导入模型。
- 导入失败（空 XML / 非 nmaprun / 非法 XML / 文件过大）时重渲染表单页并显示错误，不留半截 run。
- 改动 app 层让 `ImportNmap` 既接受磁盘路径（CLI）也接受内存 `[]byte`（Web），避免临时文件中转。

## 非目标

- 不把导入做成异步 + 轮询进度。导入是同步的，对常见 Nmap XML（几 MB）足够快。
- 不引入新的 JS 框架或构建工具。表单提交走整页 POST + 303 重定向。
- 不在本 change 内增加批量导入或拖拽上传等增强交互。
- 不改变 `ImportNmap` 的事务语义或报告生成逻辑。

## 架构方案

采用轻量增强方案，4 层改动都贴合现有模式：

| 层 | 改动 | 复用 |
|----|------|------|
| app 层 | `ImportNmapOptions` 加 `XMLData []byte`；`ImportNmap` 在 `XMLPath` 为空时直接用 `XMLData` | 现有 `ParseNmapXML → Classify → SaveImportRun → report` 全部不动 |
| web 路由 | 新增 `/import/nmap`（GET 表单）+ `/import/nmap/run`（POST 处理）| 仿 `scanCreate` 的 POST handler 模式 |
| 模板 | 新增 `import_nmap.html` 表单页 | 复用 `base.html` layout、`project_form.html` 的 multipart 表单样式 |
| 导航 | `base.html` 侧边栏加"导入 Nmap XML"项；高亮脚本加 `/import` 分支 | 复用现有 nav-item 模式 |

导入命令的用户路径是：

```text
Web Console 侧边栏 → 导入 Nmap XML
  → GET /import/nmap 渲染表单（XML 文件 + 可选关联项目）
  → POST /import/nmap/run (multipart/form-data)
  → handler 读 xml_file → []byte
  → app.ImportNmap(ctx, store, opts{XMLData, ProjectID, RunID, JSONPath})
  → 成功：303 重定向到 /runs/{runID}
  → 失败：重渲染表单页，顶部显示错误横幅
```

## app 层改动

`ImportNmapOptions` 增加字段，`ImportNmap` 内部按字段优先级取数据源：

```go
type ImportNmapOptions struct {
    XMLPath   string  // CLI 用：磁盘路径，优先
    XMLData   []byte  // Web 用：内存数据，XMLPath 为空时使用
    RunID     string
    ProjectID string
    JSONPath  string
    HTMLPath  string
    Now       func() time.Time
}
```

`ImportNmap` 开头的读数据逻辑：

```go
var data []byte
if opts.XMLPath != "" {
    data, err = os.ReadFile(opts.XMLPath)
} else {
    data = opts.XMLData
}
```

`--xml is required` 校验放宽为：`XMLPath == "" && len(XMLData) == 0` 才报错。

后续 `ParseNmapXML(data) → Classify → SaveImportRun(事务) → 写 JSON/HTML` 全部不动。现有 CLI（传 XMLPath）和现有测试零影响——XMLPath 优先。

## Web 路由

### `/import/nmap`（GET）— importNmapForm

渲染表单页，加载项目列表供下拉选择。复用 `render(w, "templates/import_nmap.html", data)` 和现有的项目列表读取方法。

### `/import/nmap/run`（POST）— importNmapRun

仿 `scanCreate`（`server.go:449-593`）的 POST handler 结构：

1. 方法守卫 + `r.ParseMultipartForm(8 << 20)`（复用 `parseProjectRequest` 的 8 MiB 上限）。
2. `r.FormFile("xml_file")` → `io.ReadAll` → `[]byte`。文件读取失败（含未选文件）归为导入错误。
3. 组装 `ImportNmapOptions{XMLData: data, ProjectID: r.FormValue("project_id"), JSONPath: managedReportPath(s.opts.DBPath, projectID, runID), RunID: newID("run", s.opts.Now())}`。
4. 调 `app.ImportNmap(ctx, scanStore, opts)`。
5. 成功 → `http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)`。
6. 失败 → 用错误信息重新渲染表单页（顶部错误横幅），返回 HTTP 200（保留表单状态，便于浏览器回退）。

报告路径用 `managedReportPath(dbPath, projectID, runID)`，让导入的 run 落到和扫描 run 相同的 managed 数据目录（`<dataroot>/runs/<runID>/report.json`），run 详情页能正常展示。

## 表单页模板

新增 `internal/web/templates/import_nmap.html`，参照 `project_form.html` 的 multipart 表单样式，用 `{{define "content"}}...{{end}}` 包裹。

表单字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| XML 文件 | `input type="file"` 必填，`accept=".xml"`，`name="xml_file"` | multipart 上传 |
| 关联项目 | `select` 可选，`name="project_id"` | 下拉选择已有项目 |
| 提交 | `submit` | "导入" |

表单顶部预留错误横幅位：`{{if .Error}}<div class="error-banner">{{.Error}}</div>{{end}}`。页面文案使用中文，与 `.comet/config.yaml` 的 `language: zh-CN` 一致。

## 导航入口

`base.html` 侧边栏（"扫描历史"后）加一项：

```html
<a href="/import/nmap" class="nav-item" id="nav-import">
  <svg>...upload icon...</svg>
  <span>导入 Nmap XML</span>
</a>
```

高亮脚本（`base.html:64-79`）加一条 `else if (path.startsWith("/import"))` 激活该项。

## 错误处理

所有导入失败都走"重渲染表单 + 顶部错误横幅"，不留半截 run（事务保证）：

| 场景 | 错误来源 | 用户看到的提示 |
|------|---------|--------------|
| 没选文件 / 文件为空 | ImportNmap `empty XML file` | 重渲染表单 |
| 根节点非 nmaprun | ImportNmap `root element is not nmaprun` | 重渲染表单 |
| 非法 XML | ImportNmap `invalid Nmap XML: ...` | 重渲染表单 |
| 上传超过 8 MiB | `ParseMultipartForm` 错误 | 重渲染表单（"文件过大"）|
| 落库失败 | 事务回滚，底层错误 | 重渲染表单 |

## 测试策略

测试沿用仓库现有的临时 SQLite + 内联 fixture 模式：

- **app 层**：新增 `TestImportNmapAcceptsXMLData`，传 `XMLData`（而非 `XMLPath`）验证成功导入，确认两条 53 服务（tcp/udp）落库。
- **web 层**：新增 handler 测试（仿 `server_test.go` 的 multipart 写法 `writer.CreateFormFile`）：
  - 成功上传（合法 XML）→ 303 重定向到 `/runs/{id}`，DB 有完成态 run。
  - 空文件上传 → 200 + 表单含错误横幅，DB 无新增 run。
  - 非 nmaprun XML → 200 + 错误横幅，DB 无新增 run。
  - 未选文件（无 multipart 字段）→ 200 + 错误横幅。
- **现有测试回归**：确认所有传 `XMLPath` 的现有测试（app 和 CLI）仍全过。

## 迁移与兼容

- app 层改动向后兼容：`XMLPath` 优先，现有 CLI 和测试零影响。
- 无 schema 变更（数据库列在 enhance-nmap-xml-import change 已加好）。
- 新增路由和模板不影响现有页面。

## 后续能力

本 change 刻意保持同步导入。后续如需：
- 异步导入 + 轮询进度（大文件场景），可给 Manager 加 `StartImport` 复用 activeID 互斥。
- 批量导入 / 拖拽上传，可基于本次表单页扩展。
