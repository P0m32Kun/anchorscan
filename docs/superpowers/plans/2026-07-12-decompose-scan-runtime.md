---
change: decompose-scan-runtime
design-doc: docs/superpowers/specs/2026-07-12-decompose-scan-runtime-design.md
base-ref: f61b7109ba37a0d85466fbd023c22568723308b1
---

# Decompose Scan Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `RunScan` 的生命周期、多目标调度和单目标固定流水线拆成三个同包职责边界，同时保持所有调用和可观察行为不变。

**Architecture:** `internal/app/scan.go` 继续拥有公开入口和 run 生命周期；`scan_targets.go` 接管存活探测、worker pool 与失败归并；`scan_target.go` 接管单目标 RustScan → Nmap → HTTPX → NSE/Nuclei 固定流水线。此次只机械移动现有代码，使用现有 `tools.Runner`、store、report 和测试 fake，不增加接口、上下文对象、依赖或子包。

**Tech Stack:** Go 标准库、现有 `internal/tools`、`internal/store`、`internal/report`、Go `testing`、Make。

## Global Constraints

- 从提交 `f61b7109ba37a0d85466fbd023c22568723308b1` 开始实施；开始前运行 `rtk git status --short`，若工作区存在与本计划文件重叠的未提交修改，先停止并请求确认。
- `RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) error`、`ScanOptions`、Manager 和 `tools.Runner` 契约不变。
- 工具调用与顺序、参数、worker 裁剪、channel 容量、goroutine 启停、取消优先级、首错选择、事件、heartbeat、artifact 名称和 JSON 报告行为不变。
- 所有生产代码仍在 `internal/app`；不新增导出符号、子包、接口、stage registry、插件系统、配置项或第三方依赖。
- helper 仅在已有多个真实调用方时保留；不得创建可变的 scan context、orchestrator 类型或一行转发包装。
- 每次机械迁移后先执行 `rtk gofmt`，再执行指定测试；测试失败时撤销本任务尚未提交的机械迁移，不增加兼容分支。

## File Map

- Modify: `internal/app/scan.go` — 只保留 `ScanOptions`、`RunScan`、run 状态生命周期、最终报告写入，以及同时被生命周期和流水线使用的 `logf`、`emit`、`normalizeToolError`。
- Create: `internal/app/scan_targets.go` — `targetResult`、存活探测、worker 裁剪、目标分发/收集、取消与部分/全部失败归并。
- Create: `internal/app/scan_target.go` — `scanTarget`、Nmap heartbeat、单目标工具流水线、`formatNucleiEvidence` 和 `findingFromNuclei`。
- Modify: `internal/app/scan_test.go` — 保留共享 runner/store fake 和少量完整 `RunScan` 组合测试；从中机械移动边界测试。
- Create: `internal/app/scan_lifecycle_test.go` — run 完成/失败/取消、报告失败和最终状态测试。
- Create: `internal/app/scan_targets_test.go` — 存活探测、worker 边界、取消、部分失败和全部失败测试。
- Create: `internal/app/scan_target_test.go` — 单目标工具顺序、参数、heartbeat、finding 和失败 artifact 测试。
- Verify only: `cmd/anchorscan/main.go`, `internal/app/manager.go` — 调用方不得修改。

---

### Task 1: 锁定尚未覆盖的生命周期和 worker 边界

**Files:**
- Create: `internal/app/scan_lifecycle_test.go`
- Create: `internal/app/scan_targets_test.go`
- Reuse: `internal/app/scan_test.go` 中的 `newScanStore`、`newPostAliveConcurrencyRunner`

**Interfaces:**
- Consumes: 当前公开 `RunScan(context.Context, tools.Runner, *store.Store, ScanOptions) error`。
- Produces: 报告写入失败的 run 状态契约；`HostWorkers <= 0` 默认 1、超过目标数时裁剪到目标数的契约。

- [ ] **Step 1: 运行现有扫描测试基线**

Run: `rtk go test ./internal/app`

Expected: PASS；若失败，不开始拆分，先报告基线失败。

- [ ] **Step 2: 写报告失败生命周期特征测试**

创建 `internal/app/scan_lifecycle_test.go`，内容如下。该测试只使用无存活主机路径，使失败点确定地落在最终 `report.WriteJSON`：

