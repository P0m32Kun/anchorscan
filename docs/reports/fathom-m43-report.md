# Fathom 集成 M4.3 — 收尾清理实施报告

> 实施日期：2026-08-07
> 任务书：`docs/fathom-m43-brief.md`；Spec：`docs/plans/fathom-integration/spec.md`（v2.0）
> 前置：M4.1（PR #42）+ M4.2（PR #43）已合并

## 结论

M4.3 全部验收项通过：`go build ./...`、`go test ./... -count=1`、`make web-smoke` 全绿；doctor 输出含 fathom（缺失标红，rustscan 标注"not used in scan pipeline"）；config 页含 fathom 输入框；CHANGELOG / project-status / README / CONTEXT 已反映 fathom。未做任何 git 提交（由编排方审查后统一处理）。

## 关键事实核实（影响所有表述的准确性）

- **nmap `-sn` alive sweep 仍在流水线外层**：`internal/app/scan_targets.go` 在 fan-out 阶段对非 assume-up 模式执行 nmap 存活扫描；fathom 的 discover 段（`--discover`）未启用（`internal/tools/fathom.go` 的 `RunFathomScan` 不传该参数）。preflight 注释亦确认 "nmap is the alive-sweep engine until fathom's discover stage lands"。
- 因此所有文档/界面表述按事实校准为：**fathom 是 scan_target 内唯一端口/指纹引擎**（替代 rustscan 端口发现 + nmap -sV 指纹），而非"唯一存活引擎"；完整流水线为 nmap -sn 存活扫描 → fathom → httpx → NSE/nuclei。
- `config/default.yaml.example` 中 fathom 注释（M4.2 产物）写"一次调用完成存活/端口/指纹/高危检测"——描述的是 fathom 工具五段能力，与集成现状（discover 段关闭）不完全一致。该文件不在本次任务书修改清单内，未改动，仅在此如实说明。

## 变更清单

### 代码

| 文件 | 变更 |
|---|---|
| `internal/doctor/doctor.go` | 新增 `toolCheck("fathom", cfg.Tools.Fathom, false)`（缺失 → fail，标红；实际阻断在 preflight）；新增 `rustscanCheck`：rustscan 降为可选（缺失 → warning），并附说明 "not used in scan pipeline (single-tool mode only)"。**选择了任务书方案 B**：fathom 必需、rustscan 可选，最符合设计反转后的事实（doctor 是部署诊断，fathom 缺失即部署缺陷）。 |
| `internal/doctor/doctor_test.go` | 新测试 `TestFathomMissingFails`（fathom 缺失 → fail）、`TestRustscanMissingIsOptionalWithPipelineNote`（rustscan 缺失 → warning + 流水线说明且整体不 fail）；`TestRdpscanMissingReportsOptionalHint` fixture 补 fathom 字段。 |
| `internal/web/config.go` | 表单保存补 `cfg.Tools.Fathom` / `cfg.Timeouts.Fathom`（此前模板漏了输入框、handler 也漏了字段，用户无法在配置页修改 fathom 路径）；`toolDiagnostics` 工具名集合加 "fathom"，配置页工具可用性诊断现含 fathom。 |
| `internal/web/templates/config.html` | fathom 路径输入框（必填，置于表单最前，fathom 为主引擎）；rustscan label 改为"仅单工具执行模式"并加说明"扫描流水线不再调用"；超时表单加 Fathom 档；端口格式帮助文本改为"沿用 rustscan 写法习惯、由 fathom 直接消费"。 |
| `cmd/anchorscan/admin_command.go` | `tools check` 工具清单加 fathom（此前用户可见的工具检查命令缺 fathom）。 |
| `cmd/anchorscan/admin_command_test.go` | 三个测试 fixture 补 fathom 字段；`TestExecuteDoctorPrintsOptionalWarningsAndVersions` 断言 fathom ok 输出与 rustscan 流水线说明。 |
| `cmd/anchorscan/scan_command.go` | `logPreflight` 的 timeout 行补 `fathom=%s`（CLI 扫描 preflight 日志此前缺 fathom）。 |
| `internal/app/tool_run.go` | `RunTool` 顶部注释：明确单工具模式独立于扫描流水线（fathom 专用扫描路径）；`runRustscanTool`/`runNmapTool` 保留未动。 |

