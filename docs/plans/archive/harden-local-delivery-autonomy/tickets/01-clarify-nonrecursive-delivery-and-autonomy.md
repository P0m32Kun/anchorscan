# 01 — 明确非递归交付与持续自治

**What to build:** 同步任务证据文档、Trellis workflow、本地 skill 与 Pi prompt，消除
`merged_at`/归档导致后续元数据 PR 的语义，并写明持续自治的常规 Git 边界。

**Blocked by:** None — can start immediately.

**Status:** done

**Execution skills:** `implement`、`code-review`。

## 行为契约

- `merged_at` 为可空观测字段，complete/archive 不依赖它。
- 归档、ticket/evidence 更新属于 delivery PR；PR 合并后不得为纯元数据补录创建 PR。
- 持续自治下，常规 Git 交付无须重复确认；外部/全局/安全和未知并行工作仍升级。

## 实施

1. 更新 task evidence 与 `.trellis/workflow.md`。
2. 同步 `.agents/skills/trellis-continue`、`.agents/skills/trellis-finish-work` 及对应 Pi prompt。
3. 保留对无法确认归属的并行 dirty 文件的升级路径。
4. 运行 Markdown/link 和现有 harness 检查，完成固定点双轴审查。

## 验收

- [x] 全部入口对 delivery/自治边界表述一致。
- [x] 不再要求 `merged_at` 或合并后元数据 PR 才能完成/归档。
- [x] 常规 Git 操作不再要求 one-shot confirmation。
- [x] 外部/全局 npm 与安全升级边界未被放宽。

## 完成证据

- Fixed point：`b2517b876ce95a1a5ec972c5ceb1595a9c829cac`（`origin/main`）。
- `git diff --check`、`node scripts/check_markdown_links.mjs` 与 `make harness-check` 通过。
- Standards/Spec 两个独立只读审查完成；修复了被忽略的 `trellis-finish-work` skill 未随 PR
  交付，以及同一 PR 内 archive/journal 提交的歧义。harness 回归保护由解除阻塞后的 Ticket 02 实施。

## 非目标

- 不修改 `task_context.py` 的既有 complete gate 行为。
- 不修改上游 Trellis 或全局 skill。
