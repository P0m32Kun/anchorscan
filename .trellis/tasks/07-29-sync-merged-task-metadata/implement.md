# 执行计划

1. 以 `origin/main` 为基线，对比 `codex/add-spark-detection-rules` 的 metadata 内容；列出仅与 Spark 归档、evidence、journal/index 及 backlog 对应的允许路径。
2. 仅应用 Spark task 的 archive 目录、Spark journal/index 条目和修正后的 `docs/plans/next-session-backlog.md`。
3. 验证 Spark task 已不在 `task.py list` 的活动 inventory 中；核验其归档 evidence 的 PR #13、`eac3b33` 和 merge 时间；同时断言默认 Nuclei 模板规则 task 仍处于活动目录，`status` 为 `in_progress`，且交付字段未变。
4. 运行 Markdown 链接检查及 task-gate 检查，并独立检查 diff 只包含允许的 metadata/backlog 路径。
5. 创建维护 PR，等待 CI；合并后复核任务 inventory 与 Spark PR #13 的交付事实一致。

## Rollback Point

应用归档重命名前先检查 `git diff --name-status`。一旦 diff 包含默认 Nuclei 模板规则 task 的变化或任何产品代码、配置、测试文件，停止并丢弃该候选改动。
