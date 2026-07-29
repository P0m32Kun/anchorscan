# 同步已合并任务的 Trellis 元数据

## Goal

将已合并的 Spark 产品 PR 的本地后置 Trellis 元数据同步到 `main`，使该任务的状态、归档记录、merge evidence 与 GitHub 交付事实一致，同时不错误关闭仍未完成闭环的默认 Nuclei 模板规则任务。

## Confirmed Facts

- PR #13（Spark 服务检测规则）已在 `origin/main` 的 merge commit `c0adbb3` 合并；其实现 commit 为 `eac3b33`，本地交付分支已有完整归档、验证和审查证据。
- `origin/main` 仍将 `07-29-add-spark-detection-rules` 显示为 `in_progress`，且 `merged_at` 未回填。
- `07-29-remove-default-nuclei-template-rules` 仍为 `in_progress`；其 task metadata 的 `commit`、`pr_url` 和 `merged_at` 均未回填，且尚未完成 complete/归档闭环。
- 虽然 Git 历史中存在标题为 PR #12 的 merge commit，但该任务当前不得作为已交付产品变更或已合并 PR 计入本次同步。

## Requirements

- 仅归档已完成闭环且已由 PR #13 合并的 `07-29-add-spark-detection-rules`。
- 保留 Spark 归档任务中的验证、审查、PR URL、实现 commit `eac3b33` 和 merge 时间 `2026-07-29T13:16:07Z` 证据。
- 将 `docs/plans/next-session-backlog.md` 纳入 `main`，并将其中已完成产品变更与元数据同步描述限制为 Spark PR #13；不得称 PR #12 或默认 Nuclei 模板规则任务已完成/合并。
- 同步仅属于 Spark 交付的 developer journal/index，避免重复写入或覆盖并行会话记录。
- 保持 `07-29-remove-default-nuclei-template-rules` 及其现有 metadata 原样为 `in_progress`；不得回填其 commit、PR 或 merge 时间，不得 complete 或归档。
- 只包含 `.trellis/` 元数据及 `docs/plans/next-session-backlog.md`；不得修改扫描产品代码、配置、测试、历史 Run 或 artifact。

## Out of Scope

- 修复、关闭、归档或更改 `07-29-remove-default-nuclei-template-rules` 的任何交付状态。
- 修改 GitHub PR #12 的状态、内容或历史。
- 重建已合并 PR 的产品代码提交。

## Acceptance Criteria

- [ ] `main` 不再将 Spark 任务显示为活动任务；其位于对应月份的 archive 目录，且 `task.json` 为 `completed`。
- [ ] Spark 归档任务的 `quality-evidence.json` 保留正确的 PR #13、实现 commit 和 merge evidence。
- [ ] 默认 Nuclei 模板规则任务继续位于活动任务目录、状态为 `in_progress`，且其交付字段没有被本 PR 回填或修改。
- [ ] 跨会话 backlog 文档存在，链接检查通过，且不将 PR #12 标记为完成或已合并。
- [ ] PR diff 只包含允许的 `.trellis/` 元数据和 backlog 文档，不包含产品代码、扫描配置或测试文件。
- [ ] PR CI 通过；合并后任务 inventory 与确认的 Spark PR #13 交付事实一致。
