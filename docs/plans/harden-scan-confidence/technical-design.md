---
change: harden-scan-confidence
role: technical-design
spec: docs/plans/harden-scan-confidence/spec.md
---

# 提升扫描可信度与运行韧性技术设计

## 设计目标

本设计在现有单机、单用户、SQLite 架构内解决三个问题：让变更可重复验证，让运行在进程异常后收敛，让报告能够证明检测引擎实际执行了什么。

实现坚持最小边界：不引入任务队列、调度器、远程 worker、事件总线或新的前端框架。SQLite 是跨 CLI/Web 进程的唯一协调事实源；现有 `context` 取消链、`tools.Runner`、Progress、报告模型和原生 Web 前端继续使用。

## 当前执行接缝

当前完整扫描由 `internal/app/scan.go` 创建 `running` Run，`internal/app/scan_targets.go` 执行多 Target fan-out，`internal/app/scan_target.go` 顺序调用 rustscan、nmap、httpx、NSE 和 nuclei。`internal/tools/runner.go` 使用 `exec.CommandContext` 执行工具，因此上层 context 已能取消直接子进程。

当前需要改变的行为边界：

- `internal/app/manager.go` 的 `activeID` 只能阻止同一 Web 进程内并发，CLI 和其他进程不受约束。
- `scanTarget` 的任一阶段错误会提前返回；`persistTargetScan` 只在整个 Target 返回后保存 Fingerprint 和 Finding，进程中断时内存中的结果会丢失。
- Run 终态只有 completed/failed/canceled，无法表达有可用报告但存在局部失败。
- 报告由 Fingerprint 和 Finding 构建，没有实际检测执行记录。
- 配置中没有工具 deadline；Runner 会一直等到工具退出或上层取消。
- PR 没有浏览器工作流门禁，真实工具 E2E 只通过 build tag 存在。

## 数据模型

### Run Lease

新增单行全局租约表：

| 字段 | 约束 | 含义 |
|---|---|---|
| `scope` | 主键，固定为 `global` | 明确本期只有一个全局执行槽 |
| `run_id` | 非空 | 当前拥有执行权的 Run |
| `owner_token` | 非空 | 本次进程持有租约的随机令牌 |
| `heartbeat_at` | 非空 UTC 时间 | 最近一次成功续租时间 |

不把租约字段塞进 `scan_runs`，因为租约是可替换的执行所有权，而 Run 是不可替换的历史记录。独立表让条件释放和过期接管保持单行原子操作，也避免为每个历史 Run 保存无意义的心跳字段。

所有者令牌使用标准库生成的不可预测随机值。租约 store API 接受显式 `now`，便于测试过期行为而不依赖 goroutine 睡眠或全局假时钟。

默认心跳间隔为 5 秒，过期窗口为 30 秒，先作为代码常量而非用户配置。只要没有真实环境证明需要调节，就不增加新的配置表面。

### DetectionCheck

新增 `detection_checks` 表，以 `(run_id, ip, port, protocol, engine)` 作为唯一事实键：

| 字段 | 含义 |
|---|---|
| `run_id` | 所属 Run |
| `ip` / `port` / `protocol` | 与 Fingerprint 相同的自然键 |
| `engine` | `nse` 或 `nuclei` |
| `status` | `running` / `completed` / `skipped` / `failed` / `canceled` / `interrupted` |
| `reason_code` | 稳定机器原因；成功时可为空 |
| `detail` | 面向操作者的简短详情或错误信息 |
| `started_at` | 实际开始时间；直接 skipped 时可为空 |
| `finished_at` | 进入终态的时间 |

当前 Fingerprint store API 不暴露数据库 ID，报告也以 IP/端口/协议关联 Finding。DetectionCheck 复用同一自然键，避免仅为关联新增 Fingerprint ID 传播和迁移。写入使用 upsert，使 `running` 能被同一检查的终态原地替换。

初始稳定原因码保持小而明确：

