# Ticket 04 验收报告：所有命令出口的服务端 safety 与 review 门禁

- 日期：2026-08-05
- 范围：`docs/ticket-04-brief.md`、`docs/ticket-04-approval.md`、`docs/ticket-04-continuation.md`、catalog v2 spec 第 5/6 节、ticket 04
- 风险等级：高（安全门禁，覆盖全部命令暴露路径；服务端授权边界）
- 实施状态说明：gate 主体与 web 层接入由上一轮（gpt-5.6-sol，402 中断前）完成，并由编排方提交为 `31f0021 feat(web): server-side safety and review gates on all command exits (ticket 04)`（2026-08-05 22:50:59 +0800）。本轮（kimi k3-256k）对照批准书逐项核查时发现一处与批准书第 2 条不符的遗留：`/tools/{tool}?raw_args=...` 手工构造的 query 参数仍会回显预填（旧版报告草稿声称"raw_args query 参数不再生效"，但当时代码并未实现）。本轮已收紧为**手工 raw_args query 一律忽略、预填只经一次性 gate_token 链路**，并补测试锁定。其余代码未改动。验收命令均在最终代码上实测。

## 改动摘要（提交 31f0021 + 本轮 raw_args 收紧）

- 新增 `internal/web/safety_gate.go`：统一 `enforceCommandGate` 入口 + 内存态一次性 challenge/prefill token store（32 字节随机、TTL 5 分钟、消费即失效、进程重启失效、按请求指纹绑定）。
- 三个命令 JSON 出口全部复用同一 gate：`reportCommand`（单条）、`reportBatchCommand`（批量 nuclei/nmap/msf）、`projectCandidateCommand`（workbench candidate）。
- `/tools/{tool}` 预填只经服务端一次性 prefill grant：服务端只在 gate 通过后签发 `gate_token` 链接（`toolLink`），tool 页消费 token 恢复 raw args 与项目上下文；伪造/重放 token 不预填；**手工 `?raw_args=` query 参数一律忽略**（本轮收紧，`TestToolPrefillGrantIsOneTimeAndIgnoresForgedToken` 锁定）。普通手工工具页用户自行在表单填写参数的能力（含 preset 按钮前端填充 textarea）不受影响。
- KB 详情页（`/kb/{id}`）：仅 `stable + safe` 显示原始命令，其余隐藏并提示走命令流程；复核状态徽标常显，needs-review/legacy 面板明确风险文案。
- 前端 `ReportInteractions.vue` / `Workbench.vue`：428 → 渲染五档门禁面板（level/review/safety/effects/cleanup/确认按钮文案），确认后带 `gate_token + acknowledge=1` 重放同一请求；命令到达后门禁面板保留状态行（needs-review 不消失）。
- 测试：`safety_gate_test.go` 新增 7 个测试；`report_handler_test.go` / `workbench_test.go` 全部命令断言改走 `postCommandWithGate`（自动断言 428 无泄漏并完成确认链路）；`scripts/web-smoke.mjs` 既有回归补 legacy gate 确认步骤；新增 `scripts/ticket-04-web-smoke.mjs` 并挂入 `npm run test:web`。

## 命令出口清单与 gate 接入点（引用搜索证据）

任务书要求"先用引用搜索枚举所有 `Entry.Commands` 外部输出点"。证据命令与结果：

```bash
rg -ln 'Commands\.(Nuclei|NmapNSE|Metasploit)' --type go
```

```text
internal/knowledgebase/parse.go        # Markdown loader 写入（非出口）
internal/knowledgebase/json.go         # JSON loader 写入（非出口）
internal/report/vulnerability_command.go  # 命令构造层（消费 match.Commands.*）
internal/report/vulnerability_delivery.go # 仅 CanBatch* 布尔存在性（120-122 行，无命令内容）
internal/web/report_handler.go         # 215 行存在性判断 + 构造调用
internal/web/safety_gate.go            # commandForTool 读取（gate 内部）
```

