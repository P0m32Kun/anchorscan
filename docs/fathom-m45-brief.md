# 任务书：fathom M4.5 — 彻底删除 rustscan + IPv6 legacy 路径

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。
> Spec：`docs/plans/fathom-integration/spec.md`（v2.0，fathom 唯一路径）
> 前置：M4.1-M4.4 已合并。fathom 已是唯一扫描引擎（存活+端口+指纹+高危检测）。

## 背景（用户决策 2026-08-07）

用户明确要求彻底移除两个 legacy 残留：
1. **nmap IPv6 存活探测（`-sn` legacy 路径）**：fathom IPv4-only，IPv6 不支持就不做——IPv6 target 直接报错（"fathom does not support IPv6"），不再保留 nmap -sn 兜底。
2. **rustscan 单工具执行模式**：fathom 已完全替代 rustscan（端口发现），单工具页面的 rustscan 执行模式也删除。

目的延续 fathom 集成主线：**减少工具数量、降低复杂度**。不再保留任何 rustscan 相关代码路径。

## 必读

1. `AGENTS.md`（仓库根）
2. `docs/plans/fathom-integration/spec.md`（v2.0）
3. 以下文件先全量读一遍再动手（删除范围广，禁止漏删或误删）：
   - `internal/app/tool_run.go`（runRustscanTool、rustscan case、timeout 映射）
   - `internal/tools/rustscan.go` + `internal/tools/rustscan_test.go`
   - `internal/tools/nmap.go`（CheckAlive、DiscoverAliveInScopeWithOutput、-sn 相关）
   - `internal/app/scan_targets.go`（IPv6 分支、RequiresNmapDiscovery）
   - `internal/target/scope.go`（RequiresNmapDiscovery、IsIPv6、NmapTargets/NmapExcludes）
   - `internal/config/config.go` + `init.go`（rustscan 字段、timeouts.rustscan）
   - `internal/web/config.go` + `templates/config.html`（rustscan 输入框/超时）
   - `internal/web/tools.go` + `templates/tool_page.html`（rustscan 工具页）
   - `internal/doctor/doctor.go`（rustscan check）
   - `cmd/anchorscan/scan_command.go` + `tool_command.go` + `admin_command.go`（rustscan 相关 flag/校验）
   - `internal/preflight/preflight.go`（rustscan optional check）
   - `internal/app/provenance.go`（rustscan 引用）

## Scope

### 要做

**A. 删除 rustscan 单工具执行模式（完整删除，不留死代码）**
1. `internal/app/tool_run.go`：删 `runRustscanTool`、`case "rustscan"`（工具分派 + binary 选择 + timeout 映射 + extra args 校验）
2. `internal/tools/rustscan.go` + `rustscan_test.go`：整文件删除（`DiscoverPorts`/`DiscoverPortsWithOutput` 若无其他调用方）
3. Web：`/tools/rustscan` 路由/处理、`tool_page.html` 的 rustscan 分支、`config.html` 的 rustscan 输入框 + timeout_rustscan 档、`internal/web/config.go` 的 rustscan 字段读写
4. Config：`config.go` 的 `Rustscan` 字段、`init.go` 的默认值/PATH 探测、`timeouts.Rustscan`、`config/default.yaml.example` 的 rustscan 行
5. Doctor：`doctor.go` 的 `toolCheck("rustscan", ...)`
6. Preflight：`preflight.go` 的 rustscan optional check
7. CLI：`scan_command.go`/`tool_command.go`/`admin_command.go` 的 rustscan flag、帮助文本
8. `provenance.go` 及测试：rustscan 引用清理
9. 所有测试文件中的 rustscan fixture/mock 清理（`scan_prepare_test.go`、`manager_test.go`、`run_lease_test.go`、`scan_lifecycle_test.go`、`progress_test.go`、`scan_target_test.go`、`tool_run_test.go`、web/cmd 各测试）
10. 文档：CHANGELOG、README、CONTEXT、project-status、deploy、testing-lab-checklist、config 注释

**B. 删除 nmap IPv6 存活探测 legacy**
1. `scan_targets.go`：IPv6 分支删除——IPv6 target 直接报错（明确错误信息，如 "fathom does not support IPv6 targets"）；IPv4 路径不变
2. `scope.go`：`RequiresNmapDiscovery`/`IsIPv6` 相关检查——若不再被需要则清理（注意 `IsIPv6` 可能被其他地方用，grep 确认）
3. `nmap.go`：`CheckAlive`（单工具模式 nmap alive 也用？grep 确认 tool_run.go 的 nmap 单工具是否依赖）、`DiscoverAliveInScopeWithOutput`、`-sn` 相关——**若 IPv6 删除后无调用方则删除**，但 nmap 的 `-sV`/NSE 相关保留（NSE 引擎角色 + 单工具模式）
4. preflight：nmap 必需性检查改为"nmap 用于 NSE 引擎"（不再有 IPv6 discovery 语义）
5. 文档同步

**注意**：nmap 本身不删（NSE 引擎 + 单工具模式仍用 `-sV`/NSE/`--script`）。只删 `-sn` 存活探测相关。`tools.Fingerprint`/`RunNSEWithOutput`/`RunNmapTool` 保留。

### 不要做
- 不删 nmap（NSE 引擎 + 单工具 nmap 模式保留）
- 不删 httpx/nuclei/rdpscan/dameng
- 不删 `RunFathomScan` 及 fathom 相关
- 不改 scan_target.go 的后段（httpx/NSE/nuclei/达梦）逻辑
- 不做 git 操作
- 不引入新依赖

## 铁律

1. 零新依赖
2. **完整删除，不留死代码**：删除函数/字段后必须 grep 确认无残留引用（`grep -rn "rustscan" internal/ cmd/ config/` 应为空或仅文档提及）
3. 诚实报告：报告分列「实测」与「静态推断」
4. 已知未跟踪文件（spikes/ 等）不得删除或修改
5. 完成后不得自行 commit（编排方会核对 git log）
6. 报告文件：`docs/reports/fathom-m45-report.md`

## 验收

1. `go build ./...` 通过
2. `go test ./... -count=1` 全过
3. `make web-smoke` 通过
4. `grep -rn "rustscan" internal/ cmd/ config/`（非文档文件）→ **零残留**（工具路径 config 字段/注释允许在 `config/default.yaml.example` 里保留？**不**——用户要求彻底删除，config 示例也删 rustscan 行）
5. IPv6 target → 明确报错（测试覆盖）
6. nmap `-sn` 相关函数（CheckAlive/DiscoverAliveInScopeWithOutput）若无调用方 → 已删除
7. 报告 `docs/reports/fathom-m45-report.md`
