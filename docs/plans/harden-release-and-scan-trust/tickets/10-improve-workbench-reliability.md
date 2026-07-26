# 10 — 改善 Workbench 失败恢复与职责边界

**What to build:** 让 Verification 创建后的 Evidence 部分上传失败可见、可重试，并沿真实变化轴缩小 Workbench 的职责。

**Blocked by:** 09 — 强化发布与 CI 门禁。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- Verification 创建成功但部分 Evidence 上传失败时，用户看到每个文件状态和重试入口；已成功内容不丢失。
- 对话框重复提交被抑制，关闭后 object URL 被释放，读取详情失败不能静默。
- Go→JSON→TypeScript DTO 对空数组、null 和未知 enum 有稳定降级。
- 只在行为变化需要时抽取纯函数、API client 和对话框；保持 Vue islands。
- 键盘、焦点恢复和动态错误提示满足现有可访问性契约。

## 测试 seam

- Go `httptest`：稳定 API 错误与上传状态。
- JavaScript unit：纯过滤/状态转换。
- Playwright：一个部分上传失败后重试的用户旅程及焦点恢复。

## 验收

- [ ] 先写部分 Evidence 失败会失去恢复入口的失败测试。
- [ ] 实现最小逐文件状态和重试，不引入全局状态库。
- [ ] 抽取 DTO/API client 与实际修改涉及的 dialog，未触及区域不机械拆分。
- [ ] 相关 object URL、重复提交和错误反馈有行为测试。
- [ ] `npm run typecheck:web`、聚焦测试、`make test`、`make pr-check` 通过。

## 非目标

- 不迁移 SPA、Pinia/Redux，不按文件行数拆 CSS，不增加视觉快照平台。
