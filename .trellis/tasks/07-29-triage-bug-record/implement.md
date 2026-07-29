# Bug 记录处置执行计划

## 规划完成条件

- [x] 将八项原始记录映射为复用、已解决、功能或研究任务。
- [x] 确认默认规则旧 `template:` 的迁移行为：加载失败而非静默忽略。
- [x] 确认 SSL/TLS 与 Spark 保持独立。
- [ ] 为六个子任务补齐 PRD 收敛、设计、执行计划和真实上下文清单。
- [ ] 对 Spark、TLS、未知服务分别记录本地与外部版本证据；未知项不进入实现队列。
- [ ] 向用户提交最终规划摘要并取得一次新的实施批准。

## 后续执行顺序

1. 完成 `remove-default-nuclei-template-rules` 设计；记录配置严格解码、扫描分支删除、单工具回归与发布归档检查。
2. 完成 `enhance-report-service-filters` 和 `fix-console-shanghai-time` 的跨层设计；指定后端/前端测试 seam。
3. 完成 TLS 与未知服务研究方案；研究证据写入各自任务目录或 `docs/research/`，不修改代码。
4. 使用研究结果收敛 Spark 规则的外部 tags、指纹条件、安全排除和实验室验证；它受步骤 1 阻塞。
5. 将真实的 spec/research 条目加入每个子任务的 `implement.jsonl` 与 `check.jsonl`。
6. 进行 PRD convergence pass，检查无遗留 open question、无重复交付与显式依赖。
7. 向用户展示最终规划；只有后续明确批准后，启动一个满足依赖的子任务。

## 全局验证

- 父任务验证只检查任务映射、依赖与文档一致性；不运行扫描或修改历史 Run。
- 实施子任务按照项目流程执行 TDD Red -> Green -> 自检 -> 双轴只读审查 -> 全量验证 -> PR。
- 子任务完成后，父任务复核八项记录均有唯一处置和链接证据。
