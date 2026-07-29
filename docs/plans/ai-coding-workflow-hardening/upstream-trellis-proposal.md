# Trellis 上游改进提案草案：可验证的任务生命周期

**状态：** 项目级提案草案；不修改 Trellis 上游、全局安装或 npm 包。

## 问题与项目证据

AnchorScan 将 Trellis 用于会话任务，而将跨会话需求保存在 `docs/plans/`。项目层补充了严格的 ready/complete gate、`quality-evidence.json`、TDD 与独立评审顺序，以及 `make harness-check`。实现位置与契约见：

- ready/complete 校验：`.trellis/scripts/common/task_context.py` 的 `validate_task_gate`；
- 生命周期调用点：`.trellis/scripts/task.py` 与 `.trellis/scripts/common/task_store.py`；
- self-check / 独立评审边界：`.trellis/workflow.md`；
- 可机器验证的项目防漂移检查：`scripts/check_ai_workflow.mjs`；
- 项目级数据模型和边界：`docs/plans/ai-coding-workflow-hardening/technical-design.md`。

两项已合并的真实行为变更提供了使用证据：

| 任务 | 交付 | 已观察到的风险降低 |
| --- | --- | --- |
| Spark 服务按 Nuclei tag 路由 | PR #13，`eac3b33` | `quality-evidence.json` 记录 Red → Green、独立 Standards/Spec 评审和交付引用；自检曾补齐 PrepareScan 边界、URL fallback 与未知端口检测事实，避免仅凭局部规则改动宣称完成。 |
| 报告服务 facet 与未识别服务排除 | PR #17，`3682f73` | Red 测试锁定协议为空 Finding 的回退关联；只读评审与浏览器 smoke 覆盖跨后端/导出/前端分页语义，避免只修页面而破坏报告读取契约。 |

证据文件分别位于 `.trellis/tasks/archive/2026-07/07-29-add-spark-detection-rules/quality-evidence.json` 和 `.trellis/tasks/archive/2026-07/07-29-enhance-report-service-filters/quality-evidence.json`。它们说明通用需要是“可验证的完成声明”，不是 AnchorScan 的扫描规则或测试命令。

## 建议的上游能力

### 1. 可配置的 ready / complete gate

Trellis 提供可选的项目配置，而不是写死 AnchorScan 规则：

```yaml
task_gates:
  ready:
    enabled: true
    require:
      - branch
      - fixed_point
      - source
      - approval
      - planning_artifacts
  complete:
    enabled: true
    require:
      - tdd_evidence
      - verification
      - independent_reviews
      - delivery
```

命令可统一为 `trellis task validate --ready` 与 `--complete`，并让 `start` / `archive` 在 gate 失败时拒绝状态迁移。规则由项目声明；框架只提供校验器、结构化诊断和生命周期接入点。

### 2. 可扩展 source 与质量证据 schema

建议为 `task.json.meta` 和 task 目录提供版本化、可扩展的公共字段：

```json
{
  "meta": {
    "fixed_point": "<git sha>",
    "source": {"kind": "ticket", "refs": ["docs/plans/.../ticket.md"]}
  },
  "evidence": {
    "schema": 1,
    "approval": {"result": "passed"},
    "tdd": {"red": "failed", "green": "passed"},
    "reviews": {"standards": "passed", "spec": "passed"},
    "delivery": {"commit": "<sha>", "pr": "<url>"}
  }
}
```

`source.kind` 应允许项目 ticket、Trellis PRD 和外部 issue 等类型，由项目 policy 决定哪一种可用于某类风险。`delivery.merged_at` 应是可选观察值：PR URL 和交付 commit 足以完成 task gate；强制 merge 后回填会制造只为元数据而生的递归 PR。

### 3. 明确 self-check 与独立 review 的权限边界

工作流 API/提示应将两类动作显式建模：

- `self-check`：可写、可修复，不能声称独立批准；
- `review`：只读，分别针对 standards 与 ticket/spec/AC，输出固定基线、范围和结论。

框架可提供 review manifest，例如 `{fixed_point, source, axis, readonly: true}`，而不绑定某个代码审查工具或具体测试命令。

### 4. Hook doctor 的 configured / observed 分层

`trellis doctor --format json` 应分开报告：

- `configured`：配置与 hooks 是否存在；
- `observed.manual`：用户/Agent 本轮实际执行的命令与结果；
- `observed.automatic`：hook 自动执行的事件、时间、退出码。

这样“已配置”不会被误说成“已运行”；同时不要求框架访问 GitHub、CI 或项目私有日志。

### 5. mode-aware 的上下文完整性校验

对于需要 curator context 的分发模式，ready gate 应拒绝 seed-only 或零条有效 JSONL；inline 模式则明确跳过该项并要求直接读取 task artifacts。该判断应由 dispatch mode 与角色能力驱动，不能把所有运行环境一概视为缺上下文。

## 项目配置与框架默认的边界

| 归属 | 内容 |
| --- | --- |
| Trellis 框架 | gate 生命周期接入、schema 扩展点、结构化诊断、doctor 状态模型、mode-aware 校验。 |
| 项目配置 | 哪类 task 必须 TDD/评审、source 类型白名单、必需产物、质量命令、审批策略、分支保护。 |
| 项目业务 | AnchorScan 的 `make pr-check`、扫描授权边界、目录名称、领域契约和具体测试。 |

## 兼容与迁移

1. 默认保持现有宽松行为；仅项目显式启用 gate 才阻断 `start` / `archive`。
2. 旧 task 没有 schema 时显示 `not_evaluated`，不追溯性判失败；新建 task 采用 schema v1。
3. schema 校验忽略未知扩展字段，允许项目在 `meta.extensions` 下携带私有信息。
4. `source.kind=trellis-prd` 保持兼容；项目可逐步迁移到外部 ticket，而不强制复制已有需求。
5. `delivery.merged_at` 仅作为可选 observed 字段，避免合并后再开元数据 PR 的递归交付。

## 最小测试建议

- gate：ready/complete 的成功与每项失败诊断；失败时状态和目录不改变。
- schema：v1、未知扩展、旧 task 无 schema、多个 source kind。
- review：self-check 不能满足 independent-review requirement；两个 review axis 可独立报告。
- doctor：configured 与 manual/automatic observed 状态不互相推断。
- context：auto-dispatch 的空/seed-only JSONL 被拒绝，inline 任务不误拒。
- migration：禁用 gate 的旧项目行为不变；启用后只影响新配置范围内的 task。

## 建议的上游 issue 结构

1. 问题：Trellis 可以记录 task，但缺少可选、可审计的完成门禁与观察状态模型。
2. 使用证据：引用上述两项真实行为变更及其证据文件，不附带业务代码。
3. 最小方案：配置化 gate + versioned schema + doctor 状态分层。
4. 兼容性：opt-in、旧 task `not_evaluated`、未知字段兼容。
5. 非目标：不内置项目测试命令、不管理 GitHub ruleset、不替代项目审批策略。
