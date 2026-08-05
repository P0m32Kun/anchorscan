# Ticket 04 验收报告：在所有命令出口强制 safety 与 review 门禁

- 日期：2026-08-05
- 范围：`docs/ticket-04-brief.md`、`docs/ticket-04-approval.md`、`docs/ticket-04-continuation.md`、catalog v2 spec §5-6 与 ticket 04
- 风险等级：高（服务端统一授权 gate + 风险确认 UI，覆盖所有命令出口，禁止绕过路径）
- 执行说明：本票由上一轮 pi 中断后续跑完成；工作区既有改动全部保留，未推倒重写。本轮补缺包括：旧 `web-smoke.mjs` 两处断言适配 legacy 门禁、全量验收复跑、Playwright 产物重新生成、本报告。

## 改动摘要

- 新增 `internal/web/safety_gate.go`：统一门禁核心。
  - `commandGateViewForEntry`：根据**服务器 catalog 条目**重新计算五档门禁（safe / needs-review / optional / manual-gated / legacy-unknown），不接受客户端传入的 mode/effects/cleanup 作为事实；safety/命令缺失或非法时拒绝并返回 422。
  - `commandGateStore`：内存一次性 challenge token（消费即失效、5 分钟 TTL、重启失效、绑定请求指纹），以及一次性 tool-prefill token。确认字段由服务器生成并校验，不可复用为长期授权。
  - `enforceCommandGate`：未确认 → 428 + gate 视图（含 challenge）；确认（`acknowledge=1` + 有效 token）→ 放行；伪造/重放 token → 403。
  - `toolLink`/`toolPrefill`：`/tools/{tool}` 预填只经一次性 token 链路，`raw_args` query 参数不再生效。
- 接入点（全部复用同一 gate，无新 handler 绕过）：
  - `report_handler.go`：`reportCommand`（单条）、`reportBatchCommand`（批量 nuclei/nmap/msf，含 msf/nmap 分支）在构造命令前调用 `enforceCommandGate`。
  - `workbench.go`：`projectCandidateCommand`（工作台候选命令）接入同一 gate；tool link 改为 token 预填并携带 project/zone/verification/return 上下文。
  - `tools.go`：`toolPage` 仅消费一次性 prefill token；无 token 的手工工具页保留用户自行填写参数的能力。
  - `knowledgebase.go` + `knowledgebase_detail.html`：KB 详情仅在 `stable + safe` 时展示原始命令（`ShowCommands`），其余档位隐藏并显示门禁说明；复核状态与 safety 常驻可见。
- 前端 `ReportInteractions.vue` / `Workbench.vue`：命令对话框内嵌 gate 面板（effects 列表、cleanup、acknowledge 按钮），确认后命令才渲染；needs-review 的复核状态在确认后仍显示（不清除）。
- 测试：新增 `safety_gate_test.go`；既有 `report_handler_test.go` / `workbench_test.go` 改为经 gate 流程断言（未确认拒绝 → 确认成功 → 重放拒绝）；`scripts/web-smoke.mjs` 两处命令对话框断言适配 legacy fail-closed（先出现门禁面板并点击确认）。

## 命令出口清单（引用搜索证据）

`Entry.Commands` 全部消费点（`rg "Commands\.(Nuclei|NmapNSE|Metasploit)" internal/`，排除 loader 写入与测试）：

| # | 出口（HTTP 端点） | 代码位置 | Gate 接入 |
|---|---|---|---|
| 1 | `POST /reports/{runID}/commands`（单条 nuclei/nmap/msf） | `internal/web/report_handler.go:215`（命令检查）、`reportCommand` 内 `enforceCommandGate`（`report_handler.go:271` 附近） | `enforceCommandGate`（Action=report-single，key 绑定 runID+finding key，fingerprint 绑定 URL query） |
| 2 | `POST /reports/{runID}/commands/batch`（批量 nuclei/nmap/msf） | `internal/web/report_handler.go:387,397`（MSF/Nmap 分支），gate 在 `reportBatchCommand` 内（`report_handler.go:326` 附近），`batchCommandEntry` 按 group_key 反查 catalog 条目 | `enforceCommandGate`（Action=report-batch） |
| 3 | `POST /projects/{pid}/candidates/{key}/commands`（工作台候选命令） | `internal/web/workbench.go:478`（`BuildCandidateCommands`），gate 在 `projectCandidateCommand` 内（`workbench.go:472` 附近），`s.catalog.Entry(cand.GroupKey)` 取条目 | `enforceCommandGate`（Action=project-candidate，fingerprint 绑定 zone/asset/verification/return） |
| 4 | `GET /tools/{tool}`（工具页预填） | `internal/web/tools.go:82-94`（`toolPrefill` 消费一次性 token；`raw_args` query 直接丢弃） | 仅经 `toolLink` 签发的一次性 prefill token，tool 名绑定 |
| 5 | `GET /kb/{id}`（KB 详情命令展示） | `internal/web/knowledgebase.go:44`（`ShowCommands` 仅 stable+safe） | 服务端渲染条件，其余档位隐藏原始命令 |

