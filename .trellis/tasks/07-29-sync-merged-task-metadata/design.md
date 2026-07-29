# 元数据同步设计

## Sources

- `origin/main` 是目标基线：Spark 与默认 Nuclei 模板规则两个 task 目录目前仍处于活动目录。
- 本地分支 `codex/add-spark-detection-rules` 保存 Spark 任务的归档、PR #13 merge evidence、journal/index 和 backlog 候选内容。
- `codex/repair-ssh-nuclei-runtime-failure` 的归档候选与 journal 条目不属于本次同步范围，不能作为 PR #12 已交付的证据。

## Boundary and Data Flow

从 `origin/main` 建立专用维护分支，只选择 Spark 交付的下列文件：

1. 将 `07-29-add-spark-detection-rules` 移至 `tasks/archive/2026-07/`，包含其 `task.json`、`quality-evidence.json` 和原有规划/检查文件。
2. 在 `quality-evidence.json` 中保留实现 commit `eac3b33`、PR #13 URL 和已确认 merge 时间。
3. 合并 Spark 对应的 developer journal/index 条目，并添加经更正的跨会话 backlog。

不移动、不编辑或回填 `07-29-remove-default-nuclei-template-rules`。任务 inventory 因而只减少 Spark 这一项活动任务。

## Compatibility and Rollback

本 PR 仅改变项目跟踪元数据，不影响已发布扫描行为。若 Spark 归档或 evidence 有误，回退该维护 PR 或以新的 metadata PR 恢复 Spark 任务目录；默认 Nuclei 模板规则任务在整个过程中保持不变。
