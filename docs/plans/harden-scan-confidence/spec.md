# 提升扫描可信度与运行韧性

## Problem Statement

AnchorScan 已能完成多目标扫描、保存指纹与漏洞发现、生成 Web/JSON/HTML 报告，并具备较完整的 Go 单元与集成测试。但当前仍有三个会直接削弱用户信任的问题：

1. 日常变更缺少覆盖真实 Web 工作流的自动化门禁，真实扫描工具实验室也没有形成发布前必须留存的通过记录。现有测试很难让非程序员快速判断“哪里坏了、页面当时是什么样”。
2. Run 的执行所有权只存在于单个进程内。CLI、Web 和单工具入口可能在同一数据库上并发运行；进程退出后，历史 Run 可能长期停留在 `running`。扫描流水线遇到次要检测失败时也会过早结束，已取得的结果和后续独立检测机会可能丢失。
3. 报告只能展示 Fingerprint 和 Finding，无法回答“每个服务当时实际执行了哪些检测、为什么跳过、是否失败”。当前规则变化后也无法可靠重建历史执行事实，因此不能形成可审计的检测执行覆盖。

这三个问题共同造成一个产品风险：报告可能存在，但操作者无法快速证明这次运行是否完整、哪里降级、哪些检测没有执行，以及发布版本是否经过可重复验证。

## Solution

建立一套最小但闭环的“可信扫描”能力，由三条独立工作流组成，并在最终发布验收中汇合：

1. **质量与发布门禁**：每个 PR 自动运行 Go/JavaScript 测试、构建/打包检查和基于 Chromium 的确定性 Playwright Web 流程；真实 rustscan、nmap、httpx、nuclei Docker 实验室按计划或人工运行，发布必须引用一份通过记录。
2. **Run 韧性**：使用数据库 Run Lease 统一 CLI、Web 和单工具入口的执行所有权；通过心跳识别失联进程，将过期 Run 标记为 `interrupted`；保留已保存结果并允许用户从预填表单重新运行。次要检测失败不再终止整个 Target，最终以明确状态区分成功、带错误完成、失败、取消和中断。
3. **检测执行覆盖**：为每个 Fingerprint 持久化 NSE 和 nuclei 的实际 DetectionCheck，包括运行、完成、跳过、失败、取消和中断状态及原因。运行页展示实时汇总，报告展示逐指纹详情，并以兼容方式加入 JSON 与独立 HTML 报告。

所有工具超时均为可选的全局逐工具配置，默认值为 `0`，表示不设置 deadline。计划不通过默认超时解决可靠性问题，也不按 profile 隐式覆盖超时；重点是执行所有权、状态收敛、取消可靠性、部分结果保留和可观测性。

## User Stories

### 质量门禁

1. 作为维护者，我希望每个 PR 自动运行现有 Go 与 JavaScript 测试，以便基础行为回归在合并前暴露。
2. 作为维护者，我希望每个 PR 验证构建和发布打包路径，以便测试通过但制品无法生成的问题不会进入主分支。
3. 作为非程序员操作者，我希望 Web 流程失败时能拿到截图、trace 和浏览器控制台日志，以便不阅读代码也能判断失败发生在哪个页面和操作。
4. 作为维护者，我希望浏览器测试使用隔离的本地服务、临时数据库和确定性数据，不依赖真实扫描工具，以便 PR 门禁稳定且可重复。
5. 作为发布人员，我希望发布前有一份真实工具实验室的通过记录，以便证明核心工具链在当前版本上实际协作成功。

### Run 韧性

6. 作为操作者，我希望 CLI、Web 和单工具运行共享同一执行锁，以便同一数据库不会出现两个互相干扰的活动任务。
7. 作为操作者，我希望新任务遇到存活租约时明确拒绝，而不是排队、抢占或静默并发。
8. 作为操作者，我希望进程崩溃或被终止后，失去心跳的 Run 自动收敛为 `interrupted`，而不是永远显示 `running`。
9. 作为操作者，我希望 `interrupted` Run 保留此前保存的 Fingerprint、Finding、Artifact、DetectionCheck 和报告数据，以便故障不会抹掉已取得的证据。
10. 作为操作者，我希望从 `interrupted` 或 `completed_with_errors` Run 发起重新运行时先进入预填确认表单，以便可以检查和调整参数，系统不得自动执行。
11. 作为操作者，我希望取消操作可靠地终止当前工具进程并把 Run 与未完成 DetectionCheck 标记为 `canceled`，以便取消不被误报成工具失败或进程中断。
12. 作为分析人员，我希望 httpx、NSE 或 nuclei 的局部失败不会丢弃之前的指纹、发现和制品，也不会阻止其他独立引擎或 Target 继续执行。
13. 作为分析人员，我希望报告可用但存在 Target 或 DetectionCheck 失败时，Run 明确显示 `completed_with_errors`，而不是伪装成完整成功。
14. 作为维护者，我希望必要时能为单个工具配置超时，但默认不启用，以便只在实际环境需要时限制异常挂起，而不误杀成熟工具的长任务。

### 检测执行覆盖

