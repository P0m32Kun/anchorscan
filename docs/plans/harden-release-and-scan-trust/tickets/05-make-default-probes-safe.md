# 05 — 受控默认凭据检测

**What to build:** 保留非 SSH 服务的默认凭据/弱口令检测，并将 SSH 凭据探测限制为仓库自定义 Nuclei 模板中的 2 用户 x 2 密码（最多 4 次）尝试；准确说明默认风险边界。

**Blocked by:** 04 — 保护本地 Web 控制面。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 非 SSH 服务的默认 Nuclei service tag 允许命中其 default-login/弱口令模板；NSE 保留既有弱口令与枚举检查。
- SSH 必须排除官方大字典 `default-login` 模板，只允许仓库自定义 `ssh-mini-brute` 的最多 4 次尝试。
- 默认 nuclei 命令不得全局排除 `default-login` 或 `brute`；仍排除与凭据检测无关的 `fuzz`、`dos` 类模板。
- 单工具 raw args 仍允许授权操作者显式运行主动检查，必须显示风险提示并保留审计记录。
- Profile 继续只表达速度；本 ticket 不增加 ProbePolicy 抽象。

## 测试 seam

- Rule/command unit：固定默认配置生成命令，断言非 SSH default-login 可达、SSH 官方模板被排除且 mini 模板预算为 4 次。
- Web handler：raw args 风险提示或确认契约。

## 验收

- [ ] 先为 SSH 官方 default-login 排除与非 SSH 默认凭据模板可达性写失败测试。
- [ ] 验证 `ssh-mini-brute` 模板存在且其尝试预算为 2 用户 x 2 密码。
- [ ] 更新 README、配置注释和报告说明，准确描述默认凭据检测与 SSH 限额。
- [ ] 保存 raw tool run 的操作者参数事实，同时避免在客户报告泄露 secret。
- [ ] 聚焦测试、`make test`、`go vet ./...` 通过。

## 非目标

- 不实现自动凭据扫描、secret storage 或主动策略 DSL。