```bash
rg -n 'BuildNucleiCommand|BuildNmapCommand|BuildMSFCommand|BuildBatchNucleiCommand|BuildBatchNmapCommands|BuildBatchMSFCommands|BuildCandidateCommands' --type go -g '!*_test.go'
```

构造层全部调用点仅在 `internal/web/report_handler.go`（347/387/397/492/494/496 行）与 `internal/web/workbench.go`（478 行）——即三个已接入 gate 的 handler；无其他调用方，无绕过 handler。

| # | 出口 | 位置 | gate 接入点 |
|---|------|------|------------|
| 1 | `POST /reports/{runID}/commands` 单条命令（full_command/tool_args/tool_link JSON） | `report_handler.go` `reportCommand` | `report_handler.go:269` `enforceCommandGate`（构造命令之前） |
| 2 | `POST /reports/{runID}/commands/batch` 批量命令（nuclei/nmap/msf 三分支） | `report_handler.go` `reportBatchCommand` | `report_handler.go:331` `enforceCommandGate`（在 MSF/Nmap/Nuclei 分支之前统一拦截） |
| 3 | `POST /projects/{id}/candidates/{key}/commands` workbench 候选命令 | `workbench.go` `projectCandidateCommand` | `workbench.go:469` `enforceCommandGate`（`BuildCandidateCommands` 之前） |
| 4 | `GET /tools/{tool}` catalog 命令预填 | `tools.go` `toolPage` | `tools.go:80-93` 仅消费服务端签发的一次性 `gate_token` prefill grant；手工 `raw_args` query 一律忽略。签发点全部位于 gate 通过之后：`report_handler.go:284`（单条）、`:377`（批量 nuclei）、`:431`（批量 nmap）、`workbench.go:486`（candidate） |
| 5 | `GET /kb/{id}` KB 详情原始命令展示 | `knowledgebase.go` + `templates/knowledgebase_detail.html:34-41` | `ShowCommands = stable && safe`，其余隐藏并显示门禁提示；needs-review/legacy 面板常显状态 |

非出口（已核查，不泄漏命令内容）：

- 报告页命令按钮可见性 `commandTools`（`report_handler.go`）：只输出工具名与不可用原因；`CanBatchNuclei/Nmap/MSF`（`vulnerability_delivery.go:120-122`）仅为存在性布尔。
- workbench JSON API（`projectWorkbenchAPI`）：`ProjectVulnerabilityCandidate`（`internal/report/project.go:122-134`）无 Commands 字段；`negativeFingerprintGroup.NmapCommand/NucleiCommand` 由本地 `nse.yaml`/`service-tags.yaml` 规则构造，非 `Entry.Commands`，属既有负向验证建议，不在本票范围。
- HTML 报告导出（`EnrichFindingsWithVulnerabilityDetails`，`vulnerability_delivery.go:51-70`）：只写 Description/Remediation，不含命令。
- docx 导出：`rg 'Command' internal/report/docx_context.go` 无命中。
- CLI（`cmd/anchorscan/`）：`rg 'Commands|catalog.Match' cmd/anchorscan/*.go` 无命中（无 catalog 命令出口）。

## 门禁语义与防绕过设计

- 五档判定全部由服务端按当前 catalog 条目重新计算（`commandGateViewForEntry`）：`stable+safe` 直通；`needs-review+safe` 要求 acknowledgement；`optional` 展示 authentication-attempt；`manual-gated` 展示全部 effects/cleanup；`legacy-unknown` 至少 manual-gated 强度并明示"旧 Markdown 未声明 safety"。
- safety/command 缺失或非法（含 `verify`/`command` 不一致、effects 白名单外、safe 携带 effects、file 类 effects 缺 cleanup、legacy 交叉不一致等）→ 422，不返回任何 raw args / full command / tool link。
- 未确认请求 → `428 Precondition Required` + 一次性 challenge；确认请求必须携带 `gate_token + acknowledge=1`；token 按 `action/tool/key/fingerprint/entryID` 绑定，消费即失效，伪造 query 参数（`safety_mode=safe`、`mode=safe`、`effects`、`cleanup`、`confirmed=1` 等）不参与判定。
- 前端只解释风险与提交确认，无任何前端侧放行逻辑。

