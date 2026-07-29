# Docs-only fixture

该 fixture 不在 `.trellis/tasks/` 下，不能被 task lifecycle 当成可执行任务；它只验证
`task.json.meta.source_of_truth` 和 `quality-evidence.json` 的稳定数据形状。脚本化 gate
将在 Ticket 06 添加。
