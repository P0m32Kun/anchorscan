# Fathom 集成 M4.4 实施报告 — 存活探测切换（移除 nmap -sn）

> 实施日期：2026-08-07
> 任务书：`docs/fathom-m44-brief.md`；Spec：`docs/plans/fathom-integration/spec.md`（v2.0）
> 前置：M4.1-M4.3 已合并。fathom 已是 scan_target 内唯一端口/指纹引擎。

## 结论

M4.4 全部验收项通过：`go build ./...`、`go test ./... -count=1`、`make web-smoke` 全绿。IPv4 流水线外层 nmap `-sn` 已移除，存活探测完全由 fathom scan 内置承担（`alive::find` → ICMP Datagram/Raw 分级 + TCP 回退 80/443/445/22）；IPv6 保留 nmap `-sn`；`assume-up` 保持 anchorscan 侧不预处理语义。未做任何 git 提交（由编排方审查后统一处理）。

## 一、fathom alive 输出行为确认（铁律 2）

### 源码确认（`~/DEV/fathom`，静态推断）

- `src/main.rs` `scan()`（106-148 行）：`for host in alive::find(targets.iter(), use_icmp)` → `ports::scan` 只对 open 端口起 fingerprint 线程 → `results` 仅含**存活且至少一个探测端口开放**的主机。
- `write_results()`（162-190 行）：只遍历 `results` 输出，**存活但指定端口全关的主机不输出任何行**。
- `alive.rs` `probe()`（322 行）：ICMP Datagram → Raw 分级失败后 TCP 回退（`tcp_ping` 80/443/445/22 四端口并发，`ConnectionRefused` 也视为响应）；`find()`（333 行）64 并发 worker。
- `targets.rs`：`Targets::parse` 内部展开 CIDR（无 exclude 参数），IPv4 专用解析。

### 实测（本机真实 fathom binary `~/DEV/fathom/target/release/fathom`）

| 场景 | 命令 | 结果 |
|---|---|---|
| 存活 + 探测端口全关 | `fathom scan --json 127.0.0.1 -p 1` | **0 行输出**，exit 0 |
| 存活 + 无服务地址 | `fathom scan --json 127.0.0.2 -p 22`（TCP connect 超时） | **0 行输出**，exit 0 |
| 存活 + 开放端口 | `fathom scan --json 127.0.0.1 -p 22` | JSONL 指纹行（`{"host":"127.0.0.1","port":22,"service":"ssh",...}`） |
| 探测端口全开 | `fathom scan --json 192.0.2.1 -p 22`（本机 utun1 隧道可达） | JSONL 指纹行 |
| IPv6 单地址（额外） | `fathom scan --json ::1 -p 22` | JSONL 指纹行（TCP 回退路径可用） |

**结论（实测 + 源码双重确认）**：fathom scan 输出 = 存活且有开放端口的主机；alive 判定必须结合端口扫描。因此 M4.4 的设计成立——外层 nmap -sn 删除后，存活过滤在 fathom 内部天然完成，anchorscan 侧不再需要单独探测步骤。

**附注（诚实披露）**：本机 `utun1` VPN 隧道对任意目标地址的 22/80/443/445 均 TCP 响应（代理/NAT 行为），无法构造"ICMP/TCP 全无响应"的真 down 主机实测；该场景由 `alive::find` 代码路径保证（不通过探测 → 不进入端口扫描 → 无输出），列为静态推断。另实测发现 fathom 对 **IPv6 单地址**目标经 TCP 回退路径部分可用（ICMP 仅 IPv4、CIDR 解析仅 IPv4）——这与 M4.2/M4.3 现状一致（nmap -sn 确认的 IPv6 主机进 scanTarget 后调 fathom），M4.4 未改变该行为。

## 二、assume-up 决策说明（任务书决策点）

**选择：assume-up 保持 anchorscan 侧语义，不改变 fathom 调用参数。**

