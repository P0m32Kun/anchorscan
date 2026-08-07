# Fathom 集成 M4.2 实施报告 — scan_target 前段切换为 fathom 唯一路径

> Spec：`docs/plans/fathom-integration/spec.md`（v2.0，设计反转：fathom 唯一路径，不保留 legacy 回退）
> 任务书：`docs/fathom-m42-brief.md` + `docs/fathom-m42-continuation.md`（续派）
> 范围：M4.2。scan_target 前段（rustscan 端口发现 + nmap -sV 指纹）→ 单次 `RunFathomScan`；达梦/TLS 衔接；preflight 必填校验。
> 不做：不删除 internal/tools 中 rustscan/nmap 函数与 config 字段（M4.3）；不做 git 操作；零新依赖。

## 一、结论

| 验收项 | 命令 | 结果 |
|---|---|---|
| 1. 全量构建 | `go build ./...` | ✅ PASS（exit 0） |
| 2. 三包单测 | `go test ./internal/app/ ./internal/tools/ ./internal/preflight/ -count=1` | ✅ 全 PASS（app 9.5s / tools 1.5s / preflight 0.3s） |
| 3. scan_target 集成：fathom JSONL → 指纹 → 后段 | `TestRunScanStoresFingerprintAndWritesJSONReport`、`TestRunScanRunsNSEAndNucleiForSSH/Redis`、`TestRunScanPassesExtraArgsToTools` 等 | ✅ PASS |
| 4. fathom 未配置 → preflight error | `TestRunBlocksMissingFathom`（精确断言 brief 消息文案） | ✅ PASS |
| 5. 达梦 fathom 检出跳过 nuclei dameng-identify | `TestRunScanRecordsDamengPanicAsCompletedWithErrors`（间接钉死）+ `TestScanTargetDamengNucleiGate`（nuclei 仍权威分支） | ✅ PASS |
| 6. TLS web 增强触发 | `TestRunScanTLSWebEnhancementTriggersHttpx` + `TestNeedsTLSWebEnhancement` | ✅ PASS |
| 7. 报告 | 本文档 | ✅ |

附加验证：`go test ./... -count=1` 全仓 17 个包全 PASS（含前两轮未触及、本轮适配的 `internal/web`、`cmd/anchorscan`）。

铁律遵守：零新依赖；fathom 唯一路径（scan_target 内无任何 rustscan/nmap-sV 回退分支）；未做任何 git 操作；`spikes/` 等未跟踪文件未触碰。

## 二、改动文件清单（15 个，工作树 vs HEAD 898ecb5）

| 文件 | 改动 |
|---|---|
| `internal/app/scan_target.go` | **核心**：前段 rustscan+nmap 替换为 `RunFathomScan`；fathom checks → DetectionCheck（engine=fathom）/ vulnerable finding 持久化；达梦 gate 增加 `fp.Normalized=="dameng"` 直通分支；httpx 触发条件加 `NeedsTLSWebEnhancement`；新增 `parseScanPorts`/`scanPortBounds`/`parseScanPort`（CSV+range 展开，对齐 internal/ports 语法）；移除 nmap heartbeat goroutine |
| `internal/app/scan_prepare.go` | top1000 预设展开为 CSV（fathom -p 只收显式列表/区间）；Fathom binary/timeout 经 `config.ToolPaths`/`ToolDurations` 类型别名直达 ScanOptions（M4.1 已备字段） |
| `internal/preflight/preflight.go` | `checkRequiredFathom`：路径为空 → **error**（brief 精确文案）；rustscan 从必填降为 optional；nmap 保持必填（NSE/alive-sweep 引擎） |
| `internal/report/model.go` | 注释更新（OpenPorts 来源 rustscan → fathom） |
| `internal/tools/fathom.go` | `FathomScanResult` 结构、`RunFathomScan`（argv：`scan --json <ip> -p <ports>`）、`fathomPortSpec`（range 折叠，规避 ARG_MAX）、checks→finding/DetectionCheck 映射、`TLSWebCandidatePorts`/`NeedsTLSWebEnhancement`（M4.1 预留，M4.2 接线） |
| `internal/app/scan_target_test.go` | fixture 全面 rustscan/nmap → fathomJSONL；新增 `TestRunScanTLSWebEnhancementTriggersHttpx` |
| `internal/app/scan_target_dameng_test.go` | 5 个达梦测试 fixture 换 fathomJSONL |
| `internal/app/scan_target_direct_test.go` | 直接驱动 scanTarget 的测试适配；NSE skip 计数 2→1（fathom 单次调用） |
| `internal/app/scan_target_rdpscan_test.go` | fixture 换 fathomJSONL |
| `internal/app/scan_targets_test.go` | RunScan 级测试：Rustscan→Fathom、alive-sweep 后并发、kill-after-cancel |
| `internal/app/scan_test.go` | fixture 换 fathomJSONL；新增 fathomJSONL 助手；`killedAfterCancelRunner` 适配（alive-sweep 先行、fathom 被 kill） |
| `internal/app/scan_prepare_test.go` | PrepareScan 层 fathom fixture + 未配置 fathom 用例 |
| `internal/preflight/preflight_test.go` | `TestRunBlocksMissingFathom`（精确消息断言）等 |
| `internal/web/scans_test.go` | web 层 preflight 渲染、top1000 展开测试适配 |
| `cmd/anchorscan/scan_command_test.go` | CLI 层 fixture 适配 |