非命令文本出口（只影响按钮可见性，不输出命令；点击后仍走 gate）：
- `internal/web/report_handler.go:191` `commandTools` → report.html 单条命令按钮。
- `internal/report/vulnerability_delivery.go:120-122` `CanBatch*` → report.html 批量命令按钮。
- 命令构造层 `internal/report/vulnerability_command.go:163,209,260` 与 `project_command.go` 仅被出口 1/2/3 消费（`grep -rln "BuildCandidateCommands\|vulnerability_command" cmd/` 无结果，CLI 无命令出口）。

## 机制要点

- **服务端重算**：`commandGateViewForEntry(entry, tool, diagnostics)` 只读当前 catalog 条目（ReviewStatus/Safety/Commands）与 loader diagnostics；`validCommandGateSafety` 校验 safety 形状（safe 无 effects；optional 恰好 `authentication-attempt`；manual-gated 至少一个合法 effect 且有 cleanup 条件；legacy-unknown 无 effects/cleanup）。客户端提交的 `mode`/`effects`/`cleanup`/`confirmed`/`safety_mode`/`review_status` 参数一律不参与判定。
- **一次性 challenge**：428 响应携带 `gate.challenge`；确认请求必须带 `acknowledge=1` 且 challenge 与 `commandGateKey`（action+tool+key+fingerprint+entry ID）匹配；消费后删除，重放返回 403；内存存储，进程重启即失效。
- **legacy-unknown**：确认消息明确"旧 Markdown 未声明 safety；此命令按至少 manual-gated 强度处理"，且 KB 详情显示同样说明。
- **needs-review**：gate 面板显示 `review=needs-review` 与"待复核"说明；确认后前端保留 gate 面板（challenge 清空）使复核状态不消失；KB 详情常驻"复核状态：needs-review"徽章与 acknowledgement 提示。
- **不泄漏**：428/422 响应体不含 full_command、tool_args、tool_link、raw_args 或命令片段（测试逐项断言）。

## 实测：必需验收

```bash
go test ./internal/...
```

结果：全部 `ok`（14 个包；`internal/web` 3.189s 含 gate 测试）。

```bash
go build ./...
```

结果：`EXIT=0`（无输出）。

```bash
go vet ./...
```

结果：`EXIT=0`（无输出）。

```bash
npm run typecheck:web
```

结果：`vue-tsc --noEmit` 通过。

```bash
npm run test:web   # = node scripts/web-smoke.mjs && node scripts/ticket-04-web-smoke.mjs
```

结果：`Web browser smoke test passed.`（两个脚本均通过）。

```bash
make test && make doc-check && make docx-test && make build && make package-test && make web-smoke
```

结果：全部通过（node --test 19/19；markdown 链接检查 OK；docx 5/5；前端构建 OK；package smoke `ok .../scripts 9.072s`；web-smoke 通过）。

### HTTP 直接调用证明（httptest，`internal/web/safety_gate_test.go`）

