# 02 — 对运行时资源给出可执行诊断

**What to build:** 已启用的核心规则缺失、为空或解析失败时，在执行扫描器前明确失败；doctor 能区分健康、可选降级和错误。

**Blocked by:** 01 — 完整发布运行时资源并统一版本。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 已启用的 NSE/nuclei 规则文件缺失或零规则是错误，不再静默等同于“无规则可运行”。
- 明确未配置的可选工具是 warning；配置了无效路径是 fail。
- doctor 显示已解析的工具版本和规则数量；无法获取可选工具版本不阻止与其无关的功能。
- CLI 与 Web 复用同一 preflight/diagnostic 规则。

## 测试 seam

- Config/preflight unit：临时配置和规则文件。
- Doctor command：fake executable/version output 与临时文件系统。

## 验收

- [ ] 先覆盖“规则文件不存在却返回空规则”的失败测试。
- [ ] loader 返回可分类错误，调用者不得依赖日志文本判断。
- [ ] doctor 支持 `ok/warning/fail`，进程退出语义与 fail 对齐。
- [ ] 显式配置但不可执行的 rdpscan 不再显示 ok。
- [ ] CLI/Web 扫描在调用 Runner 前报告相同资源错误。
- [ ] 聚焦测试、`make test`、`go vet ./...` 通过。

## 非目标

- 不创建通用健康检查框架。
- 不自动下载扫描器或规则。