## 三、前段替换前后对比

```
替换前（每 target）                          替换后（每 target）
rustscan 端口发现（-a --range -g）     →    一次 `fathom scan --json <ip> -p <ports>`
  → openPorts 列表                              → 解析 JSONL → Fingerprints/Findings/Checks
nmap -sV 指纹（-sV --version-intensity 7）     → fingerprint.Classify 归一化（fathom 无 CPE，
  → ServiceFingerprint[]                            降级已接受，spec 决策 3）
```

- 存活探测（`nmap -sn`）**不在** scan_target 内，属 RunScan 的 target 发现阶段，M4.2 范围未动（测试仍钉 `nmap -sn` 行为；fathom discover 段落地属 M4.3）。
- 后段（httpx / NSE / nuclei / dameng / rdpscan / 默认口令）循环体逐条保留，仅指纹来源与两条衔接分支变化。
- 无开放端口：fathom 输出为空 → 提前返回（同旧逻辑），httpx/NSE/nuclei 全跳过。
- scope 过滤位置保持：`filterScopeFingerprints` 在 fathom 输出后、后段循环前，逻辑不变。
- artifact：`fathom-<target>.jsonl`（原 `rustscan-<target>-ports.txt` + `nmap-service-<target>.xml`）。
- 移除 nmap heartbeat goroutine（其服务对象 nmap 前段已不存在；进度事件改为 fathom 起止两行）。

## 四、达梦 / TLS 衔接说明

**达梦（spec 决策 4）**：`fp.Normalized == "dameng"` 时直接 `damengMatched = true`，跳过 nuclei dameng-identify 往返，立即进入默认口令检查——fathom 协议握手识别权威性等同 nuclei dameng-detect。fathom 未检出 dameng（如 padl2sim/unknown）仍走 nuclei dameng-detect 路径（nuclei 仍是达梦协议权威之一）。测试覆盖：
- 直通分支（间接钉死）：`TestRunScanRecordsDamengPanicAsCompletedWithErrors` — fathom 报 dameng → 若错误地先跑 nuclei 且不匹配，dameng 检查会是 skipped/no_matching_rule 而非 failed/command_failed，测试即失败。
- nuclei 权威分支：`TestScanTargetDamengNucleiGate`（padl2sim + dameng-detect 模板命中 → checker 调用 2 次）；`TestRunScanTriggersDamengFinding`（nuclei 命中 → fp 升级 Service=dameng/Product=Dameng Database/Version=8.1.2.128 + high finding）。

