# 05 — 现代化任务、工具与导入流程

**What to build:** 统一扫描历史、任务详情、单工具调用、工具运行页和 Nmap 导入页面的状态、日志、表单和操作层级。

**Blocked by:** 04 — 现代化项目与扫描流程。

**Status:** done

**Execution skills:** `frontend-visual-design`、`tdd`（仅交互 seam）、`browser:control-in-app-browser`、`ponytail`。

- [ ] 扫描历史使用清晰列层级呈现状态、Run ID、项目和时间。
- [ ] 任务详情的运行状态、事件、日志、取消和报告入口保持现有行为。
- [ ] 运行中动画克制并支持 `prefers-reduced-motion`。
- [ ] 单工具表单、预设参数、执行结果和复制反馈使用共享原语。
- [ ] Nmap 导入页面明确文件、项目、错误和提交状态。
- [ ] 不改变轮询频率、工具参数、提交字段或导入语义。
- [ ] 相关 Go/JS 测试通过。
- [ ] 提供 1440px 扫描历史、任务详情、单工具和导入截图，经用户批准后完成。
