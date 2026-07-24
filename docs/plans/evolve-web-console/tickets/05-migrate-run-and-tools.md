# 05 — 迁移 Run 状态与工具交互

**What to build:** 统一 Run 轮询、阶段反馈、日志视图、取消动作以及工具/导入页的非阻塞反馈。

**Blocked by:** 04 — 迁移报告交互。

**Status:** done

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

- [x] Run 详情显示生命周期、当前阶段、可靠计数、最新事件与下一步动作。
- [x] 未知进度不伪造百分比；轮询不抢焦点、不重置滚动或展开状态。
- [x] 取消、失败、部分错误与完成使用准确领域文案和非阻塞反馈。
- [x] 工具与导入页只迁移真实交互状态，普通服务端表单保持原生。
- [x] 删除被替代的轮询和 dialog 脚本并通过离开/返回、取消和失败回归。
