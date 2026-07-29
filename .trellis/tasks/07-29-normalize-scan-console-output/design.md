# Technical Design

## Seam

在 `storeProgress.Emit` 分离两个输出：`Logf` 接收原始消息，持久化的 `ScanEvent.Message` 接收确定性的摘要。这样不改变工具 artifact 或 CLI 诊断，同时限制 Web Console 负载。

`scanTarget` 不再为每个 Dameng 候选发事件，仅在实际匹配时发出结果。Nuclei 等工具阶段继续按现有顺序写 artifact、记录 DetectionCheck、再发错误事件。

## Summary Contract

- 去掉 ANSI 和空白噪声。
- 单行普通进度保持语义。
- 嵌入多行工具输出的失败提取最终 FTL/错误原因；保留上层工具与目标前缀。
- 绝不将原始 stdout/stderr 写入 ScanEvent；原始内容仍在日志/artifact。

## Risks and Rollback

摘要器只能改变事件展示内容，不能影响 status、artifact、DetectionCheck 或 runner 参数。若误删可行动错误，回滚摘要器而不改变历史事实。
