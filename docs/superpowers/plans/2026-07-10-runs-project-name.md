# 扫描历史显示项目名称 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仅在 `/runs` 扫描历史页面显示所属项目名称，同时保留项目详情链接。

**Architecture:** `server.runs` 读取扫描记录后调用现有 `ListProjects`，构建 `map[string]string` 项目 ID 到项目名称的映射，并将其与记录传给 `runs.html`。模板用记录中的 ID 生成链接，用映射名称作为链接文字；无法关联的历史项目显示“已删除项目”。

**Tech Stack:** Go、`html/template`、现有 SQLite store、Go 标准测试库。

## Global Constraints

- 只修改 `/runs` 页面，不改变首页、详情页或数据库结构。
- 复用现有 `store.ListProjects`、`store.ListScanRuns` 和项目链接。
- 无项目 ID 的记录继续显示 `—`。
- 不新增依赖或抽象层。

---

### Task 1: 为扫描历史项目名称建立回归测试并实现页面映射

**Files:**
- Modify: `internal/web/server_test.go:639-668`（现有 `TestRunsPageShowsProjectID`）
- Modify: `internal/web/server.go:778-790`（`runs` 方法）
- Modify: `internal/web/templates/runs.html:34-37`（项目列）

**Interfaces:**
- Consumes: `store.ListScanRuns(limit int) ([]store.ScanRun, error)` 和 `store.ListProjects() ([]store.Project, error)`。
- Produces: 模板数据 `ProjectNames map[string]string`，供 `runs.html` 通过 `index` 查找项目名称。

- [ ] **Step 1: 修改现有页面测试，先断言项目名称**

在 `TestRunsPageShowsProjectID` 保存扫描记录前加入：

```go
if err := scanStore.SaveProject(store.Project{
    ID:        "p1",
    Name:      "Local Lab",
    CreatedAt: time.Unix(1, 0),
    UpdatedAt: time.Unix(1, 0),
}); err != nil {
    t.Fatalf("SaveProject returned error: %v", err)
}
```

并将页面断言改为同时要求项目名称和原有链接，且拒绝单独显示项目 ID：

```go
body := res.Body.String()
if !strings.Contains(body, `/projects/p1`) || !strings.Contains(body, "Local Lab") {
    t.Fatalf("expected project name and link in runs page: %s", body)
}
if strings.Contains(body, `>p1</a>`) {
    t.Fatalf("expected project name instead of project ID: %s", body)
}
```

- [ ] **Step 2: 运行针对性测试，确认当前实现失败**

Run: `go test ./internal/web -run TestRunsPageShowsProjectID -count=1`

Expected: FAIL，因为当前模板仍渲染 `p1`，处理器也没有提供项目名称映射。

- [ ] **Step 3: 在 `runs` 处理器构建项目名称映射**

在成功读取 `runs` 后加入项目列表读取和映射构建；读取项目失败时返回 500：

```go
projects, err := s.store.ListProjects()
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
projectNames := make(map[string]string, len(projects))
for _, project := range projects {
    projectNames[project.ID] = project.Name
}
render(w, "templates/runs.html", map[string]any{
    "Runs":         runs,
    "ProjectNames": projectNames,
})
```

- [ ] **Step 4: 更新模板只替换项目列文字**

将现有项目列替换为：

```gotemplate
<td>
  {{if .ProjectID}}
  <a href="/projects/{{.ProjectID}}" class="status-badge">
    {{with index $.ProjectNames .ProjectID}}{{.}}{{else}}已删除项目{{end}}
  </a>
  {{else}}<span class="muted">—</span>{{end}}
</td>
```

这保留项目 ID 作为 URL，只把可见文本换成名称；历史记录找不到项目时不回退显示 ID。

- [ ] **Step 5: 运行针对性测试，确认实现通过**

Run: `go test ./internal/web -run TestRunsPageShowsProjectID -count=1`

Expected: PASS。

- [ ] **Step 6: 运行完整 Web 包测试和格式检查**

Run: `gofmt -w internal/web/server.go internal/web/server_test.go && go test ./internal/web`

Expected: PASS，且无测试失败。

- [ ] **Step 7: 检查最终差异，确保范围仅限本需求**

Run: `git diff --check && git diff -- internal/web/server.go internal/web/server_test.go internal/web/templates/runs.html`

Expected: 只有 `/runs` 处理器的项目映射、扫描历史模板项目列和对应测试断言发生变化；不修改其他页面或数据结构。
