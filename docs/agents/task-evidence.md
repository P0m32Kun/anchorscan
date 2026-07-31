# Task 权威引用与质量证据

跨会话、多 ticket 工作的需求、验收和状态唯一保存在 `docs/plans/`。不要再建立平行的运行时 task store 或复制同一需求。

## Durable ticket

行为任务的 ticket 至少记录：

- 对应 spec 和验收标准；
- 风险与实现边界；
- 开始实现时观察到的 fixed point；
- 实际执行的测试、静态检查和评审结果；
- 分支、提交和 PR 等交付引用。

无法观察到的证据必须明确标记为 `unobserved` 或 `unavailable`，不得用空字段暗示通过。文档任务可以将不适用的测试标为 `not_applicable`。

## Completion

完成声明必须和实际交付变更一起记录，不为补写状态、归档或日志创建纯元数据 PR。单会话的小改动可以只在会话和 PR 中报告证据，不要求创建持久 ticket。
