# 未识别服务通用指纹增强研究

- 状态：结论已收敛，待质量复核。
- 日期：2026-07-29。
- 范围：只评估 `unknown`、`tcpwrapped` 和空服务名的二次识别；不修改扫描代码，也不对真实网络目标运行额外探测。

## 结论

**不建议创建“对所有未识别端口进行通用主动指纹或泛化 Nuclei tags”的 MVP。**

现有流水线已经先用 `nmap -sV` 做协议识别，再仅对确认服务执行 httpx、NSE 和按服务标签筛选的 Nuclei。以“未识别”作为触发条件会丢失这一安全前提：它不是协议，而是证据不足、连接被快速关闭，或 XML 服务字段缺失的不同事实。对它们统一重探会扩大请求数、误报面和模板选择面，却没有可验证的通用匹配规则。

当前存在一个必须如实保留的例外：默认配置启用 Dameng，`shouldProbeDameng` 会对非 Web、非有限 known-map 的弱指纹（包括 `tcpwrapped`、`unknown` 和空服务名）尝试一次 Dameng 专用握手。这不是通用 fallback，且本任务不改变它。下文的“不得自动重探”均指**不得新增任何通用或非 Dameng 的自动重探**。

可保留的后续方向是**按单一、已证明无副作用的协议另建独立任务**：该任务必须有服务维护方协议文档、窄候选条件、loopback fixture 和单独授权；不得称为 unknown fallback，也不得复用或扩大 Dameng 触发器。

## 当前本地事实

| 环节 | 证据 | 结论 |
| --- | --- | --- |
| 初次指纹 | `internal/app/scan_target.go:58-87` 调用 `FingerprintWithOutput`；`docs/project-status.md:85` 记录常规使用 `nmap -sV --version-intensity 7`。 | 已有通用、版本化服务探测层；二次层必须证明其增益。 |
| Web 增强 | `internal/app/scan_target.go:97-117` 仅在 `fp.IsWeb` 时调用 httpx；`internal/tools/httpx.go:23-27` 固定使用 `-json -silent -status-code -title -tech-detect -u <URL>`。 | 空服务、`unknown`、`tcpwrapped` 不能安全地直接作为 URL 输入。 |
| NSE / Nuclei | `internal/app/scan_target.go:149-286` 分别通过 `MatchNSE` 和 `MatchNucleiTags` 调度；无规则时持久化 `skipped/no_matching_rule`。`config/service-tags.yaml:12-15` 明确默认规则只按 tags，且需有服务/产品/技术命中。 | 默认扫描没有“unknown -> 任意 tags”的现有契约；新增该映射会破坏指纹驱动设计。 |
| Dameng | `internal/app/scan_target.go:124-138,451-488` 对非 Web、且不在已知服务排除表中的指纹调用专用握手；`internal/fingerprint/probes/dameng.go:54-60` 说明它不完成认证。 | Dameng 是单协议、固定报文的例外，不是通用探针框架。现有选择器仍会覆盖某些弱识别名称，因此不能泛化。 |
| 现场信号 | 根目录 `bug记录.md:8-11` 记录 Dameng 对空服务名端口的探测日志；`bug记录.md:22` 单独记录了大量 `tcpwrapped`。 | 两类信号共同提示探测噪声/成本风险，但不是同一批样本，不能据此支持扩大探测。 |
| 已有单元测试 | `internal/fingerprint/probes/dameng_test.go` 覆盖本地 listener 的命中路径；未发现同时覆盖 unknown、tcpwrapped、空服务、已知服务四类选择/预算的 pipeline fixture。 | 尚不能声称通用方案已被验证。 |

### 本机工具与版本证据

2026-07-29 在开发机读取：Nmap `7.99`，httpx `v1.10.0`，Nuclei `v3.11.0`，public nuclei-templates `v10.4.6`。项目不把这些二进制或模板版本锁入 Go module；`config/default.yaml.example` 只配置路径、超时和 profile 参数。因此结论依赖本次观察版本，实施任务必须重新记录其实际版本与模板提交/版本。

## 三类状态必须分开

