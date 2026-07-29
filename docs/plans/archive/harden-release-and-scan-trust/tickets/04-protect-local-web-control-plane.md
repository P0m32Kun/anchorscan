# 04 — 保护本地 Web 控制面

**What to build:** 统一阻止跨站状态修改、非 loopback 监听和无界请求体，并为 HTTP server 设置合理 timeout。

**Blocked by:** 03 — 建模并执行授权 Scan Scope。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 所有非安全 HTTP 方法经过统一同源保护；来自不匹配 Origin 的请求被拒绝。
- 默认 Web 只允许 IPv4/IPv6 loopback listen address。
- 普通表单、JSON、Nmap XML 和 Evidence multipart 都有明确硬上限。
- `http.Server` 设置 header/read/write/idle timeout。
- 同源 Go template 表单和 Vue fetch 保持原行为。

## 测试 seam

- `httptest`：恶意 Origin、同源请求、缺失 Origin 的受控兼容规则和超限 body。
- CLI command：loopback/non-loopback listen validation。
- Playwright：一个代表性同源提交成功流程。

## 验收

- [x] 先写跨站 POST 当前被接受的失败测试。
- [x] 优先使用 Go 标准库同源能力，在 server 外层一次实现。
- [x] 先写超限 Nmap XML/multipart 会继续解析的失败测试，再加 `MaxBytesReader`。
- [x] 非 loopback listen 在启动前明确失败。
- [x] 增加最小安全响应头，不破坏现有内联资产策略。
- [x] 聚焦测试、`make test`、`go vet ./...`、`make pr-check` 通过。

## 验收记录

- 外层使用 `http.CrossOriginProtection`，拒绝恶意 Origin 与 `Sec-Fetch-Site: cross-site`，同时保留缺失浏览器来源信号的本地 CLI/表单兼容路径。
- 监听地址在打开数据库前限制为 IPv4/IPv6 loopback 或 `localhost`；显式 `http.Server` 设置 header/read/write/idle timeout。
- 所有非安全请求具有 10 MiB 外层硬限制，Nmap XML multipart 另有 8 MiB 专用限制，Evidence 保持 10 MiB handler 限制。
- 添加 `nosniff`，未引入会破坏既有内联资源的 CSP。
- `make test`、`go vet ./...`、`make pr-check`、LSP diagnostics 均通过；最终 Standards/Spec 双轴 review 无 blocker/high。

## 非目标

- 不增加登录、RBAC、TLS 或 `--unsafe-listen` 永久逃生口。