## 实测验收（每条命令实际退出码）

验收时的树状态：ticket-04 主体已提交（31f0021）；工作区另有他人并行进行的 ticket-05 WIP（`Makefile`、`config/catalog.json`、`internal/config/init.go`、`scripts/package_smoke_test.go` 等，22:54 起陆续写入），不属于本票范围。本轮先在该树上完成一轮全量验收（全绿），随后实施 raw_args 收紧并在**最终代码上重跑全部验收命令**（下表退出码均为收紧后的最终结果）。

| 命令 | 退出码 | 摘要 |
|---|---|---|
| `go test ./internal/...` | 0 | 14 个测试包全部 ok（version 包无测试文件），含 `internal/web` 门禁测试 |
| `go build ./...` | 0 | 无输出 |
| `go vet ./...` | 0 | 无输出 |
| `npm run typecheck:web` | 0 | `vue-tsc --noEmit` 通过 |
| `npm run test:web` | 0 | `web-smoke.mjs`（既有回归）+ `ticket-04-web-smoke.mjs` 均 PASS |
| `make pr-check` | 0 | test + doc-check + docx-test + build + package-test + web-smoke 全链路 |
| `go test ./internal/web/ -run 'TestCommandGate\|TestToolPrefill\|TestLegacyCommand\|TestKnowledgeBaseDetail' -v` | 0 | 7 个 gate 测试全 PASS（见下） |
| `go test ./internal/web/ -run 'TestReportNucleiCommandGeneration\|TestReportBatch\|TestWorkbenchCandidateCommand' -v` | 0 | 5 个命令 handler 测试全 PASS |

`make pr-check` 过程说明：首轮（收紧前）曾在 web-smoke 步骤失败一次（`scripts/web-smoke.mjs:581` `response.json()` 报 Playwright 协议错误 `No resource with given identifier found`，位于 verification/evidence 上传断言，与 ticket-04 改动无关）；单独重跑 `make web-smoke` EXIT=0，再次完整 `make pr-check` EXIT=0；raw_args 收紧后最终一轮 `make pr-check` 亦 EXIT=0。判定为既有回归脚本的瞬态竞态（环境 flake），非代码失败。

## HTTP 直接调用证据（httptest，服务端强制、不可绕过）

全部在 `internal/web/safety_gate_test.go` + handler 测试中实测通过：

- `TestCommandGateEnforcesCatalogSafetyAndOneTimeAcknowledgement`：对 needs-review/optional/manual-gated 三条目，携带伪造字段（form：`mode=safe&effects=&cleanup=attacker-controlled&confirmed=1`；query：`safety_mode=safe&review_status=stable`）的未确认 POST 仍返回 428，响应体不含 `full_command`/`tool_args`/`tool_link`/`raw_args`/`nuclei -t`；gate 视图内容由服务端 catalog 重算（level/effects/cleanup/message 逐项断言）；确认后 200 且返回 full_command 与 tool_link；同一 token 重放 → 403。
- `TestCommandGateRejectsMissingOrInvalidCatalogDataWithoutLeakingCommand`：`command` 与 `verify` 不一致、safety 非法（safe 携带 effects）两类条目 422/400，响应无命令泄漏。
- `TestCommandGateTokensAreRequestBoundAndRestartLocal`：token 跨请求 key 无效；新 store（模拟重启）中旧 token 无效。
- `TestToolPrefillGrantIsOneTimeAndIgnoresForgedToken`：tool_link 仅含 `gate_token=`（不含 `raw_args=`）；首次消费 200 且预填参数；重放同一链接预填消失；`gate_token=forged&raw_args=...` 伪造请求不预填；**仅带 `?raw_args=` 的手工构造 URL 同样不预填**（本轮补的锁定断言）。
- `TestLegacyCommandAndKnowledgeBaseDetailFailClosed`：legacy Markdown 条目命令流程返回 `legacy-unknown` 档（message 含"旧 Markdown 未声明 safety"），确认后得命令；`/kb/{id}` 显示 legacy 说明与状态且不含原始命令字符串。
- `TestKnowledgeBaseDetailKeepsNeedsReviewVisibleAndHidesGatedCommands`：KB 详情对 safe 正常显示命令；needs-review/optional/manual-gated 均显示状态与 effects/cleanup 且不泄漏命令。
- `TestReportBatch*` / `TestWorkbenchCandidateCommandGeneratesToolLink` / `TestReportNucleiCommandGeneration*`：单条、批量（nuclei/nmap/msf）、workbench candidate 三出口全部经 `postCommandWithGate`（428 无泄漏 → token 确认 → 200）完成，证明三出口复用同一 gate。

