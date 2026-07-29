# Implementation Plan

1. 执行版本契约子任务：确认现有 `dev` 回退和 release linker 注入的行为、补足最低充分回归验证；若契约已满足，记录无产品代码变更的证据。
2. 执行控制台输出子任务：以 ScanEvent 写入边界为 seam，原样保留日志/制品诊断，持久化简洁摘要；删除未命中 Dameng 的探测进度噪声；以测试覆盖 ANSI、多行工具输出和关键错误保留。
3. 执行状态处置子任务：以 supplied `report.json` 作为历史事实，修正 SSH 模板运行时错误并在受控 SSH 环境验证；不对生产目标重跑。
4. 每个子任务先走最低充分 TDD seam，执行独立审查和自身质量检查。
5. 父任务整合时复核版本、事件摘要和 DetectionCheck 状态没有交叉破坏。

## Completion Gate

三个子任务均完成其验收、审查和验证；父任务只汇总结论，不修改历史 Run 状态。