```go
package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunScanRecordsReportWriteFailure(t *testing.T) {
	scanStore := newScanStore(t)
	reportPath := filepath.Join(t.TempDir(), "missing", "report.json")
	err := RunScan(context.Background(), &downHostRunner{}, scanStore, ScanOptions{
		RunID:          "run-report-failure",
		Targets:        []string{"172.22.0.7"},
		Ports:          "1-65535",
		Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
		JSONReportPath: reportPath,
	})
	if err == nil {
		t.Fatal("expected report write error")
	}
	run, getErr := scanStore.GetScanRun("run-report-failure")
	if getErr != nil {
		t.Fatalf("GetScanRun returned error: %v", getErr)
	}
	if run.Status != "failed" || !strings.Contains(run.Message, "missing") {
		t.Fatalf("unexpected failed run: %#v", run)
	}
}
```

- [ ] **Step 3: 运行生命周期特征测试，确认当前实现已满足契约**

Run: `rtk go test ./internal/app -run '^TestRunScanRecordsReportWriteFailure$' -count=1`

Expected: PASS。此处是重构前的 characterization green；若失败，说明规格与基线行为不一致，停止实施并回到 design 阶段。

- [ ] **Step 4: 写 worker 默认值和上限特征测试**

创建 `internal/app/scan_targets_test.go`，内容如下：

```go
package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunScanClampsHostWorkers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		hostWorkers int
		wantActive  int
	}{
		{name: "defaults to one", hostWorkers: 0, wantActive: 1},
		{name: "caps at live targets", hostWorkers: 99, wantActive: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := []string{"10.0.0.1", "10.0.0.2"}
			runner := newPostAliveConcurrencyRunner(targets, tc.wantActive)
			err := RunScan(context.Background(), runner, newScanStore(t), ScanOptions{
				RunID:          "run-worker-boundary",
				HostWorkers:    tc.hostWorkers,
				Targets:        []string{"10.0.0.0/30"},
				Ports:          "22",
				Tools:          ToolPaths{Rustscan: "/opt/rustscan", Nmap: "/opt/nmap"},
				JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
			})
			if err != nil {
				t.Fatalf("RunScan returned error: %v", err)
			}
			if runner.maxActive != tc.wantActive {
				t.Fatalf("max active = %d, want %d", runner.maxActive, tc.wantActive)
			}
		})
	}
}
```

- [ ] **Step 5: 运行新增边界测试和完整包测试**

Run: `rtk gofmt -w internal/app/scan_lifecycle_test.go internal/app/scan_targets_test.go && rtk go test ./internal/app -run 'TestRunScan(RecordsReportWriteFailure|ClampsHostWorkers)$' -count=1 && rtk go test ./internal/app`

Expected: 两条新增测试 PASS，随后 `internal/app` 全部 PASS。

- [ ] **Step 6: 提交特征测试**

```bash
rtk git add internal/app/scan_lifecycle_test.go internal/app/scan_targets_test.go
rtk git commit -m "test: lock scan runtime boundaries"
```

---

### Task 2: 原样移动单目标固定流水线

**Files:**
- Create: `internal/app/scan_target.go`
- Modify: `internal/app/scan.go:197-396`
- Create: `internal/app/scan_target_test.go`
- Modify: `internal/app/scan_test.go`

**Interfaces:**
- Consumes: `scanTarget(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, target string, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, error)`。
- Produces: 完全相同的私有 `scanTarget` 签名；`RunScan` 的调用点不变。

- [ ] **Step 1: 建立迁移前的单目标行为基线**

Run: `rtk go test ./internal/app -run 'Test(FindingFromNucleiUsesResultEndpoint|RunScanRunsNSEAndNucleiForSSH|RunScanRunsNSEAndNucleiForRedis|RunScanSkipsNmapWhenRustscanFindsNoOpenPorts|RunScanAddsManualReviewForRDP|RunScanLogsNmapHeartbeat|RunScanPassesExtraArgsToTools|RunScanWritesFailedNucleiOutputArtifact)$' -count=1`

Expected: PASS。

- [ ] **Step 2: 制造明确的编译红灯**

从 `internal/app/scan.go` 删除以下完整定义，但暂不创建新文件：