15. 作为安全分析人员，我希望看到每个 Fingerprint 是否实际执行 NSE 和 nuclei，以便区分“没有发现漏洞”和“没有执行检测”。
16. 作为安全分析人员，我希望跳过项说明稳定原因，例如无匹配规则或工具未配置，以便可以修正规则或环境。
17. 作为安全分析人员，我希望失败项保留执行引擎、状态、时间和错误详情，以便定位工具或目标问题。
18. 作为安全分析人员，我希望运行页实时显示 running、completed、skipped、failed、canceled、interrupted 的汇总数量，以便无需阅读完整事件日志即可判断检测进展。
19. 作为报告使用者，我希望完成后的 Web 与独立 HTML 报告按 Fingerprint 展示检测详情，并给出双引擎、单引擎、未覆盖数量汇总，以便快速识别检测空白。
20. 作为 API/自动化使用者，我希望 JSON 报告以新增字段提供 DetectionChecks，同时保持原有字段兼容，以便旧消费者不受影响。
21. 作为维护者，我希望 DetectionCheck 保存当次 Run 的实际事实，而不是由当前规则反推，以便后续修改规则不会改写历史。
22. 作为规则维护者，我希望先通过检测覆盖数据和真实实验室复现缺口，再修改服务标签或检测规则，以便规则扩展有证据而非猜测。

## Approved Product Decisions

- 总计划名称为“提升扫描可信度与运行韧性”，包含质量门禁、Run 韧性、检测执行覆盖三条工作流，以及最终集成验收。
- `Run` 状态为 `running`、`completed`、`completed_with_errors`、`failed`、`canceled`、`interrupted`。
- `completed` 表示所有计划的基础阶段与检测均成功或正常跳过；`completed_with_errors` 表示报告可用，但至少一个 Target 或 DetectionCheck 失败；`failed` 表示无法建立或保存有效结果。
- `canceled` 只表示操作者取消；`interrupted` 只表示租约过期、进程终止或执行所有权丢失。两者不得混入失败统计。
- 不支持 checkpoint 续跑。重新运行始终进入预填确认表单，不自动执行。
- Run Lease 是全局单活动任务约束。新任务遇到新鲜租约时拒绝；只有过期租约对应的 `running` Run 才能标记为 `interrupted`。
- 不增加队列、任务抢占、分布式调度或并行 Run。
- DetectionCheck 只记录检测引擎针对 Fingerprint 的实际执行事实。本期计入覆盖的引擎是 NSE 和 nuclei；httpx 属于服务增强阶段，不计作检测引擎。
- DetectionCheck 状态为 `running`、`completed`、`skipped`、`failed`、`canceled`、`interrupted`。
- “检测执行覆盖”不是漏洞覆盖率，不输出安全保证或覆盖百分比。汇总只显示双引擎、单引擎、未覆盖数量以及各状态计数。
- 次要阶段失败应保存已有结果并继续独立工作；只有无法建立扫描事实或保存有效结果的基础失败才使 Target/Run 失败。
- 所有工具超时为全局逐工具配置，默认 `0` 关闭；不设置默认 deadline，不按 profile 自动改变。
- PR 浏览器门禁引入 Playwright，仅运行 Chromium。真实工具实验室与 PR 浏览器测试完全分离。
- 现有 IP、IP:PORT、URL、CSV 导出不变；JSON 仅新增兼容字段，独立 HTML 增加检测覆盖内容。
- 删除未被读取且与硬编码别名重复的 `service-aliases.yaml`；稳定别名继续留在代码，规则扩展继续使用现有服务标签与 NSE 配置。

## Implementation Decisions

- 复用现有扫描编排、Runner、SQLite store、Progress 事件、Web handler 和报告模型，不引入任务框架、消息队列或新的后端服务。
- 使用 SQLite 持久化 Run Lease 和 DetectionCheck，确保所有入口观察同一事实；进程内互斥只作为本进程便利机制，不能承担跨进程正确性。
- Run Lease 的获取、续租和释放必须带所有者令牌并通过原子数据库操作完成，避免旧进程误释放或覆盖新租约。
- 启动或获取租约时执行过期协调：只处理确实过期的租约，将对应 Run 和仍为 `running` 的 DetectionCheck 收敛为 `interrupted`。
- DetectionCheck 在调用检测工具前写入 `running`，结束后原地更新为终态；正常不适用的检查直接写入 `skipped`。
- 跳过和失败使用稳定机器原因码加可读详情。报告逻辑不得通过解析日志文本推断状态。
- 扫描 fan-out 继续增量持久化每个成功产生的 Fingerprint、Finding、Artifact 和 DetectionCheck。局部错误汇总后决定最终 Run 状态。
- 对已配置的非零工具超时，仅使用现有 context 取消链包裹对应工具调用；零值直接复用原上下文。
- Playwright 测试通过真实 Web HTTP 接口和临时 SQLite 运行，不添加生产环境专用测试后门。外部工具结果通过现有 Runner seam 提供确定性 fixture。
- PR 门禁失败时上传 Playwright 截图、trace 和控制台日志。真实工具实验室保留命令、工具版本、日期和通过结果作为发布证据。
- 检测规则扩展顺序固定为：先完成 DetectionCheck 与覆盖展示，再扩展 MySQL、SMB、SSH、未知服务和混合目标实验室，最后只修复已复现缺口。

