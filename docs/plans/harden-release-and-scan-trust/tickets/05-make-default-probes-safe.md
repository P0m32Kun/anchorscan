# 05 — 让默认自动探测保持 safe

**What to build:** 默认扫描不再自动运行 brute、default-login、用户枚举或同等级主动探测，并让 UI/报告准确说明默认风险边界。

**Blocked by:** 04 — 保护本地 Web 控制面。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 默认 NSE 规则不包含 brute、用户枚举或可能触发账号锁定的脚本。
- 默认 nuclei 命令全局排除 default-login、brute、fuzz、dos 等不属于 safe 自动检查的模板类别。
- 单工具 raw args 仍允许授权操作者显式运行主动检查，但必须显示风险提示并保留审计记录。
- Profile 继续只表达速度；本 ticket 不增加 ProbePolicy 抽象。

## 测试 seam

- Rule/command unit：固定默认配置生成命令，断言危险脚本/tag 不存在。
- Web handler：raw args 风险提示或确认契约。

## 验收

- [ ] 先为 `pgsql-brute`、`smtp-enum-users` 和 nuclei default-login 写失败测试。
- [ ] 最小修改默认规则与全局 exclude tags。
- [ ] 更新 README、配置注释和报告说明，不再把主动探测描述成默认低风险行为。
- [ ] 保存 raw tool run 的操作者参数事实，同时避免在客户报告泄露 secret。
- [ ] 聚焦测试、`make test`、`go vet ./...` 通过。

## 非目标

- 不实现自动凭据扫描、secret storage 或主动策略 DSL。
