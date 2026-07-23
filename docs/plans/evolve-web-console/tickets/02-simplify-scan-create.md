# 02 — 简化扫描创建流程

**What to build:** 用第一个页面级 Vue 模块重组扫描创建页，缩短高频提交路径并保留 Go 表单契约。

**Blocked by:** 01 — Vue 构建基础与全局主题。

**Status:** proposed

**Execution skills:** `tdd`、`frontend-visual-design`、`browser:control-in-app-browser`、`code-review`、`ponytail`。

- [ ] 首屏呈现上下文、目标、常用配置与唯一主操作。
- [ ] 单一 Zone 自动选中；多 Zone 仍要求明确选择。
- [ ] 低频内容进入最多一层 disclosure，并显示已修改摘要。
- [ ] 校验错误展开隐藏区域并定位首个错误；提交期间阻止重复创建。
- [ ] 创建成功进入 Run 详情，服务端校验仍是权威。
- [ ] 删除被替代的旧联动脚本并通过键盘、错误与提交回归检查。
