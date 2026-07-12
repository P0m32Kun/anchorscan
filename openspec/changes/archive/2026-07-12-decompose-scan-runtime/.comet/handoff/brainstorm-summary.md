# Brainstorm Summary

- 用户于 2026-07-12 确认采用同包机械拆分方案。
- 保留 `RunScan`、`ScanOptions`、Manager 与 Runner 契约，不改变并发、取消、错误、事件、artifact 或报告行为。
- 实现边界为运行生命周期、多目标调度、单目标固定流水线；不新增子包、接口或动态 stage。
- 实施顺序为特征测试、单目标流水线提取、调度提取、生命周期收敛、全量验证。

