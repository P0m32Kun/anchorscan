# 任务书：fathom 集成 M4.4 — 存活探测切换（移除 nmap -sn）

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。
> Spec：`docs/plans/fathom-integration/spec.md`（v2.0）
> 前置：M4.1-M4.3 已合并。fathom 已是 scan_target 内唯一端口/指纹引擎。

## 背景

M4.3 报告诚实披露：**nmap `-sn` 存活扫描仍在 scan_targets.go 外层**（第 36-63 行 `DiscoverAliveInScopeWithOutput`），与 fathom 内置 alive 探测形成**双重存活探测**。fathom 的 `scan` 命令天然内置 alive 探测（`alive::find` → ICMP Datagram/Raw 分级 + TCP 回退 80/443/445/22），完全覆盖 nmap `-sn` 职责。本次将外层 nmap -sn 移除，让 fathom 承担全部存活探测。

## 必读

1. `AGENTS.md`（仓库根）
2. `docs/plans/fathom-integration/spec.md`（v2.0）
3. `internal/app/scan_targets.go`：第 20-68 行（存活探测段），第 31 行（`RequiresNmapDiscovery` 检查）
4. `internal/target/scope.go`：`RequiresNmapDiscovery`（213 行）、`IsIPv6`（222 行）、`NmapTargets`/`NmapExcludes`（226-228 行）、`Addresses`（233 行）
5. `internal/app/discovery.go`：DiscoveryMode（auto / assume-up）
6. `internal/tools/fathom.go`：`RunFathomScan`（M4.1，单 IP 调用 `fathom scan --json <ip> -p <ports>`）
7. `internal/tools/nmap.go`：`CheckAlive`（135 行）、`DiscoverAliveInScopeWithOutput`（被替换对象）
8. **fathom 仓库 `~/DEV/fathom`**（只读）：
   - `src/alive.rs`：`probe()` 322 行（ICMP + TCP 回退）、`find()` 333 行（64 并发）
   - `src/targets.rs`：`Targets::parse`（CIDR 展开内部化）、`TargetIter`（无 exclude 参数）
   - `src/main.rs`：`scan` 参数（`--json`/`--no-icmp`/`--no-checks`/`-p`/`-l`），**无"跳过 alive 探测"选项**

## 关键事实与决策点

### 已确认：fathom scan 输出 = 存活且有开放端口的主机
读 `~/DEV/fathom/src/main.rs` `write_results`（162-190 行）：scan 遍历 `results`（存活主机中 `ports::scan` 有开放端口的），**存活但指定端口全关的主机不输出任何行**。因此：
- **fathom scan 的 JSONL 不能单独做 alive 探测**（alive 无端口主机不可见）
- alive 判定必须**结合端口扫描**：fathom 输出中出现的主机 = alive（且开放了探测端口）
- anchorscan 当前外层 nmap -sn 的意义是"筛掉存活但目标端口全关的主机，避免逐个跑完整扫描"——fathom 替代后，这一层过滤**在 fathom 内部天然完成**（scan 只对 open 端口主机输出）

**因此 M4.4 的实质是：删除外层 nmap -sn 存活探测段（第 36-63 行），让 scanTargets 直接对 scope 全部地址调 scanTarget（每 target 一次 fathom scan，内部完成 alive+port+fingerprint）**。存活过滤由 fathom 内部承担，不再需要单独探测步骤。这与 M4.2 的单 IP 调用设计一致。

### 决策点（更新）