- `scanTarget`
- `formatNucleiEvidence`
- `findingFromNuclei`

保留 `logf`、`emit` 和 `normalizeToolError`，因为生命周期和单目标流水线都调用它们。

Run: `rtk go test ./internal/app -run '^TestRunScanRunsNSEAndNucleiForSSH$' -count=1`

Expected: FAIL to build，至少包含 `undefined: scanTarget`。

- [ ] **Step 3: 在同包新文件恢复相同实现**

创建 `internal/app/scan_target.go`：使用 `package app`；从原 `scan.go` 原样放入刚删除的三个函数，不改函数体、调用顺序、返回点、heartbeat goroutine、artifact 写入顺序或 error wrapping。只保留这些函数实际需要的 imports：

```go
package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)
```

函数在新文件中的顺序必须是 `scanTarget`、`formatNucleiEvidence`、`findingFromNuclei`。这是纯移动：用 `rtk git diff --color-moved=dimmed-zebra -- internal/app/scan.go internal/app/scan_target.go` 检查函数体只显示 moved code 和 import 调整；若函数体出现新增或删除行，恢复为基线实现。

- [ ] **Step 4: 运行单目标测试，确认绿灯**

Run: `rtk gofmt -w internal/app/scan.go internal/app/scan_target.go && rtk go test ./internal/app -run 'Test(FindingFromNucleiUsesResultEndpoint|RunScanRunsNSEAndNucleiForSSH|RunScanRunsNSEAndNucleiForRedis|RunScanSkipsNmapWhenRustscanFindsNoOpenPorts|RunScanAddsManualReviewForRDP|RunScanLogsNmapHeartbeat|RunScanPassesExtraArgsToTools|RunScanWritesFailedNucleiOutputArtifact)$' -count=1`

Expected: PASS。

- [ ] **Step 5: 按函数边界整理单目标测试文件**

创建 `internal/app/scan_target_test.go`，保持每个测试函数内容逐字不变，按当前出现顺序从 `scan_test.go` 移动以下测试：

- `TestFindingFromNucleiUsesResultEndpoint`
- `TestRunScanRunsNSEAndNucleiForSSH`
- `TestRunScanRunsNSEAndNucleiForRedis`
- `TestRunScanSkipsNmapWhenRustscanFindsNoOpenPorts`
- `TestRunScanAddsManualReviewForRDP`
- `TestRunScanLogsNmapHeartbeat`
- `TestRunScanPassesExtraArgsToTools`
- `TestRunScanWritesFailedNucleiOutputArtifact`

新文件 import 必须由 `rtk gofmt` 后的编译错误反推到最小集合；共享的 runner 类型、`aliveNmapXML`、`newScanStore` 和断言 helper 继续留在 `scan_test.go`，不新建测试框架或复制 fixture。

- [ ] **Step 6: 验证移动后没有测试丢失**

Run: `rtk gofmt -w internal/app/scan_test.go internal/app/scan_target_test.go && rtk go test ./internal/app && rtk git diff --check`

Expected: PASS；`rtk git diff --check` 无输出。

- [ ] **Step 7: 提交单目标边界**

```bash
rtk git add internal/app/scan.go internal/app/scan_target.go internal/app/scan_test.go internal/app/scan_target_test.go
rtk git commit -m "refactor: isolate scan target pipeline"
```

---

### Task 3: 提取存活探测和多目标调度

**Files:**
- Create: `internal/app/scan_targets.go`
- Modify: `internal/app/scan.go:46-188`
- Modify: `internal/app/scan_targets_test.go`
- Modify: `internal/app/scan_test.go`

**Interfaces:**
- Consumes: `scanTarget` 和当前 `ScanOptions`。
- Produces: `scanTargets(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, error)`；调用方获得已聚合的 fingerprints/findings 或与基线一致的 error。

- [ ] **Step 1: 运行调度行为基线**

Run: `rtk go test ./internal/app -run 'TestRunScan(SkipsPortScanWhenHostIsDown|UsesAliveSweepResultsAsTargets|ClampsHostWorkers|RespectsProfileHostWorkersAfterAliveSweep|ContinuesAfterTargetFailure|ReturnsErrorWhenAllTargetsFail|MarksCanceledWhenContextCanceled|MarksCanceledWhenToolIsKilledAfterCancel)$' -count=1`