### 文档

| 文件 | 变更 |
|---|---|
| `CHANGELOG.md` | [Unreleased] Added 加 fathom 集成条目（M4.1+M4.2，含 dameng 衔接、归一化别名表）；Changed 加 fathom 必配/rustscan 标注变更。 |
| `docs/project-status.md` | Last reviewed 更新为 2026-08-07；Implemented capabilities 流水线描述改为 nmap -sn → fathom → httpx → NSE/nuclei；删除已不存在的 "-sV heartbeat" 描述；端口选择/单工具/工具打包清单段落同步。 |
| `CONTEXT.md` | 新增 `Fathom（侦察引擎）` 词条（含 discover 段未启用的准确说明）；TargetScan/Fingerprint/Finding/Artifact/Scope/项目一句话中的 rustscan/nmap 引用更新。 |
| `README.md` | 核心思路段、前置依赖（fathom 必配）、端口格式表、单工具调用说明更新。 |
| `docs/deploy.md` | Release 归档工具清单补 fathom（必配）。（任务书范围外的最小延伸：用户可见的工具依赖说明。） |
| `docs/troubleshooting-lab.md` | 排查流程第 1/2 节与 minimal triage 的 rustscan/-sV 引用改为 fathom（第 2 节原引用已删除的 `-sV` heartbeat 日志）。（任务书范围外延伸，理由同上。） |
| `docs/testing-lab-checklist.md`、`docs/testing-strategy.md` | lab 验收清单/测试分层表中流水线断言（`[scan] rustscan` → `[scan] fathom`、`-sV` 心跳断言删除）与 fathom 协作表述同步。（范围外延伸，理由同上。） |

### 未改动（按任务书"不要做"）

- `internal/tools/rustscan.go` / `internal/tools/nmap.go` 函数定义、config 字段、`tool_run.go` 的 `runRustscanTool`/`runNmapTool`：全部保留。
- `scan_target.go` 逻辑未动（M4.2 已完成）；其顶部注释与 progress 消息已是 fathom。
- 未引入新依赖；未触碰 `spikes/` 等未跟踪文件。

## 验收对照

| # | 验收项 | 结果 | 证据 |
|---|---|---|---|
| 1 | `go build ./...` | ✅ | 通过（多轮，含最终状态） |
| 2 | `go test ./... -count=1` | ✅ | 全部包 ok，无失败输出 |
| 3 | `make web-smoke` | ✅ | "Web browser smoke test passed." + `ticket-04-web-smoke.mjs` 单独复跑通过（TICKET-04 PASSED） |
| 4 | doctor 输出含 fathom | ✅ | 实测 `anchorscan doctor`：`fathom: fail path is empty`（未配置）/ `fathom: fail stat ...`（坏路径）；测试 `TestFathomMissingFails`、cmd 层断言 `fathom: ok` |
| 5 | config.html 有 fathom 输入框 | ✅ | `name="fathom"`、`name="timeout_fathom"` 均在模板中；渲染由 `internal/web` 测试覆盖 |
| 6 | CHANGELOG / project-status / README 反映 fathom | ✅ | 见变更清单 |
| 7 | 报告文件 | ✅ | 本文件 |

## 残余风险与未验证项

- 未在真实 lab 环境运行 fathom 扫描（无 lab 环境）；`docs/troubleshooting-lab.md` 中 fathom 排查命令 `fathom scan --json <IP> -p <PORT>` 依据 `RunFathomScan` 的 args 构造（`internal/tools/fathom.go`），可靠但未在真实二进制上复验。
- `make web-smoke` 运行会刷新已跟踪的 `docs/reports/ticket-04-playwright/` 截图与 `server.log`/`trace.zip`（smoke 测试产物，每次运行覆盖）——非本次有意修改，已随工作区呈现给审查方。
- `config/default.yaml.example` fathom 注释与集成现状的细微出入（见"关键事实核实"），未改动，留待后续（fathom discover 段落地时自然解决）。
- 单工具执行模式（tool 页/`anchorscan tool`）仍支持 rustscan/nmap——按任务书为有意保留。

## 交付物

- 本报告：`docs/reports/fathom-m43-report.md`
- 未执行任何 git 提交/推送（编排方审查后统一处理）。