- `no_matching_rule`：当前配置没有适用规则。
- `tool_unconfigured`：检测工具未配置。
- `missing_target`：缺少引擎所需的 URL 或目标信息。
- `command_failed`：工具命令返回错误。
- `invalid_output`：工具输出无法解析。
- `artifact_failed`：原始输出无法保存。
- `persistence_failed`：检测结果无法可靠写入数据库。
- `run_canceled`：操作者取消 Run。
- `lease_expired`：租约过期或执行所有权丢失。

`detail` 可以变化并包含具体错误；业务逻辑、筛选和测试只依赖状态与原因码，不解析展示文本。

### 兼容迁移

数据库迁移按现有版本机制追加，不修改旧 migration。Run Lease 与 DetectionCheck 分成顺序版本，便于独立回滚代码。

升级后的首次协调会把没有可续租所有者的遗留 `running` Run 标记为 `interrupted`。这是旧版本兼容的一次性收敛；正常新版本运行只允许“租约已过期”触发 interrupted。

## Run Lease 协议

### 获取

完整扫描和单工具入口在输入校验、配置加载和 preflight 完成后，执行工具前获取租约：

1. 生成 `run_id` 和 `owner_token`。
2. 在 SQLite transaction 中读取 `global` 租约。
3. 没有租约时插入并成功。
4. 租约仍新鲜时返回冲突，包含活动 Run ID；不创建排队项，也不创建新的 `running` Run。
5. 租约已过期时，把旧 Run 和其仍为 `running` 的 DetectionCheck 标记为 `interrupted`，然后用新所有者替换租约。
6. 成功获取后保存新的 `running` Run；若保存失败，使用 owner token 条件释放租约。

Web `Manager` 可保留 `activeID` 和 cancel function，用于本进程状态与即时取消，但它不再承担并发正确性。CLI 直接调用应用入口时也经过相同租约协议。

### 续租与所有权丢失

Run 启动后开启一个轻量 heartbeat goroutine，每 5 秒执行带 `scope + run_id + owner_token` 条件的更新时间：

- 更新一行表示仍拥有租约。
- 更新零行表示所有权已经丢失，立即取消运行 context；旧进程不得再写最终 Run 状态或删除新租约。
- 单次数据库错误只记录并在下一次 tick 重试；若从最后一次成功续租起已超过过期窗口，则本地取消 context，避免在无法证明所有权时继续启动新工具。

heartbeat 生命周期绑定 Run context，Run 结束后必须停止，不留下后台 goroutine。

### 完成与释放

正常完成、带错误完成、失败或操作者取消时，通过一个 transaction 验证 owner token、更新 Run 终态、终结仍为 `running` 的 DetectionCheck，并删除租约。旧所有者无法覆盖新所有者已经协调出的 `interrupted` 状态。

进程崩溃无法执行清理时，Run 暂时保持 `running`，直到 Web 启动协调或下一次任务获取租约时发现过期并标记 `interrupted`。本期不增加常驻 watchdog 进程。

### 取消

Web 取消继续调用当前 `Manager` 持有的 cancel function；CLI 的中断信号取消同一 Run context。`exec.CommandContext` 负责终止当前直接工具进程，fan-out worker 停止领取新 Target。

取消收敛规则：

- Run 为 `canceled`。
- 所有仍为 `running` 的 DetectionCheck 为 `canceled`，原因 `run_canceled`。
- 已经 completed/skipped/failed 的检查和已保存事实保持不变。
- 取消不是 failed，也不是 interrupted。

租约不是远程进程控制通道。Web 只对本 Web 进程拥有的 Run 显示即时取消；跨进程远程取消、信号代理和 worker 控制不在本期范围。

## Run 状态收敛

最终状态由执行事实决定，而不是仅看最后一个 error：