## Testing Decisions

以下公共 seam 使用 `tdd`，每个 ticket 从一个失败行为开始，避免测试私有 helper、SQL 语句文本、CSS 选择器或 goroutine 时序：

1. **扫描应用 seam**：使用 fake Runner 与临时 SQLite 调用完整扫描和单工具入口，验证租约冲突、心跳、状态收敛、取消、部分失败、结果保留和 DetectionCheck 生命周期。
2. **Store seam**：验证租约的原子获取/续租/条件释放、过期判断以及 DetectionCheck 的持久化和终态更新；只断言公共存储行为。
3. **Web HTTP seam**：通过真实请求验证启动协调、运行状态计数、重跑预填、报告详情和导出兼容性。
4. **CLI seam**：通过命令入口验证跨进程冲突提示、超时配置解析与 preflight 展示。
5. **Playwright seam**：使用 Chromium、隔离 Web 服务、临时数据库和确定性 fixture，覆盖项目创建、扫描表单验证、Run 状态、取消、导入、报告筛选/分页/复制和配置页面。
6. **真实工具 E2E seam**：继续使用 build tag 与 Docker 实验室验证 rustscan、nmap、httpx、nuclei 的真实协作，不进入每个 PR 的硬门禁。

每个 PR 必须运行 Go 测试、JavaScript 测试、构建/打包检查和确定性 Playwright；发布必须附带最近一次适用于该提交的真实工具实验室通过记录。

## Acceptance Criteria

### 质量与发布

- PR 自动门禁覆盖 Go、JavaScript、构建/打包和 Chromium Playwright，失败时可下载截图、trace 和控制台日志。
- Playwright 不依赖真实扫描工具、外网或共享数据库，并能稳定复现主要 Web 工作流。
- 发布流程能指出一份包含提交、日期、工具版本和实验室结果的真实工具通过记录。

### Run 韧性

- CLI、Web 和单工具入口无法在同一数据库上同时持有有效 Run Lease。
- 新鲜租约导致新任务明确拒绝；过期租约对应的 Run 与运行中 DetectionCheck 收敛为 `interrupted`。
- 心跳、释放和过期接管不会被旧所有者令牌误操作。
- 操作者取消后 Run 与未完成 DetectionCheck 为 `canceled`，外部工具进程停止，不计入失败或中断。
- 次要检测失败后，已产生的 Fingerprint、Finding、Artifact 和 DetectionCheck 仍可用，其他独立检测与 Target 继续执行。
- 存在局部失败且报告可用时最终状态为 `completed_with_errors`；无法建立或保存有效结果时为 `failed`。
- `interrupted` 和 `completed_with_errors` 页面提供预填重跑入口，但不会自动开始任务。
- 所有逐工具超时默认关闭；只有显式非零配置才创建 deadline，并能在 preflight 中看到有效值。

### 检测执行覆盖

- 每个 Fingerprint 对 NSE 和 nuclei 均有可审计的 DetectionCheck 事实，状态和原因与实际执行一致。
- Run 页面实时显示各 DetectionCheck 状态计数；完成后报告显示逐 Fingerprint 详情。
- Web、JSON 和独立 HTML 报告显示检测执行覆盖；现有 IP、IP:PORT、URL、CSV 导出保持原行为。
- 覆盖汇总使用数量而非百分比，不暗示漏洞检测完整性或目标安全。
- 修改检测规则后，历史 Run 的 DetectionCheck 不发生变化。
- MySQL、SMB、SSH、未知服务和混合目标实验室可重复运行；规则变化均关联一个已复现缺口。

## Out of Scope

- checkpoint、断点续跑或从中断步骤继续。
- 并行 Run、任务队列、优先级、抢占、远程 worker 或分布式租约服务。
- 为成熟工具设置默认超时，或按扫描 profile 隐式设置不同超时。
- 将 httpx 计入检测引擎覆盖，或把 DetectionCheck 解释为漏洞覆盖率、安全评分或安全保证。
- 改变现有 IP、IP:PORT、URL、CSV 导出格式。
- 引入多浏览器 PR 矩阵、移动端浏览器测试、视觉像素快照平台或前端框架。
- 在没有实验室复现证据时大规模重写服务识别、NSE 或 nuclei 规则。
- 新建可配置服务别名系统、插件架构或通用工作流引擎。

## Further Notes

- 领域状态和术语以仓库根目录 `CONTEXT.md` 为准。
- 已接受的 Run Lease 与 DetectionCheck 决策见 `docs/adr/0002-use-sqlite-run-lease.md` 和 `docs/adr/0003-persist-detection-check-facts.md`。
- 具体数据库、执行流程、报告和测试接缝见同目录 `technical-design.md`。
- 三条工作流可以形成多个 ready frontier，但实施时仍严格遵守本地 tracker：一次只执行一个无阻塞 ticket。
- 若实现证明状态语义、租约模型或检测事实模型不成立，先更新本 spec 与受影响 ticket，再继续实现。