**TLS web（spec 决策 2）**：fathom http 探针为明文 GET，无法完成 TLS 握手，TLS-only 端口报 service=unknown。`NeedsTLSWebEnhancement(normalized, port)`（unknown ∧ 443/8443/9443/4443/8843）为 true 时，即使 `fp.IsWeb=false` 也触发 httpx，并以合成的 `https://ip:port` URL 探测（TLS 候选端口表 `TLSWebCandidatePorts`）。httpx 成功 → `fp.IsWeb=true` + `fp.URL` 更新（同既有 httpx 逻辑）。测试：`TestRunScanTLSWebEnhancementTriggersHttpx`（unknown@8443 → httpx 收到 https URL → 指纹升级 web+URL）。

**fathom checks 持久化**：每个 check（vulnerable/safe/unknown）记一条 DetectionCheck（engine="fathom"，status="completed"，reasonCode=verdict，detail=proof）；仅 vulnerable 额外生成 high-severity Finding（`fathomCheckSeverity` 表，兜底 high）。

## 五、实测 vs 静态推断

| 项 | 类别 | 说明 |
|---|---|---|
| `go build ./...` + 三包测试 + 全仓测试 | 实测 | 本机真实执行，全部 PASS |
| scan_target JSONL→指纹→后段衔接 | 实测 | 单测驱动：fathomJSONL fixture → Classify → httpx/NSE/nuclei/dameng/rdpscan 各分支均有断言 |
| preflight fathom error 文案 | 实测 | `TestRunBlocksMissingFathom` 精确断言 brief 文案 |
| 达梦跳过 / TLS 增强触发 | 实测 | 上节所列测试 |
| top1000 展开行为 | 实测 | `TestScanCreateExpandsTop1000PresetForFathom`（web 层）+ PrepareScan 测试 |
| `fathomPortSpec` ARG_MAX 动机（全量 1-65535 平铺 CSV ≈382KiB > macOS ARG_MAX ≈256KiB） | 静态推断 | 基于字节数计算，未在真实 binary 上以 65535 端口实测 |
| 存活探测仍为 `nmap -sn` | 实测 | `TestRunScanUsesAliveSweepResultsAsTargets` 钉死；fathom discover 替换属 M4.3 |
| 真实 fathom binary 端到端（lab） | 未验证 | 本任务为单测验收；M4.1 已对真实 binary 做过 schema 实证（见 M4.1 报告），M4.2 未引入新 schema 字段，lab 复验非必需 |

## 六、遗留风险 / M4.3 清单

1. **rustscan 残留**：`internal/tools` 中 `DiscoverPortsWithOutput` 等函数、config `rustscan` 字段、`ToolExtraArgs.Rustscan` 均保留（本任务范围外）；preflight 已降为 optional 警告。M4.3 统一删除。
2. **存活探测未切 fathom**：RunScan 的 `nmap -sn` alive sweep 仍依赖 nmap；fathom discover 段落地后替换（spec 总目标「fathom 包揽存活探测」尚未完全达成）。
3. **CPE 缺失**：fathom 不产 CPE，ServiceFingerprint.CPE 恒空（spec 决策 3 接受的降级）；依赖 CPE 的规则匹配能力弱于 nmap 时代。
4. **fathom checks 覆盖有限**：首批 10 个 check-id（M4.1 表），服务门控外的服务无 checks 输出。
5. **parseScanPorts 为 internal/ports 语法的镜像实现**：两处语法（CSV/range）若未来分叉需同步；M4.3 可将 internal/ports 的展开器导出后复用。
6. **并发会话协调**：本任务执行期间检测到上一轮 pi 进程（PID 81427，9:17 启动）未被真正杀死，仍在同一工作树继续适配（其改动与本轮验证一致，最终 15 文件 diff 均经全仓测试确认）；后续编排应确认旧会话已终止，避免双写。

## 七、铁律遵守声明

- 零新依赖（diff 无 go.mod/go.sum 变更）。
- fathom 唯一路径：scan_target.go 无 rustscan/nmap-sV 分支；fathom 未配置 = preflight error = 不启动扫描。
- 未做任何 git 操作（commit/push/checkout/stash 均未执行）；`spikes/` 未跟踪目录未触碰。
- 报告分列「实测」与「静态推断」，lab 验证缺项已说明原因。
