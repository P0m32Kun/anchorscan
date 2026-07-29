# AI 编码工作流加固

**Status:** in_progress

## 背景

2026-07-28 的 AI 编码工作流审查发现，AnchorScan 已有完整的规则、测试命令和 Trellis 基础设施，但多数关键约束仍依赖 Agent 提示词自律：`main` 无分支保护，PR CI 可被直接提交绕过；`docs/plans/` 与 Trellis 出现状态分叉；TDD/独立双轴评审没有进入默认 Trellis 状态机；任务开始和归档不检查持久证据。

审查报告是本计划的证据基线：[`docs/reports/ai-coding-workflow-review-2026-07-28.md`](../../reports/ai-coding-workflow-review-2026-07-28.md)。

## 目标

将 AI 编码流程从“文档建议”加固为“本地状态机 + 远端分支保护 + 可验证证据”的闭环，使行为变更无法在缺少计划、测试、独立评审和 PR 门禁时进入 `main`。

## 已批准的范围决策

- 优先实施 AnchorScan 项目级规则；不直接修改全局 npm 安装目录或 `node_modules` 中的 Trellis。
- `docs/plans/` 是跨会话、多 ticket 功能工作的唯一需求/验收权威；Trellis task 只保存执行状态与对权威 ticket 的引用，不复制需求。
- 本计划实施前后，当前 `.trellis/tasks/07-29-fix-v2-0-2-release-build/` 仍由其现有任务负责；不得为了本计划切换、归档或重写该任务。
- `trellis-check` 保留可写、自修复职责；最终 Standards/Spec 评审必须是独立只读步骤。
- TDD 的最低充分 seam 继续由 `docs/testing-strategy.md` 决定，不要求重复性跨层覆盖。
- 可复用的机制在 AnchorScan 连续完成至少 2 个真实行为变更后，再整理为 Trellis 上游提案；上游提案不阻塞项目级落地。

## 行为契约

### 1. 合并与发布

- `main` 必须要求 PR 与 `quality-gate`；直接 push、force push 和删除被拒绝。单协作者仓库经明确批准可将必需审批数设为 0，独立评审仍由仓库内 ticket 约束。
- `quality-gate` 必须覆盖既有 `make pr-check` 与 `make security-check`。
- release tag 只能指向已通过 required checks 的 `main` 提交；失败的 release 不能作为首次发现集成问题的渠道。

### 2. 单一权威计划

- 已存在 `docs/plans/<feature>/` 的工作必须由一个 `source_of_truth` 引用到 spec 和 ticket。
- ticket 未标记 `ready-for-agent` 时，关联的 Trellis task 不得启动实现。
- 完成后，权威 ticket、Trellis task 的交付引用和归档状态必须可机械核对。
- 旧的计划/Trellis 分叉必须在实施前对齐；达梦默认口令检测是首个迁移样本。

### 3. 任务状态机

- `task.py start` 必须先通过 `validate --ready`；失败不得把状态从 `planning` 改为 `in_progress`。
- `task.py archive` 必须先通过 `validate --complete`；失败不得标记 `completed`、移动目录或生成 archive commit。
- sub-agent 模式下，seed-only 或 0-entry 的 `implement.jsonl`/`check.jsonl` 必须失败；仅 inline 或显式豁免任务可跳过。
- 行为变更任务必须记录 branch、base branch、review fixed point、权威 ticket、TDD 证据、验证结果和双轴评审结果。
- 紧急绕过必须使用显式 `--force --reason`，并保存在 task evidence 中；不得静默降级。

### 4. 执行与评审

- 高风险行为变更的顺序为：规划批准 → Red → 最小实现/Green → 可写 self-check → Standards review → Spec/AC review → 全量验证 → PR。
- 独立评审使用 `code-review` 或等效的两个只读审查；self-check 不能替代它。
- 完成前按变更风险运行 `make test`、`go vet ./...`、`make pr-check`、`make e2e` 中的必要子集，并在证据中记录实际命令与结果。

### 5. 规范和回归

- backend/frontend spec index 必须有可执行的 Pre-Development Checklist 和 Quality Check，且质量规范不得保留模板占位符。
- 新增 `make harness-check`，在 PR CI 中检查 AI 工作流自身的关键契约。

## 非目标

- 不改变 AnchorScan 产品扫描、报告或发布包的业务行为。
- 不引入外部 issue tracker、通用工作流 DSL、数据库或新的 task runner。
- 不要求将所有历史 Trellis archive 追溯补齐到同一证据标准；只修复已识别的分叉样本，并让新任务开始受强制门禁。
- 不在本计划中直接 fork 或发布 Trellis 上游。

## 实施顺序

1. 保护 `main` 并暂停自动 bookkeeping commit。
2. 对齐现有达梦计划/任务状态，定义权威引用与证据模型。
3. 为 task start/archive 实施 ready/complete gate。
4. 同步 Trellis workflow、Codex/Pi 代理与 TDD/独立评审路径。
5. 完成项目 spec bootstrap。
6. 实现并接入 harness 自检。
7. 用至少两个真实行为变更验证后，输出 Trellis 上游改进提案。

## 总体验收

- GitHub 拒绝绕过 PR 的 `main` 更新，required checks 与 approval 实际生效。
- 新的行为任务若缺 branch/fixed point/权威 ticket/上下文/批准，`task.py start` 返回非零且不改变状态。
- 新任务若缺 TDD、验证、双轴评审或交付引用，`task.py archive` 返回非零且不移动任务。
- 已存在 `docs/plans/` 的任务不再复制需求；计划和 Trellis 状态无冲突。
- backend/frontend quality spec 具有项目真实规则，`make harness-check` 能阻止本审查中发现的关键回归。
- 最近两个完成的行为 ticket 均可追溯到 source ticket、fixed point、red/green、评审、验证、PR 与 merge。

## 执行规则

- 每次只实施一个所有阻塞项已完成的 frontier ticket。
- 每个 ticket 开始前记录当前 `main` 的 fixed point；不得把 SHA 写死到本计划文档，运行时写入 ticket 验收记录或 task evidence。
- 所有行为变更按 `docs/agents/issue-tracker.md` 执行 `implement`、`tdd`、`code-review`。
- GitHub branch protection/ruleset 属于外部持久变更：实施 ticket 必须在执行前展示目标设置并取得用户确认。
