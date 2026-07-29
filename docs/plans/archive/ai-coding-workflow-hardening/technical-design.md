# AI 编码工作流加固：技术设计

## 约束与边界

- 本设计只改项目内文件、GitHub 仓库设置和可复现的 CI；不改 Trellis 全局安装目录。
- `.trellis/` 是生成后可定制的项目层。对 `.trellis/scripts/` 的本地修改必须由 `make harness-check` 覆盖，并在 `trellis update --dry-run` 时人工审查冲突。
- 当前活动 release 修复任务与本计划并行，禁止修改其 runtime session 文件或任务状态。
- 规则必须以可机器读取的数据为主，Markdown 说明为辅；避免仅增加更长 prompt。

## 1. 治理边界

### GitHub

GitHub ruleset 是最终不可绕过门禁，负责：PR-only、required status check、审批、禁止 force push/delete。它不承担 TDD/验收细节，后者由仓库内 gate 产生可检查结果。

### `docs/plans/`

跨会话、多 ticket 功能工作的需求、验收和状态唯一保存在 `docs/plans/<feature>/`。每一个实际执行 task 必须引用其 spec/ticket；不再将同一需求复制进 Trellis PRD。

### Trellis task

Trellis task 负责会话、研究、执行上下文和本地证据。新增字段放在 `task.json.meta`，避免破坏 Trellis 的标准字段：

```json
{
  "meta": {
    "source_of_truth": {
      "type": "docs-ticket",
      "spec": "docs/plans/<feature>/spec.md",
      "ticket": "docs/plans/<feature>/tickets/<nn>-<slug>.md"
    },
    "risk": "behavioral",
    "fixed_point": "<runtime-recorded-sha>"
  }
}
```

轻量、一次性任务可使用 `type: "trellis-prd"`，但不得与 `docs/plans/` 并存。

## 2. 证据模型

每个行为变更 task 使用 `<task>/quality-evidence.json`，由执行步骤渐进更新。
完整且唯一的 JSON 契约见 [`docs/agents/task-evidence.md`](../../../agents/task-evidence.md)；
下例只是该 v1 契约的摘要，不得另行扩展或放宽字段：

```json
{
  "schema": 1,
  "approval": {"recorded_at": "...", "summary": "...", "result": "passed"},
  "tdd": {
    "required": true,
    "red": {"command": "...", "result": "failed", "summary": "..."},
    "green": {"command": "...", "result": "passed", "summary": "..."}
  },
  "verification": [{"command": "make pr-check", "result": "passed", "at": "..."}],
  "reviews": {
    "standards": {"result": "passed", "artifact": "..."},
    "spec": {"result": "passed", "artifact": "..."}
  },
  "delivery": {"branch": "...", "commit": "...", "pr": "...", "merged_at": "..."}
}
```

证据文件是 gate 的输入，不是替代现有 ticket 验收记录。评审产物可为 task 下的 Markdown 文件，quality JSON 只记录路径和结论。

## 3. Ready / Complete Gate

在 `task.py validate` 增加两种严格模式：

| 模式 | 最低检查 | 调用方 |
| --- | --- | --- |
| `--ready` | 不在 `main`、branch/base/fixed point、source ticket、批准、必要规划产物、真实 JSONL | `task.py start` 前 |
| `--complete` | `--ready` 相关可追溯字段、验收完成、red/green、验证、两个 review、commit/PR | `task.py archive` 前 |

普通 `validate` 保留为诊断兼容入口，但输出必须清楚标识“合法 JSONL”不等于“planning-ready”。

实现位置优先为：

- `.trellis/scripts/common/task_context.py`：解析 JSONL 与 evidence；
- `.trellis/scripts/task.py`：暴露 strict 参数并在 `cmd_start` 前调用；
- `.trellis/scripts/common/task_store.py`：在归档移动前调用 complete gate；
- `.trellis/scripts/common/task_utils.py`：路径和 metadata 校验辅助。

不能只使用 `after_start` / `after_archive` hooks：Trellis lifecycle hooks 的失败语义是 warning，无法阻止状态变迁。

## 4. Workflow 和代理职责

不新增自定义 status，先保持 `planning` / `in_progress`，以免扩大 `/trellis:continue` 路由改动；在详细 Phase 步骤、workflow-state 和 agent prompt 中增加必经步骤：

1. Plan：权威 ticket、批准、`validate --ready`。
2. Execute：TDD Red → implement/Green → self-check。
3. Review：只读 Standards review 和 Spec/AC review。
4. Deliver：全量检查、PR、merge 后 `validate --complete`、archive/journal。

`trellis-check` 明确为可写 self-check；新建项目级 review 调度说明以调用现有 `code-review` skill 或两个独立只读代理，避免将“修复”和“独立审计”混为一体。

需同步 `.trellis/workflow.md`、`.agents/skills/trellis-continue/SKILL.md`、`.codex/agents/trellis-*.toml`、`.pi/agents/trellis-*.md` 和 `.trellis/agents/*.md`。仅在角色语义确有变化时改平台 agent；channel runtime 代理独立维护。

## 5. 项目 spec

补全 backend/frontend index 的两个固定章节：

- Pre-Development Checklist：领域文档、ADR、测试 seam、适用代码规范；
- Quality Check：按变化类型选择 `make test`、`go vet ./...`、`make pr-check`、`make e2e`，并要求 TDD 与双轴 review。

quality guideline 引用真实项目文档和代码例子，不复制通用模板。`00-bootstrap-guidelines` 只在规范实际填充并由本计划 gate 覆盖后归档。

## 6. Harness 自检

新增无依赖的 `scripts/check_ai_workflow.mjs` 与 `make harness-check`。它读取文本与 JSON，检查：

- workflow 包含 TDD、独立 review、PR 和 gate；
- task scripts 在 start/archive 前调用 strict gate；
- sub-agent JSONL 0-entry 不能被 ready gate 接受；
- backend/frontend quality spec 无占位符并含两个 checklist；
- required platform agent/Workflow 术语未漂移；
- 已完成 task 具有最低 delivery/evidence 字段；
- 当前活跃 docs plan 与关联 Trellis task 没有已知状态冲突。

该脚本不调用 GitHub API；GitHub ruleset 的实际状态用独立、可认证的 `gh api` 检查或 CI 文档说明验证，避免 fork PR 权限问题。

## 7. 回滚和维护

- GitHub ruleset 前先导出目标规则的 JSON/截图；若错误配置导致阻塞，只有仓库管理员可临时回退。
- `session_auto_commit: false` 是可逆配置；等 task 分支与 complete gate 稳定后再评估恢复。
- `.trellis/scripts/` 本地补丁在每次 `trellis update --dry-run` 时审查；升级不自动覆盖用户修改。
- 上游化只输出 issue/PR 草案，不改变本项目已验证的行为。
