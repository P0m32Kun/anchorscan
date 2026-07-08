# Report Asset Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add practical asset search, grouped host view, copy helpers, and export endpoints to the report page so filtered results can be reused in external validation workflows.

**Architecture:** Extend the existing `/reports/{runID}` route instead of creating a new page. Reuse current filter and pagination flow, add a lightweight grouped-host projection in `internal/web/reports.go`, and expose text/CSV exports from the same report handler so the frontend can copy or download the currently filtered asset set.

**Tech Stack:** Go, `net/http`, Go templates, vanilla JavaScript, Go test

## Global Constraints

- Keep the report page as the primary workflow; do not introduce a separate asset-search subsystem.
- Reuse current query-string filters for exports and copy actions.
- Prefer minimal UI additions focused on usability for operators.
- Use TDD for behavior changes and new endpoints.

---

### Task 1: Lock report filtering and grouped-host behavior with tests

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/reports_test.go`
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/server_test.go`

**Interfaces:**
- Consumes: `filterFingerprints([]fingerprint.ServiceFingerprint, reportFilters)`, `NewServer(ServerOptions)`
- Produces: regression coverage for keyword matching, grouped host rendering, and asset export responses

- [ ] **Step 1: Write the failing tests**

```go
func TestFilterFingerprintsMatchesKeywordAcrossFingerprintFields(t *testing.T) {}

func TestReportPageRendersHostViewAndAssetWorkbench(t *testing.T) {}

func TestReportAssetExportSupportsFilteredTXTAndCSV(t *testing.T) {}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web`
Expected: FAIL because grouped host view, export routes, and keyword filtering do not exist yet.

- [ ] **Step 3: Write minimal implementation**

```text
- Extend report filters with a keyword field and host/port view selection.
- Add grouped-host projection helpers in internal/web/reports.go.
- Add /reports/{runID}/assets.txt and /reports/{runID}/assets.csv handling in the report route.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/reports_test.go internal/web/server_test.go internal/web/reports.go internal/web/server.go
git commit -m "feat: add report asset exports"
```

### Task 2: Add report-page copy/export controls and host view UI

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/templates/report.html`
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/static/app.js`
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/static/style.css`

**Interfaces:**
- Consumes: report template data from `reportDetail`
- Produces: operator-facing controls for copy and export without changing the run/report flow

- [ ] **Step 1: Write the failing UI expectations in server tests**

```go
if !strings.Contains(body, "按主机聚合") { t.Fatal(...) }
if !strings.Contains(body, "复制 IP:PORT") { t.Fatal(...) }
if !strings.Contains(body, "/assets.csv") { t.Fatal(...) }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web -run 'TestReportPageRendersHostViewAndAssetWorkbench'`
Expected: FAIL because the controls are not rendered.

- [ ] **Step 3: Write minimal implementation**

```text
- Add a compact workbench bar above the asset table.
- Add copy buttons backed by fetch + clipboard in app.js.
- Add a host view table that shows IP, open ports, services, and row-level copy actions.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/templates/report.html internal/web/static/app.js internal/web/static/style.css
git commit -m "feat: add report asset workbench ui"
```

### Task 3: Full verification

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/README.md` (only if usage text is needed)

**Interfaces:**
- Consumes: final report behavior
- Produces: verified implementation and any small documentation alignment needed

- [ ] **Step 1: Update documentation only if the new export workflow needs it**

```text
- Add one short README note if report export/copy endpoints are user-visible enough to document.
```

- [ ] **Step 2: Run full verification**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: note report asset exports"
```
