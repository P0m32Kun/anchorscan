# Target Input Separator Copy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align AnchorScan target input UX so CLI documents comma-separated multi-target input, while the web UI accepts comma- or newline-separated targets.

**Architecture:** Keep backend parsing simple and reuse the existing target parser. Change only presentation and tests: make the scan page target field multi-line, update copy in templates and README, and verify with focused tests.

**Tech Stack:** Go, html/template, Go test

## Global Constraints

- Do not change scan semantics beyond target input presentation and documented separator rules.
- CLI examples should prefer comma-separated targets.
- Web UI should explicitly allow comma-separated or newline-separated targets.
- Use TDD for any code or behavior change.

---

### Task 1: Lock the intended UX with tests

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/internal/web/server_test.go`
- Modify: `/Users/kun/DEV/new-Anchor/internal/target/parse_test.go`

**Interfaces:**
- Consumes: `NewServer(ServerOptions)`, `Parse(string) ([]string, error)`
- Produces: regression coverage for scan page copy and mixed separator parsing

- [ ] **Step 1: Write the failing tests**

```go
func TestNewScanPageRendersTargetTextarea(t *testing.T) {
    // assert the page contains a textarea and the new copy
}

func TestParseSupportsCommaAndNewlineSeparatedTargets(t *testing.T) {
    got, err := Parse("192.168.1.10,192.168.1.11\n192.168.1.12")
    // assert deduplicated ordered targets
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web ./internal/target`
Expected: FAIL because the scan page still renders a single-line input and copy is outdated.

- [ ] **Step 3: Write minimal implementation**

```text
- Change scan_new.html target field from <input> to <textarea>.
- Update target help text in scan_new.html and project_form.html.
- Keep Parse behavior, only ensure mixed-separator test passes.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web ./internal/target`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/web/server_test.go internal/target/parse_test.go internal/web/templates/scan_new.html internal/web/templates/project_form.html
 git commit -m "fix: align target separator ux"
```

### Task 2: Sync CLI and README wording

**Files:**
- Modify: `/Users/kun/DEV/new-Anchor/README.md`
- Modify: `/Users/kun/DEV/new-Anchor/cmd/anchorscan/main.go` (only if wording needs tightening)

**Interfaces:**
- Consumes: current CLI help text and usage examples
- Produces: documentation that matches the actual UX

- [ ] **Step 1: Write the failing doc expectation mentally from Task 1**

```text
README and help must state: CLI multi-target input uses commas; web UI accepts commas or newlines.
```

- [ ] **Step 2: Update docs minimally**

```text
- Replace web target wording that says "每行一个目标" with "支持英文逗号或换行分隔多个目标".
- Ensure CLI examples show comma-separated targets when multiple targets are demonstrated.
```

- [ ] **Step 3: Verify rendered help and tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add README.md cmd/anchorscan/main.go
 git commit -m "docs: clarify target separator usage"
```