| 条件 | Run 状态 |
|---|---|
| 所有计划基础阶段成功，所有 DetectionCheck completed 或正常 skipped，报告成功保存 | `completed` |
| 至少有一个 Target 或 DetectionCheck 失败，但数据库中存在有效扫描结果且报告可生成 | `completed_with_errors` |
| 所有可执行 Target 的基础阶段失败，或关键持久化/报告保存失败导致无法形成有效结果 | `failed` |
| 当前所有者收到操作者取消 | `canceled` |
| 协调器发现租约过期，或所有权已被合法接管 | `interrupted` |

Run 的 `error` 字段继续保存面向操作者的简短错误汇总，不新增通用错误表。详细阶段过程继续由 ScanEvent、DetectionCheck 和 Artifact 承担。这样足以支持单机产品，不建立第二套日志系统。

单工具运行使用同一 Lease 和 interrupted/canceled 语义。因为它不是按 Fingerprint 编排两个检测引擎，本期不为单工具运行伪造 DetectionCheck；其成功或失败继续由 Run、Finding 和 ScanEvent 表达。

## 扫描流水线与部分结果

### 基础阶段和次要阶段

基础阶段决定是否能建立 Target 的有效扫描事实：

- Run 级 nmap alive sweep（启用时）。
- Target 级 rustscan 端口发现。
- Target 级 nmap 服务识别。
- 数据库中关键 Run/Fingerprint/Finding 写入和最终报告保存。

次要阶段可以降级但不得抹掉基础事实：

- httpx Web 增强。
- 单个 Fingerprint 的 NSE。
- 单个 Fingerprint 的 nuclei。
- 次要工具输出解析和制品写入。

基础阶段对某 Target 失败时，该 Target 结束；其他 Target 继续。次要阶段失败时记录 issue 或 failed DetectionCheck，然后继续同一 Fingerprint 的独立引擎以及其他 Fingerprint/Target。

### 增量持久化

为了在进程中断时保留事实，持久化时点从“Target 全部完成后”前移到阶段边界：

1. nmap 识别出 Fingerprint 后立即保存基础 Fingerprint。
2. httpx 成功后按 `(run_id, ip, port, protocol)` 更新已保存 Fingerprint 的 Web 增强字段，不重复插入。
3. ManualReview、NSE、nuclei Finding 在各自产生后立即保存。
4. DetectionCheck 在工具调用前保存 running，在结束后立即保存终态。
5. Artifact 仍在每次工具调用后立即写入。

`TargetScan` 继续作为调用结果束供当前流程和测试使用，但数据库成为中断恢复后的事实来源。实现直接复用现有 `*store.Store` 与小型保存方法，不增加通用 repository、事件总线或异步写队列。原先“scanTarget 完全纯”的边界让位于明确的数据安全需求；测试使用 fake Runner 加临时 SQLite 覆盖公共行为。

次要阶段错误在 `TargetScan` 中形成轻量 issue 汇总，供上层决定 `completed_with_errors` 和生成 Run error 摘要；不会作为立即中止整个 Target 的普通 error 返回。基础失败仍通过 error 返回。

### DetectionCheck 生命周期

对每个 Fingerprint，NSE 与 nuclei 独立评估：

1. 不满足条件时直接写 skipped 和稳定原因。
2. 满足条件时先写 running。
3. 工具成功、输出可解析且结果已保存时写 completed；Finding 数量可以为零。
4. 命令、解析或 Artifact 失败时写 failed 并继续其他独立工作。
5. 操作者取消时，统一把仍 running 的检查写 canceled。
6. 过期租约协调时，统一把仍 running 的检查写 interrupted。

规则判断只决定本次要不要执行并立即形成事实。历史报告只读取 DetectionCheck 表，不根据当前 `service-tags.yaml` 或 `nse.yaml` 重新计算。

## 可选逐工具超时

配置增加全局 `timeouts` 段，包含 `rustscan`、`nmap`、`httpx`、`nse`、`nuclei`。值使用 Go duration 字符串或 `0`：

