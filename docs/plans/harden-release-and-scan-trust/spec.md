# 加固发布完整性与扫描可信度

## 状态

已批准，按 ticket 顺序实施。

## 问题

AnchorScan 的核心编排、SQLite Run Lease、DetectionCheck 历史事实和 Project Report 聚合已经形成稳定基础，但当前仍有四类会直接削弱交付可信度的问题：

1. 发布归档没有携带 NSE、nuclei 路由和端口预设等运行时资源，程序版本也与 Git tag 漂移；源码测试通过不能证明实际归档可用。
2. 扫描目标只是字符串列表，CIDR 展开前的精确字符串排除不能可靠执行授权范围，默认探测还包含可能触发账号锁定或产生高噪音的主动检查。
3. 本地 Web 控制面没有统一同源保护、监听边界和请求体硬限制，浏览器中的恶意站点可能尝试向 localhost 发起状态修改请求。
4. 报告缺少完整工具/规则 provenance，在线备份契约不完整，长扫描的事件读取和进程终止能力也需要增强。

维护层同时存在版本、部署、ADR 路径和根状态文件漂移，以及少量已有引用证据的死代码。前端 Workbench 的多请求恢复能力和职责边界也值得在不迁移 SPA 的前提下改善。

## 目标

1. 发布归档包含运行所需的全部 sidecar，并能通过解包后的契约测试。
2. Git tag、CLI、Web 和归档名称共享同一个构建版本。
3. 扫描授权范围被统一解析、规范化、限制并在所有工具调用前后执行。
4. 默认自动检查只包含低风险探测；主动凭据或枚举检查必须显式开启。
5. 本地 Web 控制面默认只监听 loopback，状态修改请求具有同源保护和大小/时间限制。
6. Run 可以追溯到工具、模板、规则、配置和 Artifact 版本。
7. SQLite、Evidence 和必要配置可以一致备份并验证恢复。
8. 长扫描提供增量事件、可见 heartbeat 和可靠的进程取消。
9. 发布门禁验证实际归档，文档与 Agent 工作流只有明确的权威入口。
10. 在不引入新框架或通用插件系统的前提下改善 Workbench 恢复能力并删除确认无引用的代码。

## 已批准的产品决策

- 产品继续定位为本机单操作者工具；本计划不引入账号、权限、远程 Web 部署或分布式 worker。
- Web 默认只允许 loopback 监听。未来 LAN/远程访问需要单独 ADR，同时设计认证与 TLS。
- 扫描速度 `Profile` 与探测风险是两个概念。默认流水线保留非 SSH 服务的弱口令/默认凭据检测；SSH 排除官方大字典模板，改由仓库提供的 `ssh-mini-brute` 模板执行最多 2 用户 x 2 密码（4 次）尝试。
- Scan Scope 支持文档承诺且能可靠验证的 IP、IPv6 和 CIDR。未明确支持的 hostname/range 输入不得以任意字符串形式传给扫描器。
- 大网段不得为了计数而完整展开到内存；Scope 保留 prefix 表示并在执行前做规模判断。
- DetectionCheck 仍表示历史执行事实，不解释为漏洞覆盖率或安全保证。
- UDP 扫描不进入默认流水线。短期修正文案和规则可达性；后续仅考虑显式 opt-in 的常用 UDP 端口。
- 保持 Go SSR + Vue islands，不迁移完整 SPA，不增加前端状态管理库。
- 保持 `tools.Runner`、`app.Progress`、具名 `TargetScan`、SQLite Run Lease 和共享 Project Report 聚合模型。
- 不引入泛型 Repository、检测插件注册表、消息队列或宽泛 `internal/domain` 包。

## 实施顺序

1. 发布资源与版本。
2. 运行时资源诊断。
3. Scan Scope。
4. 本地 Web 控制面。
5. 默认探测安全。
6. Run provenance。
7. 备份与恢复。
8. 主机发现、进度和取消。
9. 发布与 CI 门禁。
10. Workbench 可靠性。
11. 文档与 Agent 契约。
12. 有证据的死代码清理。

除非实现证明本 spec 的产品决策无效，否则 ticket 必须按上述依赖顺序实施。计划失效时先更新本 spec 和受影响 ticket。

## 测试接缝

遵循最低充分 seam，每项行为默认只在一个最低层验证：

| 风险 | 测试 seam |
| --- | --- |
| 规则解析、Scope 集合运算、参数分类 | Go unit test |
| CLI 参数和版本输出 | CLI command test |
| HTTP 同源、状态码、请求限制 | Go `httptest` |
| SQLite snapshot、迁移和恢复 | Store/integration test |
| 发布归档内容和 sidecar 接线 | Packaging integration test |
| Workbench 对话框、焦点、上传恢复 | Playwright representative flow |
| 外部扫描器真实协作和进程树行为 | Docker/OS-specific integration |

每个 ticket 采用单个垂直切片的 red-green 循环。不得为了覆盖率在 handler、浏览器和 E2E 重复同一断言。

## 总体验收

- 解包后的发布归档能够加载默认规则与端口预设，程序显示的版本与 release tag 一致。
- 启用的核心规则文件缺失或为空时，扫描在执行外部工具前失败并给出可操作诊断。
- CIDR 中被排除的单 IP/子网不会进入发现结果、后续工具调用或报告；危险范围在执行前被阻止。
- 默认自动扫描不会调用 brute、default-login 或用户枚举探测。
- 跨站状态修改请求被拒绝，非 loopback Web 监听被拒绝，超限上传不会耗尽临时磁盘。
- 报告可追溯工具、模板、规则、Scope 和 Artifact 哈希，且敏感参数被脱敏。
- 带 Evidence 的项目可在新目录恢复并重新导出 HTML/DOCX。
- 长 Run 只增量传输新事件，Web 可见 heartbeat，取消后无残留外部扫描进程。
- `make test`、`go vet ./...` 和涉及发布/Web 的 `make pr-check` 通过；真实工具变化具有对应实验室记录。
- 当前文档不存在失效的 Project 默认参数、错误版本或悬空计划入口。

## 非目标

- 多用户、RBAC、远程 Web 服务、TLS 终止和分布式执行。
- 自动认证扫描、secret vault 或 Metasploit 编排。
- 默认全 UDP 扫描、全网段自动扩展或 checkpoint 续跑。
- 通用检测插件框架、工作流 DSL、前端 SPA 迁移或视觉像素平台。
- 无引用证据的大规模 archive 删除、CSS 拆分或依赖替换。

## 执行规则

- 每次只实施一个所有阻塞项已完成的 frontier ticket。
- ticket 开始时记录当时的 `HEAD` 作为 review fixed point，但不得把提交 SHA 写死在持久化 ticket 中。
- 行为变更通过 `tdd` 在本 spec 确认的 seam 实施。
- 候选实现完成并验证后，以 fixed point 和本 spec 执行 Standards/Spec 双轴 `code-review`。
- 修复 review 发现并重新验证后，才把 ticket 标记为 `done`。
