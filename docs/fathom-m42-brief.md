# 任务书：fathom 集成 M4.2 — scan_target 前段切换为 fathom 唯一路径

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。
> Spec：`docs/plans/fathom-integration/spec.md`（v2.0，设计反转：fathom 唯一路径，不保留 legacy 回退）

## 背景

**设计目的（用户原话）**：fathom 包揽了主机存活探测、端口扫描、服务识别和高危漏洞探测，减少了数据多工具之间流动问题。切换是为了降低复杂度。保留回退 = 维护两套路径 = 复杂度翻倍 = 违背初衷。

**fathom 成为唯一扫描路径**。`scan_target` 前段（rustscan 端口发现 + nmap -sV 指纹）整体替换为一次 `RunFathomScan` 调用。fathom 未配置 → preflight 直接报错退出，不回退、不降级。

## 必读

1. `AGENTS.md`（仓库根）
2. `docs/plans/fathom-integration/spec.md`（v2.0）
3. `internal/app/scan_target.go`：当前流水线（重点第 37-90 行 rustscan+nmap 前段，第 90+ 行后段 httpx/NSE/nuclei/达梦/rdpscan 保留）
4. `internal/tools/fathom.go`：M4.1 已实现的 `RunFathomScan`（签名 `RunFathomScan(ctx, runner, binary, ip, ports)` → `FathomScanResult{Fingerprints, Findings, Checks, RawJSONL}`）
5. `internal/tools/fathom_test.go`：M4.1 测试（JSONL 解析、归一化、Finding 映射、TLS 预留）
6. `internal/preflight/preflight.go`：工具配置检查（加 fathom 必填校验）
7. `internal/app/scan_prepare.go`：`PrepareScan`（ScanOptions 构造，需传 fathom binary/timeout）

## Scope

**要做**：

### 1. scan_target 前段替换
- `scanTarget` 第 37-90 行（rustscan 发现端口 → nmap -sV 指纹）替换为：
  - `progress.Emit("info", "fathom", "fathom %s ports=%s", target, opts.Ports)`
  - 解析 opts.Ports → `[]int`（复用 internal/ports 的解析逻辑）
  - `fathomResult, out, err := tools.RunFathomScan(ctx, runner, opts.Tools.Fathom, target, ports)`（带 timeout + artifact 落盘）
  - fathom JSONL 落 `artifactDir/safeArtifactName("fathom", target)+".jsonl"`
  - fathom 错误处理（同现有 rustscan/nmap 错误模式：normalizeToolError、operatorCanceled）
  - `fathomResult.Fingerprints` → 后段循环（复用现有第 90+ 行 `for _, fp := range fingerprints` 逻辑）
  - `fathomResult.Findings`（fathom checks → high severity Finding）持久化
  - `fathomResult.Checks`（DetectionCheck engine="fathom"）持久化
  - 无开放端口时（fathom 输出空）提前返回，同现有逻辑

### 2. fathom checks 的 DetectionCheck 持久化
- 每个 fathom check（verdict=vulnerable/safe/unknown）记一条 DetectionCheck
- engine 字段 = `"fathom"`，status 按 verdict 映射（vulnerable→"completed"，safe→"completed"，unknown→"completed"）
- reason/output 记 proof 摘要

### 3. 达梦衔接（spec 决策 4）
- fathom 指纹的 service="dameng" 时**跳过 nuclei dameng-identify**（现有第 120-157 行的 dameng-identify 块）
- fathom 已通过协议握手识别达梦（权威性等同 nuclei dameng-detect），直接进入后续默认口令检查
- fathom 未检出 dameng 的指纹仍走现有 nuclei dameng-identify 路径（nuclei 仍是达梦协议权威之一，只是 fathom 检出时跳过）

### 4. TLS web 增强（spec 决策 2）
- M4.1 已预留 `NeedsTLSWebEnhancement(fp)` 和 `TLSWebCandidatePorts`
- 在后段循环中：`fp.IsWeb` 为 false 但 `NeedsTLSWebEnhancement(fp)` 为 true 时，仍触发 httpx 增强（httpx 能做 TLS handshake）
- httpx 成功 → 更新 fp.IsWeb / fp.URL（同现有 httpx 逻辑）

### 5. preflight 必填校验
- `internal/preflight/preflight.go`：fathom 路径为空时产生 **error**（不是 warning）："fathom is required but not configured. Set tools.fathom in config."
- rustscan 从 preflight 的**必填**检查中移除（改为 optional 或删除——扫描路径不再调用它）
- nmap 仍必填（NSE 引擎角色）

### 6. ScanOptions 传递
- `scan_prepare.go`：确保 `opts.Tools.Fathom` 和 `opts.Timeouts.Fathom` 正确传入 ScanOptions
- `tools.DiscoverPortsWithOutput` / `tools.FingerprintWithOutput`（rustscan/nmap 前段）的调用从 scan_target 移除，但**不删除** internal/tools 中的函数定义（M4.3 统一清理）

### 7. 后段不变
- httpx / NSE / nuclei / rdpscan / 默认口令检查的逻辑保持不变
- 指纹来源从 nmap XML 变为 fathom JSONL，但 ServiceFingerprint 结构不变（M4.1 归一化已对齐）

**不要做**：
- 不删除 `internal/tools/` 中的 rustscan/nmap 相关函数（M4.3 清理）
- 不删除 rustscan/nmap 的 config 字段（M4.3 决定）
- 不做 git 操作（commit/push/checkout 一律禁止）
- 不引入新依赖
- 不实现 fathom discover 段、IPv6

## 铁律

1. 零新依赖；改动限于 internal/app（scan_target.go, scan_prepare.go）+ internal/preflight + 必要测试
2. **fathom 是唯一路径**：不得保留任何 rustscan/nmap-sV 回退分支；fathom 未配置 = preflight error = 不启动扫描
3. 诚实报告：报告分列「实测」与「静态推断」；lab 真实验证如不可行，说明原因
4. 后段（httpx/NSE/nuclei/达梦/rdpscan）行为不变——fathom 只替代前段（存活+端口+指纹）
5. 已知未跟踪文件（spikes/ 等）为既往产物，不得删除或修改，无需确认
6. 完成后不得自行 commit；报告文件：`docs/reports/fathom-m42-report.md`

## 验收

1. `go build ./...` 通过
2. `go test ./internal/app/ ./internal/tools/ ./internal/preflight/ -count=1` 全过
3. scan_target 集成测试：fake Runner 返回 fathom JSONL fixture → 指纹正确进入后段（httpx/NSE/nuclei）
4. fathom 未配置时 preflight 返回 error（非 warning）
5. 达梦 fathom 检出时跳过 nuclei dameng-identify（测试覆盖）
6. TLS web 增强触发（NeedsTLSWebEnhancement → httpx 调用）
7. 报告含：改动文件清单、前段替换前后对比、达梦/TLS 衔接说明、遗留风险（M4.3 清单）
