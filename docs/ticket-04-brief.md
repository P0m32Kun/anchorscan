# 任务书：在所有命令出口强制 safety 与 review 门禁（Ticket 04）

**本文件即任务书。** 先读仓库根 AGENTS.md，再执行本文件。这是本特性最困难的一票：服务端统一授权 gate + 风险确认 UI，涉及所有命令出口，不允许出现绕过路径。

## 背景

知识库 v2 对接已完成：loader 消费 catalog v2（`version:2, source:handbook-v3`），`Entry` 携带 `Safety`（mode/effects/cleanup）、`ReviewStatus`（stable/needs-review/legacy-unknown）、`Sources`/`Generated`；legacy Markdown 条目 fail-closed 为 legacy-unknown。现在要在**所有**向用户暴露命令的 HTTP 出口上强制服务端门禁。

## 必读

1. `docs/plans/catalog-json-knowledgebase/spec.md`（重点 5 节门禁语义）
2. `docs/plans/catalog-json-knowledgebase/tickets/04-server-safety-and-review-gates.md`（行为契约与验收，逐条满足）
3. `internal/knowledgebase/`（Entry 模型、Safety/ReviewStatus 字段）
4. `internal/web/`（report_handler.go、workbench.go、/tools 预填）与 `internal/report/`（命令构造层）
5. `docs/testing-strategy.md`（选最低足够测试 seam）

## 行为契约（摘自 ticket 04，逐条实现）

- 先用引用搜索枚举**所有** `Entry.Commands` 外部输出点并在报告中列出清单；每个点复用同一个 gate，不能以新 handler 绕过。
- `stable + safe` 正常返回命令。
- `needs-review + safe` 显示待复核状态，并要求服务器验证的 acknowledgement。
- `optional` 显示 authentication-attempt 并要求显式确认。
- `manual-gated` 显示完整 effects 与 cleanup，并要求显式确认。
- `legacy-unknown` 以至少 manual-gated 的强度要求确认，并明确说明旧 Markdown 未声明 safety。
- safety 或 command 缺失/非法时不返回 raw args、full command 或 tool link。
- gate 根据服务器当前 catalog 条目**重新计算**条件；客户端不能通过伪造 mode/effects/cleanup 或 query 参数提升权限。

## 铁律

- 安全确认必须服务端强制：手工构造的未确认 HTTP 请求不能获得任何可执行命令或 /tools link（用 httptest/ curl 证明）。
- 不引入账号、审批流、长期授权或自动扫描（ticket 非目标）。
- 不改变 knowledgebase loader 与 report 命令构造的既有行为（ticket 02/03 已验收）；门禁加在 HTTP 暴露层。
- 禁止 git commit/push；不为未来里程碑做占位实现。
- UI 观感：与现有页面风格一致，不引入新前端框架/依赖；实机视觉终验归编排方，报告把纯视觉项列为"未实测"。

## 验收（全部实测，报告给命令与输出摘要）

```bash
go test ./internal/...
go build ./...
```

- [ ] HTTP 直接调用（httptest 或起服务 curl）证明：未确认请求对 optional/manual-gated/legacy/needs-review 拿不到 raw args、full command 或 tool link；确认后可获得；伪造 query 参数不能提升权限。
- [ ] optional/manual-gated/legacy 确认页准确呈现 effects 与 cleanup（legacy 明确"旧 Markdown 未声明 safety"）。
- [ ] needs-review 状态在命令流程和 KB 详情中可见且不消失。
- [ ] 报告中列出全部命令出口清单及各自的 gate 接入点（附引用搜索证据）。
- [ ] 代表性 Playwright smoke（safe 正常、manual-gated/optional 确认、needs-review acknowledgement）保留截图/trace/console 与 server 诊断，存 `docs/reports/ticket-04-playwright/`。若本地 Playwright 环境不可用，诚实标注并降级为 httptest 覆盖 + 说明，不得伪造截图。
- [ ] 报告实测/fake 分列，写到 `docs/reports/ticket-04-report.md`。