| 状态 | 可确定事实 | 禁止的推断 | 默认动作 |
| --- | --- | --- | --- |
| 空服务名 | Nmap XML 的服务字段为空，或解析后为空。 | 不是 HTTP、也不是任意常见协议。 | 保留原始 fingerprint 与 artifact；`no_matching_rule`。 |
| `unknown` | Nmap 未给出可用匹配。 | 不是“可安全尝试所有协议”。 | 保留 Nmap 原始 XML；不启动 httpx/NSE/Nuclei fallback。 |
| `tcpwrapped` | 连接快速关闭可触发 Nmap 的 tcpwrapped 判断；Nmap 源码注释说明真实响应或超时会排除该可能性。 | 不是普通未知服务，不能据此反复连接。 | 除现有 Dameng 专用例外外，不自动重探；只记录/展示为独立分类。 |

Nmap 的服务检测文档说明，版本识别使用探针/匹配数据库，得到未识别响应时可输出供人工提交的服务 fingerprint；这支持“收集证据后增加具体 Nmap probe”，不支持应用层无差别猜测。Nmap 源码中的 `tcpwrap_possible` 注释说明，超时或真实响应会使 tcpwrapped 不成立，因而它本身是连接行为的分类而非协议证据。

## 候选方案比较

| 方案 | 正确性/覆盖 | 安全与成本 | 结论 |
| --- | --- | --- | --- |
| 调高 Nmap version-intensity 或加入上游/本地特定 probe | 协议匹配由 Nmap `nmap-service-probes` 的证据驱动；能处理有可复现 banner/握手的明确协议。 | 可能明显变慢；不得因 `tcpwrapped` 重复执行。需要限制端口集合和 probe 强度。 | 仅对单协议、已取得样本时可考虑；不建立 unknown 通用规则。 |
| 对所有 unknown/httpx 试探 | httpx 支持 HTTP/HTTPS fallback、端口和多种请求探针，但这只证明 HTTP 候选，而不能证明未知 TCP 服务是 Web。 | 会对每个候选产生 HTTP 请求和重试/回退；对 tcpwrapped 还会额外连接。 | 拒绝作为默认 fallback。 |
| 对所有 unknown 运行 Nuclei network/http tags | Nuclei 的 tags 是模板过滤器，不是协议识别器；默认模板集合及其请求数可随模板版本变化。 | `-tags` 可能选中多模板；并发、重试与每模板请求使全 Run 成本无法由一个端口上限表达。 | 拒绝。仅在已识别服务的既有 tags 路由中使用。 |
| 有限的协议专用轻量探针 | 若维护方文档证明固定、只读、无认证的请求/响应，可有高置信匹配。 | 每种协议需要独立授权、deadline、请求上限、fixture、冲突和 provenance；不能抽象为“猜协议”。 | 唯一可行方向，但本研究没有找到可直接提交的候选。 |

## 安全契约（若未来单协议任务具备证据）

以下是拟议的保守准入政策，不是实测得出的性能阈值，也不是当前默认行为或对 unknown/tcpwrapped 的批准；它不改变现有 Dameng 专用例外。任何独立候选必须满足：

1. **候选条件**：仅 `unknown` 或空服务名，且端口在该协议经批准的显式 allowlist；排除 `tcpwrapped`、Web、已有 `Normalized`、有 Product 的记录。不得只按常用端口判断漏洞，也不得改变现有 Dameng 选择器。
2. **请求预算**：每端口最多 1 次 TCP 连接和 1 个固定请求；不重试；不跟随重定向；不得认证、枚举、爆破、写入或读取业务/会话/内存内容。
3. **时间与并发**：连接超时不超过 1 秒、读取超时不超过 1 秒；该 probe 的并发上限 1；全 Run 默认最多 10 个候选端口。触发任何超时、拒绝或预算耗尽后记录 `skipped`/`unknown` 并停止该类别。
4. **授权**：仅限已在 Run scope 中的资产，且操作者显式启用该具体 probe。`tcpwrapped` 需要单独显式选择和额外书面授权；默认永不重探。
5. **成功证据**：响应必须同时匹配协议维护方文档中的固定魔数/版本字段和语法；端口号、连接成功、单个模糊 banner 均不足以改写 `Normalized`。
6. **置信度与冲突**：协议特异、双字段匹配可标为 high；单 banner 最多 low 且不触发 Nuclei/NSE；与 Nmap 的已知服务冲突时保留原始 Nmap fingerprint、写入 probe provenance，并不覆盖既有服务。
7. **可审计性**：保留请求类型（不记录敏感响应正文）、响应摘要/哈希、超时、预算消耗和拒绝原因为 artifact/DetectionCheck；历史 fingerprint 和 DetectionCheck 是事实，不回算。

