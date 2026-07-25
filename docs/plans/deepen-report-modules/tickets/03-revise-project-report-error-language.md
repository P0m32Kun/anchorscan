# 03 — 修订 Project Report 错误语言

**What to build:** 基于 `error-inventory.md` 重新设计 Project Report 的用户可见错误分类与中文文案，消除内部错误直接外泄，并同步测试和操作说明。

**Blocked by:** 01 — 深化 Project Report 交付组装 module。

**Status:** proposed

**Execution skills:** `grilling`、`domain-modeling`、`implement`、`tdd`、`code-review`、`update-spec`、`ponytail`。

- [ ] 与用户逐项确认错误分类、稳定错误码、中文措辞和可公开的诊断信息。
- [ ] 明确 400 / 404 / 500 / 503 的用户行动建议，避免仅复述内部失败。
- [ ] 决定数据库错误、文件路径和 sidecar stderr 的日志与展示策略。
- [ ] 更新 `error-inventory.md` 为新旧映射及兼容说明。
- [ ] 通过 module interface 修改错误语义，不把字符串匹配重新放回 adapter。
- [ ] 更新 Web 回归测试与相关操作文档。
- [ ] 完成聚焦验证、全量门禁与 Standards/Spec 双轴 code review。
