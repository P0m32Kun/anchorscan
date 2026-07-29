# 执行计划引用

Ticket 01 归档、来源 ticket 获批、`quality-evidence.json` 记录实施批准且 `validate --ready` 通过后，按 Ticket 02 的测试 seam 执行：Red -> 最小 Green -> self-check -> Standards/Spec 双轴独立评审 -> `make pr-check` -> PR。

实施前必须加载 `trellis-before-dev` 及 `implement.jsonl` 引用的上下文。不得在本迁移阶段实施产品代码或触碰历史 Run。