- 任务书方案 A（传 `--no-icmp`）被否决：`--no-icmp` 让 fathom 跳过 ICMP 只走 80/443/445/22 四端口 TCP 回退，**降低探测强度**；对 assume-up 语义（信任全部存活）是倒退——探测强度降低反而增加漏报。
- 实施方案：assume-up 分支不变（`scope.Addresses()` 全部视为 target 直接 scanTarget），fathom 调用参数与 auto 模式完全相同（`fathom scan --json <ip> -p <ports>`）。assume-up 只是跳过 anchorscan 侧预处理；fathom 内部探测照常（ICMP + TCP 回退）。
- 行为边界：assume-up + IPv6 同样全部进 scanTarget（fathom TCP 回退），不额外处理。
- 已同步：`docs/deploy.md` 运行限制说明、CHANGELOG。

## 三、progress / aliveIPs 语义迁移说明

| 项 | M4.4 前 | M4.4 后 |
|---|---|---|
| auto + IPv4 外层探测 | `nmap alive sweep targets=%v` → `nmap alive hosts=%v` → `no live hosts discovered; skip port scan` | 无 nmap 消息；`scan targets=%d (fathom alive probing is internal; nmap -sn kept for IPv6)` → fan-out 后 `alive hosts=%d` 或 `no live hosts discovered`（由 scan 结果推导） |
| auto + IPv6 | nmap -sn（ipv4/ipv6 双 artifact 名） | nmap -sn 仅 IPv6 scope 部分，artifact 恒为 `nmap-alive-ipv6.xml` |
| assume-up | `assume-up: skip alive discovery, treat N host(s) as up` | 不变 |
| aliveIPs（报告 `alive_ips`） | = nmap -sn 确认的存活主机（进入端口扫描的 target） | IPv4 = **有 fathom 指纹的主机**（fathom 只输出存活且有开放端口的主机，故 alive 判定与端口扫描绑定）；IPv6 = nmap -sn 确认的主机（即使无指纹也计入）。assume-up = 全部 scope 地址（信任声明，不变） |
| progress 分母 | totalTargets = nmap 存活数 | totalTargets = scope 全部 IPv4 地址 + nmap 确认的 IPv6 主机（fathom 对 down/全关主机静默，进度仍逐 target 推进） |
| 审计 artifact | `nmap-alive-ipv4.xml` + `nmap-alive-ipv6.xml` | 仅 `nmap-alive-ipv6.xml`（IPv4 不再产生；fathom JSONL 已落 `fathom-<ip>.jsonl`） |

## 四、改动文件清单

### 代码

| 文件 | 变更 |
|---|---|
| `internal/app/scan_targets.go` | **核心**：删除外层 nmap -sn 存活探测段。新逻辑：nmap 必需性检查改为"scope 含 IPv6"（错误文案 `nmap is required for IPv6 scan targets`）；assume-up 分支不变；auto 分支按 `DiscoveryScopes()` 拆分——IPv4 scope 部分 `Addresses()` 全部进 scanTarget（fathom 内部 alive 过滤），IPv6 scope 部分保留 nmap -sn（artifact `nmap-alive-ipv6.xml`）；fan-out 后新增 `aliveHostsFromResults(scans, ipv6Alive)` 从 scan 结果推导 aliveIPs（有指纹的主机 + nmap 确认的 IPv6 主机），并 emit `alive hosts=N` / `no live hosts discovered` |
| `internal/config/config.go` | `Fathom` 字段注释更新（M4.4 起拥有 IPv4 存活探测） |
| `internal/preflight/preflight.go` | nmap 必填注释更新（NSE 引擎 + IPv6 存活扫描） |
| `internal/target/scope.go` | `RequiresNmapDiscovery` 注释更新（M4.4 后不再门控 scan_targets；IPv4 CIDR 由 fathom 覆盖，nmap -sn 仅 IPv6 需要） |
| `internal/tools/nmap.go` | `DiscoverAliveInScopeWithOutput` 注释更新（IPv6 scope 部分 + 单工具模式专用） |
| `cmd/anchorscan/scan_command.go` | `--discovery` flag 帮助文本更新（auto 语义：fathom 内置探测，nmap -sn 仅 IPv6） |

