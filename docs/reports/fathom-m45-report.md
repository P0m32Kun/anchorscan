# Fathom M4.5 报告 — 彻底删除 rustscan + IPv6 legacy 路径

> 日期：2026-08-07 ｜ 任务书：`docs/fathom-m45-brief.md` ｜ Spec：`docs/plans/fathom-integration/spec.md` v2.0
> 本报告未执行任何 git 提交/推送（含恢复 web-smoke 再生成的截图产物，仅 `git checkout --` 还原文件，未 commit）。

## 一、变更总览

**A. rustscan 完整删除**（config 字段 / 工具页 / doctor / preflight / CLI / profile / 测试 / 文档）
**B. nmap IPv6 `-sn` 存活探测删除**（IPv6 target 直接报错；nmap 本体保留为 NSE 引擎 + 单工具模式）

## 二、删除内容明细（实测）

### A. rustscan

| 位置 | 删除内容 |
|---|---|
| `internal/tools/rustscan.go` + `rustscan_test.go` | 整文件删除（`DiscoverPorts`/`DiscoverPortsWithOutput`/`extractPortMatches`，grep 确认无其他调用方；原 `fakeRunner` 测试桩迁移到新文件 `internal/tools/fake_test.go`，被 httpx/nse/rdpscan 测试共用） |
| `internal/app/tool_run.go` | `runRustscanTool`、`case "rustscan"` 分派、`hasExtraArgs`/`nativeToolBinary`/`toolRunContext` 中 rustscan 分支 |
| `internal/config/config.go` | `ToolArgs.Rustscan`、`ToolPaths.Rustscan`、`ToolTimeouts.Rustscan`、`ToolDurations.Rustscan`、`Durations()` 解析、`Normalized()` 列表 |
| `internal/config/init.go` | `defaultConfig()` 中 rustscan 默认 timeout 与 `detectToolPath("rustscan")` |
| `internal/config/profile.go` | `Overrides.RustscanArgs`、`ResolveScan` 覆盖、`ValidateScopeSafeToolArgs` 条目 + allowlist、`builtInProfiles` 三档 `rustscan_args` |
| `internal/doctor/doctor.go` | `rustscanCheck`（含调用） |
| `internal/preflight/preflight.go` | rustscan optional check |
| `internal/web/config.go` | form 字段读写、`toolDiagnostics` 工具名列表 |
| `internal/web/tools.go` + `tool_page.html` | `manualTools()` 的 rustscan 卡片、`isManualTool`、`applyToolExtraArgs`、端口解析条件、`ToolRunOptions.Tools` 字面量、`data-insert-ports` 按钮分支 |
| `internal/web/scans.go` + `ScanCreate.vue` | `scanForm.RustscanArgs`、form 读取、`Overrides.RustscanArgs` 接线、vue 表单字段/输入框 |
| `internal/web/templates/config.html` | rustscan 路径输入框、`timeout_rustscan` 档、相关帮助文本 |
| `cmd/anchorscan/*.go` | `--rustscan-args` flag、Overrides 接线、`tools check` 列表、help 文本、`logPreflight` timeouts 串、`applyToolExtraArgs`、`ToolRunOptions.Tools` 字面量 |
| `internal/app/provenance.go` | `toolVersions` map 的 rustscan 项 |
| `config/default.yaml.example` | 全部 rustscan 行（tools/timeouts/profiles 三处） |
| `config/default.yaml`（未跟踪本地配置） | 同上清理（验收 grep 覆盖 `config/` 目录） |
| `e2e/smoke_test.go` | `toolPaths.rustscan`、`resolveToolPaths`、`writeConfig` 内容、两个用例的 toolPaths 字面量 |
| `.github/workflows/lab.yml` | `cargo install rustscan`、evidence 的 `rustscan --version` |
| `scripts/web-smoke.mjs` | 配置替换列表、`timeout_rustscan` 断言（改用 `timeout_fathom`） |
| `scripts/package_clean.go` + 测试 | 禁运模式列表移除 rustscan、fixture 与断言 |
| 测试文件 | scan_prepare/manager/run_lease/scan_lifecycle/progress/scan_target/tool_run/provenance/config(4)/doctor/web(4)/cmd(4)/preflight 各 fixture/mock 清理 |
| 文档 | README（首段/依赖/端口表/单工具清单）、CONTEXT.md、project-status、deploy、testing-lab-checklist、CHANGELOG 新增 M4.5 Removed 条目 |

### B. IPv6 legacy

| 位置 | 变更 |
|---|---|
| `internal/app/scan_targets.go` | IPv6 分支删除：scope 含 IPv6 → 直接报错 `fathom does not support IPv6 targets`（不再要求 nmap、不再跑 `-sn` 扫）；auto 分支简化为 `scope.Addresses()`；`aliveHostsFromResults` 去掉 ipv6Alive 参数 |
| `internal/target/scope.go` | `RequiresNmapDiscovery` 删除（grep 确认无调用方）；`IsIPv6`/`NmapTargets`/`NmapExcludes`/`DiscoveryScopes` 保留（scan_targets 报错判定 + nmap 单工具 alive 模式仍用） |
| `internal/tools/nmap.go` | `DiscoverAliveInScopeWithOutput` 注释更新（仅服务单工具 `--mode alive`）；函数本身保留（`CheckAlive` → `DiscoverAlive` → `DiscoverAliveInScope` 链条仍有单工具调用方） |
| `internal/preflight/preflight.go` | nmap 必需性注释改为「NSE 引擎」，删除 IPv6 discovery 语义 |
| `cmd/anchorscan/scan_command.go` | `--discovery` 帮助文本、`--target`/`--exclude-targets` 帮助文本（标注 IPv6 unsupported） |
| 测试 | `scan_targets_test.go`：删 `TestRunScanAutoModeKeepsNmapSweepForIPv6`、`TestRunScanAutoModeMixedScopeSplitsDiscovery`、`TestRunScanBlocksIPv6WithoutNmap` 与 `ipv6AliveSweepXML`；新增 `TestRunScanRejectsIPv6Targets`（单地址/CIDR/混合 scope 三例） |

