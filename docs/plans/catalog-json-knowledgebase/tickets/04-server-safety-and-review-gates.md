# 04 — 在所有命令出口强制 safety 与 review 门禁

**What to build:** 为报告、项目工作台、批量命令和 `/tools/{tool}` 预填链接建立统一的服务端命令授权 gate，并提供相应风险说明 UI。

**Blocked by:** 03 — 支持 canonical 命令与 Nuclei `-code` 绑定。

**Status:** done

**Execution skills:** `tdd`、`implement`、`code-review`、`frontend-design`、`ui-ux-pro-max`、`ponytail`。

## 行为契约

- 先用引用搜索枚举所有 `Entry.Commands` 外部输出点；每个点复用同一个 gate，不能以新 handler 绕过。
- `stable + safe` 正常返回命令。
- `needs-review + safe` 显示待复核状态，并要求服务器验证的 acknowledgement。
- optional 显示 authentication-attempt 并要求显式确认。
- manual-gated 显示完整 effects 与 cleanup，并要求显式确认。
- legacy-unknown 以至少 manual-gated 的强度要求确认，且明确说明旧 Markdown 未声明 safety。
- safety 或 command 缺失/非法时不返回 raw args、full command 或 tool link。
- gate 根据服务器当前 catalog 条目重新计算条件；客户端不能通过伪造 mode/effects/cleanup 或 query 参数提升权限。

## 测试 seam

- 纯 gate unit：每个 safety/status 组合；
- `httptest`：未确认拒绝、确认成功、invalid command 拒绝、所有命令 handler 复用 gate；
- Playwright：safe 正常、manual-gated/optional 确认、needs-review acknowledgement 的代表路径。

## 验收

- [ ] HTTP 直接调用证明未确认请求不能获得任何可执行命令或 `/tools` link。
- [ ] optional/manual-gated/legacy 的确认页准确呈现 effects 与 cleanup。
- [ ] needs-review 的状态不会在命令流程和 KB 详情中消失。
- [ ] 安全确认不只存在于前端；手工构造请求无法绕过。
- [ ] 代表性 Playwright smoke 保留截图、trace、console/server 诊断。
- [ ] 聚焦 Go/前端测试通过。

## 非目标

- 不引入账号、审批流、长期授权或自动扫描。
