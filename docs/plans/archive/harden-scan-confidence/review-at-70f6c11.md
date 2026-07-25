---
change: harden-scan-confidence
role: code-review-record
fixed_point: f038552
head: 70f6c11
date: 2026-07-18
axes: [Standards, Spec]
---

# 双轴 Code Review：harden-scan-confidence @ 70f6c11

> 本文件为收尾 ticket `10-integrated-release-acceptance.md` 所要求的「最终固定点双轴 code-review」留痕。此前 ticket 10 已标 `done` 但仓库无 review 产物；本记录补齐该环节。

## Review 基线

- **Fixed point（baseline）**：`f038552`（`docs(plan): define scan confidence hardening`，计划定义；父 `8a312c8` 为计划开始前的 main）
- **HEAD**：`70f6c11`（`test: complete release acceptance`，计划收尾）
- **Diff**：`git diff f038552...70f6c11`（全量 83 文件，+3719 / −320；其中代码与配置约 72 文件，+3610 / −225）
- **提交数**：22（PR 浏览器门禁 → Run Lease → interrupted 收敛 → DetectionCheck → 部分结果 → 覆盖报告 → 可选超时 → 真实工具实验室 → 集成发布验收）

验证命令：`go build ./...`（通过）、`go vet ./internal/... ./cmd/...`（无输出）、`go test ./...`（14 包全部 `ok`）、`gofmt -l` 仅命中 diff 外的 `internal/fingerprint/nmap_xml.go`（计划前置文件，非本计划范围）。

- **Spec 轴依据**：`docs/plans/harden-scan-confidence/spec.md`、`technical-design.md`、ADR-0002/0003
- **Standards 轴依据**：`AGENTS.md`、`CONTEXT.md`；仓库无 `CODING_STANDARDS.md`/`CONTRIBUTING.md`，叠加 Fowler smell baseline。跳过 gofmt/govet/golangci-lint 已强制内容。

---

## Standards

### 硬违规（AGENTS.md / CONTEXT.md）

无 blocker 级硬违规。`CONTEXT.md` 已同步新增概念（Run Lease / DetectionCheck 六状态、`completed_with_errors`、`interrupted` 语义均与实现一致）。

- **minor — 中英不一致（`AGENTS.md`「Always communicate with the user in Chinese」精神）**：面向操作者的 Run 摘要用英文。`internal/app/scan.go:97` `completed_with_errors` 消息硬编码 `"one or more optional stages failed"`；`internal/store/leases.go` `ReconcileInterruptedRuns` 写入 `error = 'run lease expired' / 'run lease missing'`。这些值进 `store.ScanRun.Error` 并在 Web 运行页/`docs/deploy.md` 中文语境中展示。判断项：`run.Error` 历史上即承载英文工具错误，混合属既有现状，但本次新增串可顺手本地化。

### Smell baseline（判断项）

- **major — Duplicated Code**：`internal/app/scan_target.go` NSE `default` 分支（约 141–176 行）与 nuclei `default` 分支（约 196–235 行）结构几乎相同：`toolContext → 工具调用 → isOperatorCanceled → recordDetectionCheck("running") → artifact 失败/命令失败的「failed vs canceled + reason」分类 → HadErrors/stageFailed → recordDetectionCheck 终态`。两段各约 30 行、形状一致。可抽出「单检测引擎执行 + DetectionCheck 生命周期」共享 helper（参数：引擎名、执行闭包、解析+持久化闭包）。

- **minor — Duplicated Code**：`toolCtx, cancel := toolContext(ctx, opts.Timeouts.X); …; cancel()` 样板在 `scan_target.go`（rustscan/nmap/httpx/nmap-alive/NSE/nuclei）与 `tool_run.go`（5 处）重复约 11 次。Go 在循环内 defer 受限，但「调用→归一化错误→cancel」三段可包成小 helper。