### 测试

| 文件 | 变更 |
|---|---|
| `internal/app/scan_targets_test.go` | **重写 + 新增**：`TestRunScanAutoModeScansAllScopeAddressesWithFathom`（/30 CIDR → 4 次 fathom 调用、无 nmap）；`TestRunScanAutoModeKeepsNmapSweepForIPv6`（IPv6-only → nmap -sn -6 + nmap-alive-ipv6.xml artifact + alive_ips）；`TestRunScanAutoModeMixedScopeSplitsDiscovery`（IPv4+IPv6 混合拆分）；`TestRunScanAutoModeDerivesAliveIPsFromFathomOutput`（alive_ips 从有指纹主机推导）；`TestRunScanBlocksIPv6WithoutNmap` / `TestRunScanAllowsIPv4CIDRWithoutNmap`（nmap 必需性收窄）；`TestRunScanSkipsPortScanWhenHostHasNoFathomOutput`（down 主机语义迁移）；`TestRunScanFastProfileDoesNotReduceFathomTargets`；`TestRunScanRespectsProfileHostWorkersAfterTargetExpansion`；`TestRunScanClampsHostWorkers` 用例名更新 |
| `internal/app/scan_test.go` | 21 处 sequence fixture 删除 `aliveNmapXML` 首项；`aliveSweepRunner`/`downHostRunner`/`killedAfterCancelRunner`/`postAliveConcurrencyRunner`/`profileSensitiveAliveRunner` 语义迁移（nmap -sn 分支移除或改为报错）；`TestRunScanWritesAuditArtifacts` 断言改为"无 nmap-alive artifact" |
| `internal/app/scan_target_test.go` | sequence fixture 删除 aliveNmapXML 首项（14 处）；`TestRunScanContinuesAfterNSEFailure` errors 数组错位修复 |
| `internal/app/scan_target_rdpscan_test.go` | sequence fixture 删除 aliveNmapXML 首项（5 处） |
| `internal/app/scan_prepare_test.go` | sequence fixture 删除 aliveNmapXML 首项（2 处）+ gofmt |
| `cmd/anchorscan/scan_command_test.go` | sequence fixture 删除 aliveNmapXML 首项（4 处） |

### 文档

| 文件 | 变更 |
|---|---|
| `CHANGELOG.md` | [Unreleased] Added 加 M4.4 条目；M4.1+M4.2 条目流水线描述同步（移除 nmap -sn 存活扫描）；Changed 的 nmap 角色描述更新 |
| `README.md` | 核心思路 + 前置依赖：nmap 角色 = NSE 引擎 + IPv6 存活扫描；fathom 承担存活探测 |
| `CONTEXT.md` | Fathom 词条：存活探测由 fathom scan 内置（含 alive.rs 探测细节），IPv4 流水线无外层 nmap -sn，IPv6 保留 |
| `docs/project-status.md` | 流水线描述 + 事件日志描述更新 |
| `docs/deploy.md` | auto / assume-up 行为说明重写 |
| `docs/testing-lab-checklist.md` | Scope 验证路径 + Expected Log Markers 更新（nmap heartbeat 残留清除） |

### 未改动（按任务书"不要做"）

- `internal/tools/nmap.go` 的 `CheckAlive`/`DiscoverAliveInScopeWithOutput` 保留（单工具模式 tool_run.go 仍用；IPv6 路径仍用）。
- `internal/tools/fathom.go` 的 `RunFathomScan` 单 IP 签名不动；**未新增 RunFathomAliveScan**（存活过滤由 fathom scan 内部完成）。
- `spikes/` 等未跟踪文件未触碰；未做任何 git 操作；零新依赖（无 go.mod/go.sum 变更）。

## 五、验收对照

