# 10 — 完成集成发布验收

**What to build:** 汇合质量门禁、Run 韧性和检测执行覆盖三条链，补齐新状态的浏览器回归，执行完整发布级验证，并把操作说明交付给非程序员用户。

**Blocked by:** `02-cover-core-web-workflows.md`、`04-reconcile-interrupted-runs.md`、`07-deliver-detection-coverage-reports.md`、`08-add-opt-in-tool-timeouts.md`、`09-expand-real-tool-lab.md`。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [ ] Playwright 补齐 completed_with_errors、interrupted、DetectionCheck 摘要/报告和 timeout 配置工作流。
- [ ] 1440px 主流程和 1280px 关键页面通过桌面可用性检查，键盘焦点与可访问名称可用。
- [ ] Go、JavaScript、构建/打包、Playwright 和真实工具实验室全部通过。
- [ ] 发布记录能关联当前提交、工具版本、实验室日志与测试结果。
- [ ] 用户文档用非程序员语言解释六种 Run 状态、检测执行覆盖、重跑确认和 timeout 默认关闭。
- [ ] `CONTEXT.md`、ADR、spec、technical design 与最终公共行为一致；删除过时说明。
- [ ] 以计划开始时的 fixed point 执行 Standards/Spec 双轴 code-review，修复 blocker/major 后重新运行完整验证。
- [ ] 不在最终验收中顺手增加新检测引擎、队列、恢复执行或多浏览器范围。