```yaml
timeouts:
  rustscan: 0
  nmap: 0
  httpx: 0
  nse: 0
  nuclei: 0
```

配置加载后统一使用标准库 `time.ParseDuration` 解析非零值，拒绝负数和无效文本。Profile 不包含 timeout，也不能覆盖全局值。配置页与 preflight 展示实际生效值，并明确 `0` 表示不限时。

调用点只使用一个小 helper：值为零时直接返回原 context；值大于零时使用 `context.WithTimeout`，并在调用结束后执行 cancel。`context.DeadlineExceeded` 归类为对应阶段失败，不归类为操作者 canceled：基础工具超时可导致 Target/Run 失败，NSE/nuclei 超时形成 failed DetectionCheck 并继续。

不修改 `tools.Runner` 接口，不增加自定义定时器、重试器或 watchdog。

## Web、报告与导出

### Run 页面

Run 状态接口增加 DetectionCheck 计数对象，按六种状态聚合。`run-status.js` 使用现有轮询节奏更新紧凑摘要；ScanEvent 日志继续提供详细时间线，不把每个 Fingerprint 的完整检查列表塞进轮询响应。

Run 状态标签增加 completed_with_errors 和 interrupted。只有当前 Web 进程拥有的 running Run 显示取消操作。interrupted 与 completed_with_errors 显示“重新运行”入口。

### 预填重新运行

重新运行入口读取原 Run 的 `config_snapshot`，只提取当前扫描/工具表单已经支持的字段并回填。用户必须进入表单、看到状态来源提示并再次提交；GET 请求或打开页面不得启动任务。

旧快照缺字段时使用当前正常默认值；字段已失效时显示表单校验错误，不猜测或静默改写。新 Run 获得新 Run ID，旧 Run 保持不可变。

### 报告

Web 报告在 Run 离开 running 后按 Fingerprint 显示 NSE/nuclei 状态、原因和详情。顶部提供：

- 双引擎：两个检查均为 completed。
- 单引擎：恰好一个检查为 completed。
- 未覆盖：没有检查为 completed。
- failed/canceled/interrupted/skipped 的独立状态计数。

只有 completed 计入“已执行完成”的引擎覆盖。failed 表示尝试过但未完成，必须在状态计数和逐项详情中可见，不能被算作成功覆盖。所有汇总只显示数量，不计算百分比。

JSON `ScanReport` 增加可选 `detection_checks` 数组并使用 `omitempty` 保持旧报告兼容。现有字段、排序和 IP/IP:PORT/URL/CSV 导出不变。独立 HTML 报告消费同一 DetectionCheck 数据并增加自包含样式，不加载 Web 运行时资源。

## 质量门禁

### PR 门禁

PR workflow 串行执行现有 Go/JavaScript 测试、构建/打包检查和 Playwright Chromium。失败即阻止合并。

Playwright 使用真实生产 Web 入口，但把配置、数据库、报告目录和工具路径指向临时目录。测试资产提供确定性的本地可执行 fixture，模拟 rustscan/nmap/httpx/nuclei 输出以及一个可取消的长运行命令；不增加生产专用 HTTP endpoint 或前端测试模式。

浏览器用例覆盖：

- 创建 Project。
- 扫描表单必填和错误校验。
- 启动 Run、状态轮询、DetectionCheck 摘要和取消。
- 导入确定性报告。
- 报告筛选、分页、复制入口。
- 配置页面展示和超时校验。
- 1440px 主流程与 1280px 关键布局可用性。

选择器优先使用 role、label、可见名称等可访问性接口；只有没有稳定语义的复杂控件才增加 `data-*` hook。测试不锁定 CSS 类或完整 DOM 快照。

Playwright 仅安装 Chromium。失败时上传 screenshot、trace 和 console log；不引入多浏览器矩阵、像素差异平台或前端框架。

### 真实工具实验室

