# 任务书：fathom M4.2 续派 — 在现有成果上补缺验收

> 本任务书由编排方（Hermes）下达。前两轮已完成大部分实现（10 个文件改动，go build 通过），
> 但 Hermes 意外退出杀死了 pi 进程。当前状态：**9 个测试 FAIL**（旧测试基于 nmap XML fixture，
> 需适配为 fathom JSONL fixture），报告未写。

## 当前状态（编排方已验证 2026-08-07）

- `go build ./...` ✅ 通过
- 已改动 10 个文件：scan_target.go、scan_prepare.go、scan_target_test.go、scan_targets_test.go、
  scan_test.go、scan_prepare_test.go、preflight.go、preflight_test.go、report/model.go、fathom.go
- **9 个测试 FAIL**（全是旧测试期望 rustscan+nmap 调用，现在走 fathom 路径）：
  - TestRunScanTriggersDamengFinding
  - TestRunScanRecordsDamengPanicAsCompletedWithErrors
  - TestRunScanRecordsDamengDeadlineAsCompletedWithErrors
  - TestRunScanSkipsDamengWhenToolUnconfigured
  - TestRunScanSkipsDamengWhenNoMatchingRule
  - TestScanTargetReturnsFingerprintsAndOpenPorts
  - TestScanTargetRecordsNSESkipReasons（含子测试）
  - TestScanTargetDamengNucleiGate

## 你要做

1. 阅读 `docs/fathom-m42-brief.md`（原始任务书，验收标准以此为准）
2. **修复 9 个 FAIL 测试**：这些测试的 fixture 基于 rustscan 端口发现 + nmap XML 指纹，
   现在前段改为 fathom JSONL。需要把 fake Runner 的 mock 输出从 rustscan/nmap 格式
   改为 fathom JSONL 格式（参考 internal/tools/fathom_test.go 的 fixture）。
   **不是改测试逻辑，是改测试的 mock 数据源**——断言的语义行为不变。
3. 跑完整验收：`go build ./...` + `go test ./internal/app/ ./internal/tools/ ./internal/preflight/ -count=1`
4. 写报告 `docs/reports/fathom-m42-report.md`

## 铁律

1. **在现有代码上补缺，不推倒重写**——已有改动经过编排方确认 go build 通过
2. 不做 git 操作（commit/push/checkout 一律禁止）
3. fathom 是唯一路径，不保留 legacy 回退
4. 报告分列「实测」与「静态推断」
5. 已知未跟踪文件（spikes/ 等）不得删除或修改
6. 完成后编排方会核对 git log，禁止任何提交

## 验收（同原始 brief）

1. `go build ./...` 通过
2. `go test ./internal/app/ ./internal/tools/ ./internal/preflight/ -count=1` 全过
3. scan_target 集成测试覆盖 fathom JSONL → 指纹 → 后段衔接
4. fathom 未配置时 preflight 返回 error
5. 达梦 fathom 检出时跳过 nuclei dameng-identify
6. TLS web 增强触发
7. 报告 `docs/reports/fathom-m42-report.md`
