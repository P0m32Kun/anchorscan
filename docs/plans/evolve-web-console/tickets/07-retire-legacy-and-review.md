# 07 — 收口旧实现与长期门禁

**What to build:** 完成全控制台状态所有权审计，删除迁移残留，并把缺陷修复与前端质量门禁写入维护流程。

**Blocked by:** 06 — 迁移其余页面交互。

**Status:** ready-for-agent

**Execution skills:** `code-review`、`frontend-visual-design`、`browser:control-in-app-browser`、`update-spec`、`ponytail`。

- [x] 删除无调用的原生脚本、样式选择器、迁移 adapter 和过期测试。
- [x] 确认每个交互只有一个状态所有者，Go/Vue seam 与错误模式有文档。
- [ ] `make pr-check`、release 构建、离线二进制和浏览器矩阵全部通过。
- [ ] 完成浅色/深色、键盘、焦点、状态颜色和长任务最终审计。
- [x] 将长期缺陷分级、回归检查与新能力 spec 规则写入项目维护文档。
- [ ] 以计划 fixed point 运行 Standards/Spec 双轴 code review 并关闭发现。