- **DiscoveryAuto**：删除外层 nmap -sn，直接对所有 target 调 `RunFathomScan`。fathom 内部 alive 过滤，无开放端口的主机自然无输出、无指纹——**语义等价于"存活且开放"**。行为差异：旧 nmap -sn 会显示"host alive but no open ports"（progress 日志），新行为是这些主机完全静默。**progress 语义可保留**（用 `no live hosts discovered` 类消息），由你决定如何呈现。
- **DiscoveryAssumeUp**：fathom 无 assume-up 选项。方案 A（`--no-icmp` TCP 回退探测）需评估——**fathom 的 `--no-icmp` 会降低探测强度**（80/443/445/22 四端口 TCP），对 assume-up 语义（信任全部存活）反而是倒退。**推荐**：assume-up 模式传 `--no-icmp` 以减少 ICMP 依赖？**或** assume-up 保持现有行为（scope.Addresses() 全部视为 target，直接 scanTarget——fathom 内部仍会 alive 过滤，但那是 fathom 的探测职责）。**由你权衡并在报告中说明**，倾向"assume-up 语义由 anchorscan 保持（全部地址进入 scanTarget），fathom 内部探测照常"——即 assume-up 只是跳过 anchorscan 侧预处理，不改变 fathom 调用参数。
- **IPv6**：fathom 仅 IPv4。IPv6 scope 保留现有 nmap `-sn` 路径（含 `RequiresNmapDiscovery` 检查保留给 IPv6）。IPv4 scope 完全走 fathom。
- **exclude**：scope.Addresses() 已应用 exclude。fathom 无 exclude 参数，用展开列表（注意命令行长度，大 CIDR 用 `-l` 文件或按 target 调用——M4.2 已是单 IP 调用，天然规避）。
- **RunFathomScan 单 IP 签名不动**：scanTargets 对每个 address 调 scanTarget → RunFathomScan，无新增 alive 函数（M4.4 不新增 RunFathomAliveScan）。

## Scope

### 要做

1. **scan_targets.go 存活探测段替换**（第 36-63 行）：
   - IPv4 + DiscoveryAuto：**删除 nmap -sn 段**，直接对所有 target 调 scanTarget（fathom 内部 alive 过滤）
   - IPv4 + DiscoveryAssumeUp：scope.Addresses() 全部视为 target 直接 scanTarget（fathom 内部探测照常；assume-up 只是跳过 anchorscan 侧预处理）
   - IPv6：**保留**现有 nmap `-sn` 路径（`RequiresNmapDiscovery` 检查保留给 IPv6）
   - progress 语义：保留"存活主机数/无存活主机"类消息（fathom 输出推导）
   - `aliveIPs` 语义：原用于记录"探测到的存活 IP"，fathom 模式下从 scan 结果推导（有指纹的主机）

2. **不需要新增 RunFathomAliveScan**——存活过滤由 fathom scan 内部完成（已确认：scan 只输出存活且有开放端口的主机）。

3. **DiscoveryMode 文档/语义更新**：
   - assume-up 的行为说明（fathom 模式下 anchorscan 侧不预处理）
   - CHANGELOG 补 M4.4 条目

4. **测试**：
   - scan_targets 集成测试：fake Runner 返回 fathom JSONL → 指纹/存活推导正确
   - assume-up 模式测试
   - IPv6 保留 nmap 路径测试
   - web-smoke fixture 更新（tool-fixture.sh 若需要加分支）

### 不要做
- 不删除 `internal/tools/nmap.go` 的 `CheckAlive`/`DiscoverAliveInScopeWithOutput`（单工具模式 tool_run.go:311 仍用 `CheckAlive`；IPv6 路径仍用）
- 不改 M4.2 已合并的 `RunFathomScan` 单 IP 签名
- 不做 git 操作
- 不引入新依赖

## 铁律

1. 零新依赖
2. **先确认 fathom scan 对"存活但无开放端口主机"的输出行为**（读源码或实测），再定 alive 判定方式——禁止猜测
3. IPv6 保留 nmap -sn（fathom 仅 IPv4）
4. 诚实报告：分列「实测」与「静态推断」；fathom binary 行为如无法本地实测，说明原因
5. 已知未跟踪文件（spikes/ 等）不得删除或修改
6. 完成后不得自行 commit
7. 报告文件：`docs/reports/fathom-m44-report.md`

## 验收

1. `go build ./...` 通过
2. `go test ./... -count=1` 全过
3. `make web-smoke` 通过
4. IPv4 + auto 模式：外层无 nmap -sn 调用（存活探测完全由 fathom 内部承担）
5. IPv4 + assume-up：scope.Addresses() 全部进入 scanTarget，anchorscan 侧不预处理
6. IPv6：保留 nmap -sn
7. 报告 `docs/reports/fathom-m44-report.md`（含 fathom alive 输出行为确认、assume-up 决策说明、改动文件清单、progress/aliveIPs 语义迁移说明）