Expected: PASS。

- [ ] **Step 2: 让 `RunScan` 先调用尚不存在的调度函数，验证红灯**

在 `RunScan` 中保留 artifact/run 初始化 defer，删除当前从 `scanTargets := opts.Targets` 到失败汇总结束的实现，替换为：

```go
	allFingerprints, allFindings, err := scanTargets(ctx, runner, scanStore, opts, artifactDir)
	if err != nil {
		return err
	}
```

最终报告两行保持不变。Run: `rtk go test ./internal/app -run '^TestRunScanUsesAliveSweepResultsAsTargets$' -count=1`

Expected: FAIL to build，包含 `undefined: scanTargets`。

- [ ] **Step 3: 创建最小调度边界并恢复绿灯**

创建 `internal/app/scan_targets.go`，imports、结果类型和函数签名如下：

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type targetResult struct {
	target       string
	fingerprints []fingerprint.ServiceFingerprint
	findings     []report.Finding
	err          error
}

func scanTargets(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions, artifactDir string) ([]fingerprint.ServiceFingerprint, []report.Finding, error) {
	var allFingerprints []fingerprint.ServiceFingerprint
	var allFindings []report.Finding
	scanTargets := opts.Targets
}
```

函数体取自 `rtk git show f61b7109ba37a0d85466fbd023c22568723308b1:internal/app/scan.go`：按原顺序复制该版本 `RunScan` 中从 `if opts.Tools.Nmap != "" && len(scanTargets) > 0 {` 开始、到 worker/result 归并块结束的全部语句，最后追加 `return allFingerprints, allFindings, nil`。不要复制 run 初始化、defer 或最终 report 两行。

机械迁移时只允许以下必需的返回值适配，其余语句顺序保持不变：

```go
return nil, nil, writeErr
return nil, nil, normalizeToolError(ctx, err)
return nil, nil, canceledErr
return nil, nil, fmt.Errorf("all targets failed: %w", firstErr)
```

存活探测继续使用 `opts.Targets`；`targetCh` 继续无缓冲；`results` 容量继续为 `len(scanTargets)`；worker 循环、`wg.Wait`/`close(results)` goroutine、分发 goroutine和结果 range 的相对顺序均不得改变。

- [ ] **Step 4: 运行调度测试和竞态检查**

Run: `rtk gofmt -w internal/app/scan.go internal/app/scan_targets.go && rtk go test ./internal/app -run 'TestRunScan(SkipsPortScanWhenHostIsDown|UsesAliveSweepResultsAsTargets|ClampsHostWorkers|RespectsProfileHostWorkersAfterAliveSweep|ContinuesAfterTargetFailure|ReturnsErrorWhenAllTargetsFail|MarksCanceledWhenContextCanceled|MarksCanceledWhenToolIsKilledAfterCancel)$' -count=1 && rtk go test -race ./internal/app`

Expected: 所有目标测试 PASS，race detector PASS 且不报告 data race。

- [ ] **Step 5: 把调度测试归入 `scan_targets_test.go`**

保留 Task 1 的 `TestRunScanClampsHostWorkers`，再从 `scan_test.go` 原样移动：

- `TestRunScanSkipsPortScanWhenHostIsDown`
- `TestRunScanUsesAliveSweepResultsAsTargets`
- `TestRunScanMarksCanceledWhenContextCanceled`
- `TestRunScanMarksCanceledWhenToolIsKilledAfterCancel`
- `TestRunScanRespectsProfileHostWorkersAfterAliveSweep`
- `TestRunScanContinuesAfterTargetFailure`
- `TestRunScanReturnsErrorWhenAllTargetsFail`

共享 runner fake 仍留在 `scan_test.go`；只删除 `scan_test.go` 中因测试移动而不再使用的 imports，不移动 helper。

- [ ] **Step 6: 验证调度拆分并提交**

Run: `rtk gofmt -w internal/app/scan_test.go internal/app/scan_targets_test.go && rtk go test ./internal/app && rtk go test -race ./internal/app && rtk git diff --check`

Expected: 两次测试 PASS，race detector 无报告，diff check 无输出。

```bash
rtk git add internal/app/scan.go internal/app/scan_targets.go internal/app/scan_test.go internal/app/scan_targets_test.go
rtk git commit -m "refactor: isolate scan target scheduling"
```

---

### Task 4: 收敛生命周期入口并完成测试归档

**Files:**
- Modify: `internal/app/scan.go`
- Modify: `internal/app/scan_lifecycle_test.go`
- Modify: `internal/app/scan_test.go`
- Verify only: `cmd/anchorscan/main.go`, `internal/app/manager.go`

**Interfaces:**
- Consumes: Task 3 的 `scanTargets` 三值返回。
- Produces: 保持原签名的 `RunScan`；该函数只拥有默认 profile、artifact 目录、run 初始化/defer、调度调用和报告提交。

- [ ] **Step 1: 确认 `RunScan` 最终结构**

整理 import 后，`RunScan` 的控制流必须等价于以下完整结构；不要为初始化或报告提交再提取 helper：

```go
func RunScan(ctx context.Context, runner tools.Runner, scanStore *store.Store, opts ScanOptions) (runErr error) {
	artifactDir := ""
	if opts.ProfileName == "" {
		opts.ProfileName = "normal"
	}
	if opts.RunID != "" && strings.TrimSpace(opts.ArtifactRoot) != "" {
		artifactDir = filepath.Join(opts.ArtifactRoot, opts.RunID)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return err
		}
	}
	if opts.RunID != "" && scanStore != nil {
		_ = scanStore.SaveScanRun(store.ScanRun{
			RunID: opts.RunID, ProjectID: opts.ProjectID,
			Target: strings.Join(opts.Targets, ","), Ports: opts.Ports,
			Profile: opts.ProfileName, Status: "running", StartedAt: time.Now(),
			ConfigSnapshot: opts.ConfigSnapshot, ArtifactDir: artifactDir,
		})
	}
	defer func() {
		if opts.RunID == "" || scanStore == nil {
			return
		}
		status, message := "completed", ""
		if runErr != nil {
			status, message = "failed", runErr.Error()
			if errors.Is(runErr, context.Canceled) {
				status = "canceled"
			}
		}
		_ = scanStore.UpdateScanRunStatus(opts.RunID, status, message, time.Now())
	}()

	allFingerprints, allFindings, err := scanTargets(ctx, runner, scanStore, opts, artifactDir)
	if err != nil {
		return err
	}
	emit(opts, scanStore, "info", "report", "report json %s", opts.JSONReportPath)
	return report.WriteJSON(opts.JSONReportPath, report.Build(allFingerprints, allFindings))
}
```

字段可以由 `gofmt` 展开，但赋值值、defer 时机和返回顺序不得变化。

- [ ] **Step 2: 将生命周期测试归入对应文件**

保留 Task 1 的 `TestRunScanRecordsReportWriteFailure`，从 `scan_test.go` 原样移动：

- `TestRunScanPersistsRunLifecycleAndEvents`

取消测试继续归 `scan_targets_test.go`，因为它们锁定分发和结果归并取消语义。以下组合测试继续留在 `scan_test.go`，不再拆分：

- `TestScanOptionsIncludesTask2MetadataFields`
- `TestRunScanStoresFingerprintAndWritesJSONReport`
- `TestRunScanSkipsPortScanWhenHostIsDown` 若 Task 3 已移动则不重复
- `TestRunScanWritesAuditArtifacts`

- [ ] **Step 3: 检查公开入口调用方没有变化**

Run: `rtk git diff --exit-code f61b7109ba37a0d85466fbd023c22568723308b1 -- cmd/anchorscan/main.go internal/app/manager.go`

Expected: 无输出，exit 0。

- [ ] **Step 4: 运行生命周期和包级测试**

Run: `rtk gofmt -w internal/app/scan.go internal/app/scan_test.go internal/app/scan_lifecycle_test.go && rtk go test ./internal/app -run 'TestRunScan(PersistsRunLifecycleAndEvents|RecordsReportWriteFailure|StoresFingerprintAndWritesJSONReport|WritesAuditArtifacts)$' -count=1 && rtk go test ./internal/app`

Expected: PASS。

- [ ] **Step 5: 提交生命周期收敛**

```bash
rtk git add internal/app/scan.go internal/app/scan_test.go internal/app/scan_lifecycle_test.go
rtk git commit -m "refactor: narrow scan lifecycle entrypoint"
```

---

### Task 5: 全仓兼容性验证

**Files:**
- Verify: `internal/app/scan.go`
- Verify: `internal/app/scan_targets.go`
- Verify: `internal/app/scan_target.go`
- Verify: `internal/app/*scan*_test.go`
- Verify: `go.mod`, `go.sum`, `cmd/anchorscan/main.go`, `internal/app/manager.go`

**Interfaces:**
- Consumes: 前四个任务的三个内部边界。
- Produces: 可进入 Comet verify 阶段的测试、竞态和打包证据。

- [ ] **Step 1: 检查范围和依赖没有扩大**

Run: `rtk git diff --exit-code f61b7109ba37a0d85466fbd023c22568723308b1 -- go.mod go.sum cmd/anchorscan/main.go internal/app/manager.go`

Expected: 无输出，exit 0。

Run: `rtk git diff --name-only f61b7109ba37a0d85466fbd023c22568723308b1`

Expected: 只包含本计划列出的 `internal/app` 生产/测试文件及 Comet 执行过程中更新的 change 状态文件；不得出现 CLI、Web、schema、配置或依赖文件。

- [ ] **Step 2: 运行格式、静态和扫描包检查**

Run: `rtk test -z "$(rtk gofmt -l internal/app/scan.go internal/app/scan_targets.go internal/app/scan_target.go internal/app/scan_test.go internal/app/scan_lifecycle_test.go internal/app/scan_targets_test.go internal/app/scan_target_test.go)" && rtk go vet ./internal/app && rtk go test -race ./internal/app`

Expected: 无格式文件输出；`rtk go vet` exit 0；测试 PASS 且无 data race。

- [ ] **Step 3: 运行全仓测试**

Run: `rtk make test`

Expected: Makefile 内部执行的 `rtk go test ./...` PASS，`rtk node --test internal/web/static/app.test.mjs` 全部 PASS。

- [ ] **Step 4: 验证发布打包路径**

Run: `rtk make clean && VERSION=v1.7.1-plan-check rtk make package`

Expected: exit 0，并生成当前平台的 `dist/anchorscan-v1.7.1-plan-check-<goos>-<goarch>/anchorscan` 与对应 `.tar.gz`。验证后运行 `rtk make clean`，避免把构建产物带入提交。

- [ ] **Step 5: 检查最终 diff**

Run: `rtk git diff --check && rtk git status --short`

Expected: `rtk git diff --check` 无输出；状态只显示本 change 的计划内文件或 Comet 状态文件，没有 `dist/`。

- [ ] **Step 6: 若验证阶段产生修正则提交，否则不制造空提交**

若 Step 1-5 发现并修复了计划范围内问题：

```bash
rtk git add internal/app/scan.go internal/app/scan_targets.go internal/app/scan_target.go internal/app/scan_test.go internal/app/scan_lifecycle_test.go internal/app/scan_targets_test.go internal/app/scan_target_test.go
rtk git commit -m "test: verify scan runtime decomposition"
```

若没有代码修正，跳过提交，直接进入 `/comet-verify decompose-scan-runtime`。

## Plan Self-Review

- Spec coverage: Task 1 固定生命周期和 worker 缺口；Task 2 覆盖固定工具流水线、事件、heartbeat 与 artifact；Task 3 覆盖存活探测、并发、取消和失败优先级；Task 4 保持公开入口和最终状态；Task 5 覆盖全仓、race、调用方、依赖和打包。
- Placeholder scan: 所有生产符号、文件、迁移函数清单、命令和预期结果均已明确；机械迁移只引用当前基线中的精确函数或精确行块，不引入未定义行为。
- Type consistency: `scanTargets` 的签名在 Task 3 和 Task 4 一致；`scanTarget` 保持基线签名；两者都复用 `ScanOptions`、`tools.Runner`、`*store.Store`、fingerprint/report 类型。
- Ponytail check: 只新增两个生产文件和三个按设计要求的测试文件；没有新增依赖、接口、子包、context 对象或通用 helper。