- `TestCommandGateEnforcesCatalogSafetyAndOneTimeAcknowledgement`：对 needs-review / optional / manual-gated 条目，携带伪造 `mode=safe&effects=&cleanup=attacker-controlled&confirmed=1&?safety_mode=safe&review_status=stable` 的**未确认**请求返回 428，且响应体不含 full_command/tool_args/tool_link/raw_args/命令片段；确认（challenge+acknowledge=1）后返回 200 含 full_command 与 tool_link；**重放同一 challenge 返回 403**。
- `TestCommandGateRejectsMissingOrInvalidCatalogDataWithoutLeakingCommand`：命令与 verify 不一致（loader degraded 清命令）与 safety 非法（safe 却带 effects）条目，带确认参数仍返回 422/400，且不泄漏任何命令文本。
- `TestToolPrefillGrantIsOneTimeAndIgnoresForgedToken`：safe 条目确认后的 `tool_link` 只含 `gate_token=`（不含 `raw_args=`）；首次 GET 预填成功，**重放同一链接不再预填**；手工构造 `/tools/nuclei?gate_token=forged&raw_args=<真实参数>` 不预填。
- `TestLegacyCommandAndKnowledgeBaseDetailFailClosed`：legacy Markdown 条目未确认 → 428 且 gate 显示 legacy-unknown + "旧 Markdown 未声明 safety"；确认后 200 得命令；KB 详情页不泄漏 `nuclei -t redis-default-logins` 等命令，显示 legacy 说明。
- `TestKnowledgeBaseDetailKeepsNeedsReviewVisibleAndHidesGatedCommands`：needs-review 详情页显示"待复核/acknowledgement"且不显示命令；optional/manual-gated 详情页显示 effects/cleanup 且不显示命令；stable+safe 详情页正常显示命令。
- `TestCommandGateTokensAreRequestBoundAndRestartLocal`：challenge 绑定请求 key（换 key 消费失败）；token 在新 store（模拟重启）中失效。
- 既有报告/工作台测试（`report_handler_test.go`、`workbench_test.go`）经 `postCommandWithGate` 全量走 gate：safe 直通、确认后成功、工具页预填经 token、批量命令（nuclei/nmap/msf）未确认拒绝。

### Playwright smoke（`scripts/ticket-04-web-smoke.mjs`，环境：Playwright 1.61.1 + Chromium headless）

产物（本次会话重新生成，`docs/reports/ticket-04-playwright/`，均为真实运行截图/诊断）：

- `safe-command.png`：safe 条目点击后命令立即可见（无 gate）。
- `optional-before-confirm.png` / `optional-after-confirm.png`：确认前显示 authentication-attempt effects 与 cleanup、无命令；点击"我已确认授权范围，生成命令"后命令出现。
- `manual-before-confirm.png` / `manual-after-confirm.png`：确认前显示 file-read / test-file-create 与 cleanup、无命令；点击"我已查看 effects 与 cleanup，确认生成命令"后命令出现。
- `needs-review-before-acknowledgement.png` / `needs-review-after-acknowledgement.png`：确认前无命令；acknowledgement 后命令出现且 needs-review 状态仍在面板中。
- `trace.zip`：含截图/snapshot/source 的完整 trace。
- `console.log`：仅 3 条预期的 428 gate 响应（optional/manual/needs-review 各一次），无意外 console error / pageerror。
- `server.log`：服务器诊断（启动无错误）。
- `result.txt`：`PASS: safe, optional, manual-gated, and needs-review command flows`

smoke 断言要点：确认前命令文本 count=0（未泄漏）、确认后命令可见、`pageerror`/意外 console.error 为零。

## 模拟 / 未实测分列

**已实测（自动断言驱动）：** 上表全部 Go/前端/Playwright 验收项；`make pr-check` 全链路（test、doc-check、docx-test、build、package-test、web-smoke）。

**未实测（需编排方终验）：**
- 纯视觉观感（配色、间距、门禁面板与现有页面风格的协调性）——截图已存，视觉终验归编排方，此处列"未实测"。
- 截图/trace 的人工目视复核（本环境模型不支持读图；截图内容由 smoke 的 DOM 断言间接保证）。
- 真实扫描器执行（本票非目标；所有命令仅生成不运行）。

**说明与降级：** 无。Playwright 本地环境可用，全部代表流实跑成功，无需降级。

## 审查与剩余项

- 门禁置于 HTTP 暴露层，`internal/knowledgebase` loader 与 `internal/report` 命令构造层零改动（`git diff` 不含这两个包），ticket 02/03 行为未变。
- 无账号、审批流、长期授权或自动扫描；challenge/prefill token 均为一次性、内存态、短 TTL。
- 旧 `web-smoke.mjs` 中两处"点击命令按钮后命令立即出现"的断言因 legacy fail-closed 有意失效，已改为"先出现门禁面板 → 点击确认 → 命令出现"，属行为契约要求的更新而非绕过。
- 残余风险：内存 token 在进程重启后全部失效（符合"重启失效"契约，代价是重启后需重新确认）；多实例部署（如有）下 token 不共享——当前产品为单进程服务，不受影响。
- 未执行 git commit/push。
