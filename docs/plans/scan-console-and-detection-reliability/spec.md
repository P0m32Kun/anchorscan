# 扫描 Console 与检测执行可靠性

## 状态

已批准。仅 Ticket 01 是当前 ready frontier；Ticket 02 保持已批准但受其完成归档阻塞。

## 背景

扫描 Run 的 `ScanEvent` 是 Web Console 的实时用户界面，原始工具输出和 artifact 则承担诊断与取证职责。两者不能相互替代：前者需要稳定、可行动且低噪声的摘要，后者必须保留完整事实。

一个现场 Run 还确认了 `completed_with_errors` 的具体来源：SSH Nuclei 模板发生运行时错误并留下一个失败的 `DetectionCheck`。该终态是正确的历史事实；需要修复的是受控安全边界内的模板运行失败，而不是重写历史 Run 或降低失败语义。

## 已批准的产品边界

- 默认扫描的 Console 只展示可读的阶段、进度、匹配和可行动错误；日志与 artifact 保留原始工具输出。
- 未匹配的 Dameng 候选探测不产生 Console 事件，匹配结果仍可见。
- `completed_with_errors` 继续表示至少一个 Target 或 DetectionCheck 失败且已有可用结果；不得改写历史 Run、DetectionCheck 或 artifact。
- SSH `ssh-mini-brute` 保持最多 2 个用户名 x 2 个密码、首次命中停止的尝试上限；验证只在受控实验室进行，不得对现场 IP 重跑。
- 本计划不改变 Nuclei 模板选择/授权模型，不扩展为通用凭据爆破，不改动 Web Console 的原始 stdout/stderr 展开设计。

## 执行顺序

1. Ticket 01：规范 ScanEvent 输出，作为当前唯一 ready frontier。
2. Ticket 02：修复 SSH Nuclei 模板运行时失败；它在 Ticket 01 完成归档后开始，以便为 AI 工作流 Ticket 07 提供两个独立、完整的真实行为变更证据。两者没有产品代码依赖。

每个 ticket 开始时，在 Trellis task metadata 中记录实际分支与 fixed point；不得将运行时 SHA 写入本 spec。行为变化按 `docs/testing-strategy.md` 选择最低充分 seam，必须完成 Red -> Green -> self-check -> Standards/Spec 双轴独立评审 -> `make pr-check` -> PR，并在完成 gate 通过后归档。

## 测试策略

| 风险 | 最低充分 seam |
| --- | --- |
| ANSI、多行工具失败和进度摘要 | Go unit test（纯摘要函数） |
| 日志/artifact 与 ScanEvent 输出分离 | App/Store 聚焦集成测试 |
| 未匹配 Dameng 探测不发事件 | App fake probe/进度 seam |
| Nuclei 模板运行时错误导致检测失败与 Run 终态 | App fake Runner/Store 测试 |
| SSH 模板与 Nuclei 的实际协作 | `../lab` Docker 受控实验 |

## 非目标

- 修改历史持久化扫描事实，或将 `completed_with_errors` 伪装为 `completed`。
- 对生产/现场 IP 执行凭据探测。
- 使用 `-silent` 全局静默工具，或丢弃原始日志和 artifact。
- 将两个 ticket 合并为一个不可独立验收的变更。