## 受控验证设计与当前限制

未运行真实网络扫描或新 probe。仓库现有 `TestScanTargetHidesUnmatchedDamengProbe` 和 `TestScanTargetShowsMatchedDamengProbe` 证明 `tcpwrapped` 会进入当前 Dameng 专用例外（分别为关闭连接的非匹配和固定响应的匹配），并非“默认零连接”。本研究未找到可在不修改代码的前提下覆盖 unknown、空服务名、tcpwrapped、已知服务四类的统一选择/预算 fixture；这项环境限制已记录，不能把下表当作已执行验证。后续单协议任务必须先以 Go loopback fixture 覆盖：

| Fixture | 预期 |
| --- | --- |
| unknown + 目标协议固定响应 | 仅在 allowlist、预算和显式启用同时满足时，一次请求后以匹配证据产生 enrichment/provenance。 |
| unknown + 非目标响应 | 一次请求、无 enrichment、无漏洞引擎调度。 |
| 空服务名 + 目标响应 | 与 unknown 相同；证明空值不会造成 URL/Nuclei 兜底。 |
| tcpwrapped（accept 后立即 close） | 除现有 Dameng 例外外，默认零次新 probe；任何新协议探针必须有独立授权、测试和一端口一次上限。 |
| 已知 HTTP/SSH/数据库服务 | 零次该专用 probe；保留现有 httpx/NSE/Nuclei 分支。 |
| 超时/取消/预算耗尽 | 无重试、无 goroutine 泄漏、记录可解释的 skipped/failed 事实。 |

## 一手资料

访问日期均为 2026-07-29：

1. Nmap，《Service and Application Version Detection》：https://nmap.org/book/vscan.html 。说明版本检测的探针/匹配模型、SSL 后处理与未识别 fingerprint 提交路径。
2. Nmap，`service_scan.cc`，本次读取的 `master` 提交 `508531ed`：https://github.com/nmap/nmap/blob/508531ed/service_scan.cc 。`tcpwrap_possible` 的源码注释说明真实响应或超时排除 tcpwrapped 可能性。
3. ProjectDiscovery httpx README/命令帮助，本次读取的 `main` 提交 `13037dd0`：https://github.com/projectdiscovery/httpx/blob/13037dd0/README.md 。说明 HTTP/HTTPS fallback、`-ports`、`-threads`、`-rate-limit`、重试与请求探针能力。
4. ProjectDiscovery Nuclei README/命令帮助，本次读取的 `main` 提交 `57db3835`：https://github.com/projectdiscovery/nuclei/blob/57db3835/README.md 。说明 `-tags`/`-exclude-tags` 是模板筛选，及 `-rate-limit`、`-concurrency`、`-retries`、`-timeout` 的成本控制。

本机 public nuclei-templates 仅报告版本 `v10.4.6`，不是 Git checkout，因而无法记录 templates commit；任何实施任务必须重取模板版本及对应上游提交。

## 对 PRD 验收标准的映射

- 方案在正确性、安全性、覆盖、维护和运行成本上已比较：见“候选方案比较”。
- 推荐方案的预算、超时、授权和停止条件：见“安全契约”；当前推荐为不实施通用方案。
- 证据、置信度、误报与冲突契约：见“安全契约”第 5-7 项。
- 四类样本的 fixture：已给出可执行设计，并明确记录了现有统一选择/预算 fixture 缺失这一环境限制；未虚构执行结果。
- 实施任务建议：无。只有取得具体协议样本和第一方只读协议证据后才可创建一个独立、受控任务。

## 待主代理复核

1. 已记录本次 Nmap/httpx/Nuclei 源码提交与本机工具版本；templates 只有版本号，实施前仍须重核。
2. 将该“拒绝通用 fallback”的结论回填父任务，并确认不会因未创建 MVP 而遗漏产品承诺。
3. 对本研究文档执行任务质量检查；研究任务不涉及产品代码或 TDD。
