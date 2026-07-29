# 04 — 对齐 TDD、独立评审、workflow 与代理

**What to build:** 让项目的 Trellis workflow、Codex/Pi agent 和完成路径执行根 `AGENTS.md` 已要求的 TDD 与双轴 review。

**Blocked by:** 03 — 强制 task ready / complete gate。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`。

## 行为契约

- 高风险任务的 per-turn workflow-state 明确显示 Red → Green → self-check → Standards review → Spec/AC review → full verification → PR。
- `trellis-check` 被描述为可写 self-check，不能代表独立 review。
- code-review 路径是只读且基于 fixed point 与 ticket/spec。
- Codex、Pi 和 channel runtime agent 对相同角色有一致语义。

## 实施

1. 更新 `.trellis/workflow.md` 的 Phase Index、详细步骤、workflow-state 和交付路径。
2. 更新 `trellis-continue` 的路线描述，保持 status 不新增、不破坏 resume。
3. 更新 Codex/Pi/channel runtime 的 implement/check prompt，增加 TDD 输入、验证命令和职责边界。
4. 写项目级 review dispatch 说明，复用 `code-review` skill；不新增可写 reviewer。
5. 用静态 contract test 覆盖所有入口的术语和关键顺序。

## 验收

- [x] 所有 `in_progress` 路径都包含 TDD、双轴 review、PR。
- [x] agent prompt 不再将 lint/typecheck 描述为唯一完成条件。
- [x] self-check 与独立 review 的输出位置和权限不同。
- [x] `get_context.py --mode phase --platform codex` 输出与 workflow 一致。

## 完成证据

- 2026-07-29：`scripts/test_workflow_review_contract.py` 覆盖 sub-agent、inline、Codex、Pi、channel runtime 和共享 continue skill 的 TDD/独立评审契约；4 项通过。
- 2026-07-29：`make test`、`make pr-check` 与 `make security-check` 通过。
- 2026-07-29：以 `412eb41` 为固定点完成独立 Standards/Spec 审查；所有 P1 与测试覆盖 P2 均已修复并复核。

## 非目标

- 不切换整套 Trellis workflow template，不增加新 status。
