# 加固发布完整性与扫描可信度：技术设计

## 约束

- 复用现有 `config` loader、`app.PrepareScan`、`tools.Runner`、SQLite Store、Progress 事件和报告聚合。
- 优先使用 Go 标准库，不为单一实现增加接口或框架。
- 新模块必须隐藏真实复杂性，而不是只搬运字段。
- 数据库 schema 或持久化语义变化必须有 migration 和 store seam 测试。

## 1. 发布归档与版本

`Makefile` 继续作为 build/package 的唯一入口。发布包显式复制运行时资源，不使用“复制整个 config 目录”的宽泛规则，避免携带本机配置或 secret。

最小归档资源清单：

- `config/default.yaml.example`
- `config/nse.yaml`
- `config/service-tags.yaml`
- `config/ports-highrisk.txt`
- `config/ports-top1000.txt`
- `tools/docx-render` 的锁文件、renderer 和正式模板
- README 与部署文档

`internal/version.Version` 改为默认 `dev` 的变量，由 Makefile 使用 `-X` 注入。归档测试必须解包后验证文件清单、loader 行为和二进制版本，不能只断言 Makefile 文本。

## 2. 资源诊断

规则 loader 区分三种情况：

- 显式关闭的可选能力：允许为空并产生 warning；
- 已启用但文件不存在、不可读或解析失败：fail；
- 已加载且规则非空：ok。

诊断模型从布尔值深化为 `ok/warning/fail`，但不引入通用诊断框架；只在现有 doctor/preflight 模型上增加满足用户决策所需的最小状态。

## 3. Scan Scope

`ScanScope` 是本计划新增的主要深模块，负责：

- 解析和规范化 IP、IPv6、CIDR；
- 保存 include/exclude prefix；
- 判断地址是否允许；
- 估算 Scope 大小并应用限制；
- 生成外部工具参数；
- 对外部工具返回地址执行后置过滤；
- 生成稳定 snapshot。

实现优先使用 `net/netip`。不得为计数展开大 prefix。Nmap adapter 必须接收已经验证的 Scope 值，rustscan/httpx/nuclei 等后续阶段只能接收 Scope 允许的已发现地址。

## 4. Web 控制面

在 `server` 最外层统一包装：

- 标准库同源保护；
- 安全响应头；
- 状态修改请求的统一拒绝逻辑。

`runWeb` 使用显式 `http.Server` 设置 timeout，并在启动前验证 listen address 为 loopback。请求体大小在 handler 解析之前通过 `http.MaxBytesReader` 限制；multipart 的 `maxMemory` 不被视为硬上限。

## 5. 受控默认凭据检测

第一阶段不创建 ProbePolicy 抽象：默认流水线保留非 SSH 服务的弱口令/默认凭据检测；SSH 排除官方大字典 `default-login` 模板，改用 AnchorScan 自带的 `ssh-mini-brute` 模板通过 `-t` 精确调用，字典固定为 2 用户 × 2 密码（最多 4 次尝试），并设置 `stop-at-first-match`。

非 SSH 服务的 nuclei tag 仍可能命中各自的 `default-login` 模板；全局默认只排除与凭据检测无关的 `fuzz`/`dos` 类别，不额外排除 `default-login`/`brute`。单工具原生参数（`--args` / Web raw args）继续作为专家显式入口，调用时记录审计事件并保存在 `ConfigSnapshot`，但工具运行默认不纳入客户报告，避免泄露敏感参数。

如果后续 ticket 证明需要自动 active policy，必须先更新 spec，再增加与 Profile 独立的风险选择和 Run snapshot。

## 6. Run provenance

在现有 Run/Artifact 模型附近增加最小 execution manifest，内容包括版本、规则哈希、Scope snapshot、脱敏参数和 Artifact SHA-256。manifest 由应用层聚合，Store 只负责持久化，不让 report 包读取外部工具或配置文件。

脱敏先覆盖已知 secret 参数和 URL userinfo；未知 raw args 不进入面向客户的 HTML/DOCX。

## 7. Backup

备份入口只在没有活动 Run Lease 时执行。SQLite 使用受支持的一致 snapshot 机制，Evidence 和必要配置随后写入临时目录，最后原子重命名归档。manifest 记录相对路径、大小和 SHA-256。

恢复先解包到临时目录并验证 manifest，再替换目标数据。第一版不实现增量备份、加密或远程对象存储。

## 8. Discovery、Progress 与取消

主机发现增加明确模式，而不是隐式���测：

- `auto`：现有 alive discovery；
- `assume-up`：跳过 `-sn`，直接对 Scope 执行后续发现。

事件 API 使用单调事件 ID 和 `after_id` 查询，只返回新增事件。nmap heartbeat 通过 `Progress.Emit` 持久化。

进程树清理由 OS adapter 处理；不把平台判断散布到每个工具。只有真实平台测试证明必要时才增加平台文件。

## 9. CI 与发布

PR 保持确定性单元/集成/Playwright。真实工具实验室只用于定时和发布。Release 在归档构建后运行 package smoke，并生成 SHA-256 checksum。

供应链检查优先复用标准工具，不引入新的 task runner。

## 10. Workbench

保持 Vue island。先修复 Verification 创建后 Evidence 上传部分失败的恢复模型，再沿真实变化轴抽取：

- 纯过滤/分组函数；
- DTO/API client；
- Verification、Negative Verification、Command dialog。

不为拆文件而拆 CSS，不引入全局状态库。

## 11. 文档与清理

README 是 quick start，deploy 只保留部署差异，project-status 是当前基线，ADR 索引是决策导航，archive 是历史。根 `STATE.md` 不再作为并行状态源。

死代码删除必须有全仓引用检索和聚焦测试证据；不批量删除历史研究或必要 sidecar。