## 三、验收结果（实测）

| # | 验收项 | 结果 | 实测命令 |
|---|---|---|---|
| 1 | `go build ./...` | ✅ | 通过（exit 0） |
| 2 | `go test ./... -count=1` | ✅ | 全过（无 FAIL/panic；`internal/web` 原有一例 rerun 断言因快照字段删除已同步更新） |
| 3 | `make web-smoke` | ✅ | `Web browser smoke test passed.`（含 ticket-04 脚本，`&&` 串联退出 0） |
| 4 | `grep -rn "rustscan" internal/ cmd/ config/` | ✅ 零残留 | 输出为空（exit 1）；前端产物 `internal/web/static/dist/assets/main.js` 已重建，`grep -c` = 0 |
| 5 | IPv6 target 明确报错 | ✅ | `TestRunScanRejectsIPv6Targets` 覆盖三种输入，断言错误串 `fathom does not support IPv6 targets` |
| 6 | `-sn` 相关函数无调用方则删除 | ✅（保留有依据） | `CheckAlive`/`DiscoverAliveInScopeWithOutput` 仍有调用方——单工具 nmap `--mode alive`（`tool_run.go`）+ `nmap_alive_test.go`，按任务书「若 IPv6 删除后无调用方则删除」条件不满足，保留并更新注释；扫描流水线内已零引用 |
| 7 | 报告文件 | ✅ | 本文件 |

辅助验证（实测）：
- `go vet ./internal/... ./cmd/... ./scripts/...` ✅
- `go vet -tags e2e ./e2e/` ✅（e2e 编译通过）
- `node --test internal/web/static/*.test.mjs internal/web/frontend/*.test.mjs` ✅ 29/29
- `npm run build:web`（vue-tsc 类型检查 + vite 重建 main.js）✅

## 四、静态推断（未实测项）

1. **e2e 套件（`make e2e`）未实跑**：需真实 nmap/httpx/nuclei 与 Docker lab。静态检查确认 `e2e/smoke_test.go` 编译通过（`go vet -tags e2e`）。**前置缺口**：e2e 配置与 `lab.yml` 自 M4.2 起未含 fathom（fathom 为必配，preflight 会阻断扫描用例）——此为既有问题（本次变更前已存在），不在 M4.5 范围内；若编排方需要 e2e 可用，需另行补齐 fathom 安装/配置步骤。
2. **web-smoke 覆盖的配置页交互**：`timeout_fathom` 替代 `timeout_rustscan` 的断言已实跑通过（web-smoke 实测），不属推断。
3. **旧配置文件兼容**：仍含 `tools.rustscan`/`timeouts.rustscan`/`rustscan_args` 的用户旧配置会被 yaml 解析静默忽略（未知字段），不会报错；字段删除不影响既有配置加载（静态推断，基于 `yaml.v3` 行为）。
4. **`docs/` 下 rustscan 提及**：仅存在于历史记录（CHANGELOG 历史条目、fathom M4.1-M4.5 任务书/报告、archive 计划文档、spec.md）与本次 M4.5 的「移除说明」条目，均为文档提及，符合铁律 2；`docs/plans/`、`docs/reports/fathom-m41/42/43` 属历史归档未改写。

## 五、说明与残留风险

- **未跟踪文件**：`spikes/` 未动；`config/default.yaml`（本地生成配置）按验收 grep 要求清理了 rustscan 行；`internal/web/static/dist/assets/main.js`（gitignore）已重建为无 rustscan 版本。
- **`internal/tools/fake_test.go` 新增**：原 `fakeRunner` 定义在已删除的 `rustscan_test.go` 中，httpx/nse/rdpscan 测试仍依赖，迁移至独立测试辅助文件（行为逐字段还原：`output`/`err`/`args` 记录 binary+args）。
- **web-smoke 副产物**：运行 `make web-smoke` 会重写 `docs/reports/ticket-04-playwright/` 下的截图/log/trace（随机端口与渲染差异），已用 `git checkout --` 还原该目录，未提交任何内容。
- **残余**：`nmap_alive_test.go`（单工具 alive 模式测试）保留；`internal/target/targetset.go` 仍接受 IPv6 条目（TargetSet 为项目级摄入清单，扫描时由 scanTargets 统一报错）——如需 TargetSet 层也拒绝 IPv6，属后续决策。
- 无新依赖；未改动 `RunFathomScan`、scan_target.go 后段（httpx/NSE/nuclei/达梦）、nmap `-sV`/NSE、httpx/nuclei/rdpscan/dameng。