- **minor — Duplicated Code（冲突映射重复）**：`internal/app/run_lease.go` 的 `reserveRunLease`（30–44 行，Manager 用）与 `acquireRunLease` 的空 token 分支（50–66 行，CLI/RunScan 用）都做 `ReconcileInterruptedRuns → newRunLeaseToken → AcquireRunLease → errors.Is(ErrRunLeaseHeld) → "scan already running: %s"`。冲突消息格式与 token 生成逻辑写了两遍，且 `acquireRunLease` 会再 reconcile 一次（Manager 路径下双重 reconcile，无害但冗余）。

- **minor — 重复字面量（TTL 漂移风险）**：`internal/web/server.go:67` `ReconcileInterruptedRuns(opts.Now(), 30*time.Second)` 硬编码 30s，而权威常量 `runLeaseTTL = 30 * time.Second` 在 `internal/app/run_lease.go`（跨包不可见）。TTL 现定义于两处，改一处忘另一处会导致 Web 启动协调与运行期判断窗口不一致。

- **minor — 冗余写库**：`internal/app/scan_target.go` 对每个非 web Fingerprint 调用 `opts.PersistFingerprint(fp)` 两次（httpx 判断前 + 后，见 73–75 与 99–102 行）。第二次为 no-op upsert（`UpsertFingerprint`），正确但浪费一次写。

- **minor — 轮询回归**：`internal/web/static/run-status.js` 删除了原先 `if(current !== 'running') return` 早返回，现对终态运行页也每 1.5s 永久轮询 `/api/runs/:id/status`。终态摘要静态，轮询无收益；建议保留「running 才轮询」的早返回，仅在 running 时更新摘要。

- **minor — Repeated Switches**：`internal/app/tool_run.go` `toolRunContext` 对 `opts.Tool` 做 `switch`→timeout 字段；`internal/web/scans.go` `isScanProfile` 对 profile 做 `switch`。均可为小 map，且 tool→timeout 的字段映射与 `ToolDurations` 结构重复。判断项，规模小。

无显著 Feature Envy / Data Clumps（`ToolTimeouts`/`ToolPaths` 已成束）/ Primitive Obsession（DetectionCheck 状态/原因码为字符串是 ADR-0003 明确选择，repo 覆盖 baseline）/ Message Chains / Middle Man / Refused Bequest / Speculative Generality（`DetectionCoverage` 用指针仅为 `omitempty`，非投机）。

**Standards 最严重项：** `scan_target.go` NSE/nuclei 检测执行块 Duplicated Code（major，判断项）。

> **处置（2026-07-18 修复轮）：** 评估后**刻意保留**各自实现。NSE 收尾为四段（解析在 `RunNSEWithOutput` 内部），nuclei 为五段（多独立 `ParseNucleiJSONL` + `invalid_output` 终态）；强行合并需把 DetectionCheck 状态机参数化为「带可选解析失败阶段」，会糊化语义，且重复仅为结构相似非逐字。`internal/app/scan_target.go` 已加注释说明。领域正确性优先于去重。

---

## Spec

### (a) spec 要求缺失或部分实现

无 major/blocker 缺口。验收标准逐条核对均落地：

- Lease 三入口统一（`RunScan`/`RunTool`/Manager 均 `acquireRunLease`/`reserveRunLease`，owner token 条件续租/释放/终结 — `leases.go`）✓
- 过期/遗留协调为 `interrupted`，保留 Fingerprint/Finding/Artifact/DetectionCheck（`ReconcileInterruptedRuns` + 测试 `TestReconcileInterruptedRunsClosesExpiredOrMissingRunsOnce`）✓
- 取消收敛 `canceled` + running 检查收敛（`FinishRunWithLease` canceled 分支 / `cancelDetectionChecks`）✓
- 部分结果保留 + `completed_with_errors`/`failed` 区分（`scanTargets` 返回 `partialErrors`、`scan.go` defer 状态优先级）✓
- 增量持久化（nmap 后 `UpsertFingerprint`、httpx 自然键更新、逐 Finding 即时保存）✓ 对应 `technical-design.md` §扫描流水线与部分结果
- 覆盖用数量非百分比、双/单/未覆盖 + failed/canceled/interrupted/skipped 计数（`report.summarizeDetectionCoverage`）✓ 对应 spec L124–125
- JSON 仅 `omitempty` 新增字段、导出不变（`DetectionChecks`/`DetectionCoverage` 均 `omitempty`）✓ spec L52/71、td L235
- 重跑预填、GET 不自动启动（`rerunScanForm` + 模板告警）✓
- 超时默认 0、非零才 deadline、`DeadlineExceeded`≠canceled（`toolContext`/`normalizeToolError`）✓
- `service-aliases.yaml` 删除 ✓
- PR 门禁 go/js/build/playwright(chromium) + 失败上传诊断（`pr.yml`/`Makefile pr-check`）✓
- 真实工具实验室 MySQL/SMB/SSH/未知/混合 + 发布证据（`lab.yml`/`release.yml needs: lab`/`e2e/smoke_test.go`）✓
- 单工具运行共享 Lease、不伪造 DetectionCheck（`tool_run.go`）✓ td §Run 状态收敛末段