## Playwright 代表流（实机）

`scripts/ticket-04-web-smoke.mjs` 对真实二进制 + 真实浏览器实测（catalog v2 fixture 四条目：safe/optional/manual-gated/needs-review）：

- safe：对话框直接显示绑定后的完整命令；
- optional：确认前断言无 `nuclei -t network/optional.yaml` 泄漏，面板显示 `authentication-attempt` 与 cleanup，确认后命令出现；
- manual-gated：确认前断言无泄漏，面板显示 `file-read`/`test-file-create` 与 cleanup，确认后命令出现；
- needs-review：确认前断言无泄漏，面板显示待复核，acknowledgement 后命令出现且 needs-review 状态仍可见。

产物（`docs/reports/ticket-04-playwright/`）：截图 7 张（safe-command、optional/manual/needs-review 各确认前后）、`trace.zip`、`console.log`、`server.log`、`result.txt`。`result.txt` 内容：

```text
PASS: safe, optional, manual-gated, and needs-review command flows
```

`console.log` 仅含 3 条预期 428（Precondition Required）拦截记录，无其他浏览器错误。产物刷新历史：编排方首轮 22:47；本轮验收中 `npm run test:web` / `make web-smoke` / `make pr-check` 多次重跑（含 raw_args 收紧后 23:05 的最终刷新），均为 PASS。

## 实测 / fake 分列

- 实测：上表全部 Go 测试、构建、vet、typecheck、两个 Playwright smoke、`make pr-check` 全链路、HTTP 直接调用（httptest）证据。本报告无任何 fake/编造项。
- 未实测（按批准书约定列出）：纯视觉观感（面板配色/间距与既有风格的一致性）归编排方实机终验；截图已存 `docs/reports/ticket-04-playwright/` 供核对。
- 降级标注：无。本地 Playwright 环境可用，未降级。

## 残余风险

1. challenge/prefill token 为内存态、TTL 5 分钟、重启失效——批准书认可的设计；服务长期运行后过期 token 由惰性清理回收，无持久化泄漏面。
2. 提交 31f0021 中含一份上一轮的报告草稿（HEAD 版本），其声称"raw_args query 参数不再生效"与当时代码不符；本报告取代之，对应代码收紧已在本轮完成并测试锁定（见改动摘要）。
3. `scripts/web-smoke.mjs:581` 存在 Playwright `response.json()` 瞬态竞态（本轮首次 `make pr-check` 命中一次，重试即过），与 ticket-04 无关，建议后续独立处理。
4. 工作区当前含 ticket-05 并行 WIP（catalog 打包/配置页，另有 `CHANGELOG.md`/`README.md`/`docs/deploy.md` 等改动），本票验收在其存在下全绿；ticket-04 主体已固化于提交 31f0021，本轮仅有 `tools.go` 收紧 + `safety_gate_test.go` 断言 + 本报告三处工作区改动。
5. `needs-review` 状态在确认后仍展示于门禁面板与 KB 详情（设计内"不消失"），若未来产品希望确认后弱化展示需另行立项。