| # | 验收项 | 结果 | 证据 |
|---|---|---|---|
| 1 | `go build ./...` | ✅ | 通过（多轮，含最终状态） |
| 2 | `go test ./... -count=1` | ✅ | 全仓 17 包 ok，无失败输出 |
| 3 | `make web-smoke` | ✅ | "Web browser smoke test passed."（含 ticket-04 复跑） |
| 4 | IPv4 + auto：外层无 nmap -sn | ✅ | `TestRunScanAutoModeScansAllScopeAddressesWithFathom`（nmap 调用直接报错）+ `TestRunScanAllowsIPv4CIDRWithoutNmap`（无 nmap 也能扫）+ 源码：auto 分支仅对 IPv6 discoveryScope 调 nmap |
| 5 | IPv4 + assume-up：全部地址进 scanTarget，anchorscan 侧不预处理 | ✅ | `TestScanAssumeUpExpandsScopeWithoutExcludedHosts`（.0/.1/.3 全部进 fathom）+ `TestRunScanAssumeUpSkipsAliveSweep`（无 -sn）+ 决策说明见第二节 |
| 6 | IPv6：保留 nmap -sn | ✅ | `TestRunScanAutoModeKeepsNmapSweepForIPv6`（-6 + nmap-alive-ipv6.xml）+ `TestRunScanBlocksIPv6WithoutNmap` |
| 7 | 报告 | ✅ | 本文件 |

## 六、实测 vs 静态推断

| 项 | 类别 | 说明 |
|---|---|---|
| fathom scan 对"存活但无开放端口"输出 0 行 | **实测 + 源码** | 127.0.0.1:1、127.0.0.2:22 实测 0 行；`write_results` 只遍历 open 端口结果 |
| fathom scan 对"存活且有开放端口"输出 JSONL | **实测** | 127.0.0.1:22、192.0.2.1:22（utun1 可达）、203.0.113.9（隧道代理响应） |
| down 主机（ICMP/TCP 全无响应）→ 无输出 | 静态推断 | 本机 utun1 隧道对任意地址 TCP 响应，无法构造真 down 主机实测；`alive::find` 代码路径保证 |
| fathom 对 IPv6 单地址部分可用（TCP 回退） | **实测** | `fathom scan --json ::1 -p 22` 输出指纹；ICMP/CIDR 仍仅 IPv4（spec 决策 5 不变） |
| aliveIPs 推导 / progress 迁移 | 实测 | 新增 5 个集成测试钉死（alive_ips 断言、命令序列断言） |
| `fathomPortSpec` ARG_MAX 动机（M4.1 遗留） | 静态推断 | 未在本任务复验（M4.2 已记录） |

## 七、残余风险与未验证项

1. **网络/广播地址进入探测集**：auto 模式 IPv4 用 `scope.Addresses()` 单 IP 展开（含网络/广播地址，如 /30 的 .0/.3），fathom 对其 ICMP 无响应即静默（~700ms 超时，64 并发下开销可忽略）；fathom 自身 CIDR 展开会跳过网络/广播地址，但 M4.2 单 IP 调用签名不变，故维持现状。与 assume-up 既有行为一致（`TestScanAssumeUpExpandsScopeWithoutExcludedHosts` 已钉 .0 进 fathom）。
2. **progress 分母变化**：auto 模式 totalTargets 从"nmap 存活数"变为"scope 全部地址数"，down 主机也计进度（任务书接受的语义差异）。
3. **web-smoke 运行刷新了已跟踪的 `docs/reports/ticket-04-playwright/` 截图与 `server.log`/`trace.zip`**（smoke 测试产物，每次运行覆盖）——非本次有意修改。
4. **真实 lab 端到端**：无 lab 环境，未在真实网络跑完整扫描；fathom binary 行为已本机实测（第二节表格），anchorscan 侧由集成测试覆盖。

## 交付物

- 本报告：`docs/reports/fathom-m44-report.md`
- 未执行任何 git 提交/推送（编排方审查后统一处理）。
