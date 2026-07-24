# 04 — 迁移报告交互

**What to build:** 将 Web 报告的搜索、筛选、活动条件、视图切换和命令 dialog 迁入 Vue，同时保持 URL 与导出语义。

**Blocked by:** 03 — 迁移验证工作台。

**Status:** ready-for-agent

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

- [ ] 搜索常驻，低频筛选收纳，活动条件可见并可逐项移除。
- [ ] 查询参数仍可分享、刷新和后退恢复。
- [ ] 检测执行事实默认折叠，发现与资产结果保持阅读优先级。
- [ ] popover、视图 tabs、分页、复制和命令 dialog 支持完整键盘路径。
- [ ] 删除 `app.js` 中被替代的报告交互与内联状态操作。
- [ ] Web 报告、HTML/DOCX 导出和批量命令行为回归通过。