保留现有 build-tag E2E，并扩展 Docker 场景：MySQL、SMB、SSH、未知 TCP 服务和混合目标。实验室运行真实 rustscan、nmap、httpx 和 nuclei，验证：

- 预期 Fingerprint 能建立。
- NSE/nuclei DetectionCheck 与实际工具调用一致。
- 局部失败不会丢失其他结果。
- 覆盖报告能暴露跳过和规则缺口。

真实实验室 workflow 支持人工触发和定期运行，不阻止每个 PR。发布 workflow 在生成发布制品前运行同一实验室 job；该 job 的提交、日期、工具版本、日志和测试结果构成发布通过记录。

规则修改必须引用实验室中已复现的缺口。`service-aliases.yaml` 因未被程序读取且重复代码内稳定别名而删除；不以“未来可配置”为由实现加载器。

## 测试设计

### Store 行为

- 首次获取、有效租约冲突、过期接管。
- owner token 条件续租、条件终结和条件释放。
- 遗留 running Run 的一次性 interrupted 收敛。
- DetectionCheck upsert、状态聚合、终态不可被普通更新倒退为 running。
- 历史 DetectionCheck 不受规则配置变化影响。

测试传入显式时间，不等待真实 5/30 秒窗口。

### 应用行为

- `RunScan`、`RunTool` 使用同一临时 SQLite 时不能并发获得租约。
- fake Runner 证明 context canceled 能到达当前工具调用。
- httpx/NSE/nuclei 单点失败后，其他独立引擎和 Target 继续，已有事实保留。
- 全部基础阶段成功得到 completed；局部失败得到 completed_with_errors；全部基础失败或报告不可保存得到 failed。
- canceled 和 interrupted 分别终结运行中的 DetectionCheck，且不污染 failed 数量。
- timeout 为 0 时 context 无 deadline；非零时 DeadlineExceeded 按阶段语义收敛。

### Web 与报告行为

- 真实 HTTP 请求验证运行计数、状态标签、取消权限和预填重跑。
- JSON 新字段为纯增量，旧字段和现有导出保持不变。
- Web 与独立 HTML 对相同 DetectionCheck 产生一致的覆盖分类。
- Playwright 验证用户工作流与失败诊断制品。

不测试私有 helper、具体 SQL 文本、CSS 类名、goroutine 调度顺序或 heartbeat 真实睡眠时间。

## 交付顺序与回滚

三条工作流可以各自从无阻塞 ticket 开始，但每次只实施一个 ready frontier：

1. 质量链先建立 Playwright/PR 基础，再补齐工作流矩阵。
2. 韧性链先建立 Lease 获取与所有权，再加入过期协调和 interrupted 重跑。
3. 覆盖链先持久化 DetectionCheck，再改变局部失败语义、报告和实验室规则。
4. 可选 timeout 在新状态语义稳定后接入。
5. 最终 ticket 汇合三条链，执行完整 PR 门禁、真实实验室和发布记录验收。

数据库 migration 只前进不回写。代码回滚时保留新增表和 JSON 可选字段不会影响旧读取路径；若新行为出现问题，可分别关闭 Playwright workflow、停止展示新增报告字段或回退应用协调逻辑，但不得把已标记 interrupted/canceled/completed_with_errors 的历史 Run 改写回旧状态。

## 风险与控制

- **旧进程误写新 Run**：所有 lease 更新、终结和释放都校验 owner token；失去所有权后禁止写终态。
- **heartbeat 假死判断**：心跳独立于工具调用，过期窗口远大于间隔；默认工具不设 deadline。
- **部分持久化产生重复 Fingerprint**：基础 Fingerprint 只插入一次，httpx 使用自然键更新。
- **failed 被误当覆盖**：覆盖分类只计算 completed，失败另列状态计数。
- **浏览器门禁不稳定**：不用真实工具、外网、共享端口或共享数据库；每次使用临时目录和确定性 fixture。
- **规则为指标而修改**：不输出百分比，规则变化必须先有实验室复现。