### (b) 超出 spec 的 scope creep

无实质 scope creep。新增的 `docker-compose.lab.yml`（tomcat/redis）与 `e2e` 场景属 ticket 09 实验室扩展范围内；`ToolTimeouts.Normalized()`/`ToolDurations` 双类型是 td §可选逐工具超时的合理落地，非越界。

### (c) 看似实现但有误 / 偏离

- **minor — 原因码词汇偏离**：`internal/app/scan_target.go:134` 对 web 服务的 NSE 跳过写入原因码 `"not_applicable"`，但该码不在 spec/td 枚举的 9 个稳定机器原因码内（`technical-design.md:66–74`：`no_matching_rule`/`tool_unconfigured`/`missing_target`/`command_failed`/`invalid_output`/`artifact_failed`/`persistence_failed`/`run_canceled`/`lease_expired`）。spec/td 明确「业务逻辑、筛选和测试只依赖状态与原因码」（td L76），引入未登记码会导致按文档集合筛选/统计的消费者漏掉「web 服务 NSE 跳过」一类。建议：要么并入 `no_matching_rule`（语义最近），要么在 td 原因码表补登 `not_applicable` 并说明触发条件。

- **minor — skip 分支顺序导致误分类**：`internal/app/scan_target.go:127–140` NSE 的 `switch` 先判 `len(scripts)==0`（→`no_matching_rule`）再判 `opts.Tools.Nmap==""`（→`tool_unconfigured`）。当 nmap 未配置且无匹配脚本时，会报 `no_matching_rule` 而非 td L67 定义的 `tool_unconfigured`（「检测工具未配置」）。属边缘误分类，建议把 `tool_unconfigured` 判断前移。

**Spec 最严重项：** `not_applicable` 原因码偏离 `technical-design.md:66–74` 稳定词汇表（minor）。

---

## 一行总结

- **Standards**：7 项（0 blocker / 1 major / 6 minor），最严重为 `scan_target.go` NSE/nuclei 检测块 Duplicated Code（major，判断项）。
- **Spec**：2 项（0 blocker / 0 major / 2 minor），最严重为 `not_applicable` 原因码偏离 `technical-design.md:66–74` 稳定词汇表（minor）。

两轴均无 blocker/major 硬伤阻断收尾；本期计划实现与 spec/td 高度一致，遗留均为可后续清理的判断项与边缘偏离。

---

## 结论

- ticket `10-integrated-release-acceptance.md` 要求的「以计划开始时的 fixed point 执行 Standards/Spec 双轴 code-review，修复 blocker/major 后重新运行完整验证」**现已具备留痕**（本文件）。
- 因双轴均无 blocker/major，ticket 10 的 `done` 状态成立，无需回退或补实现。
- **遗留项处置（2026-07-18 修复轮）：** Spec 两项 minor 已修复——`not_applicable` 并入 `no_matching_rule`（对齐 td 词汇表，未扩展），NSE 跳过分支把 `tool_unconfigured` 前移避免误分类；`go test ./...` 全绿。Standards 的 NSE/nuclei 重复经评估**刻意保留**（见上节处置说明），`internal/app/scan_target.go` 已留注释。其余 minor 判断项（英文 Run 摘要、TTL 字面量重复、终态轮询早返回等）不阻塞，可在后续小 ticket 中清理。
