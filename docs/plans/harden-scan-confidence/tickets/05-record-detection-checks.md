# 05 — 持久化 DetectionCheck 并显示实时摘要

**What to build:** 为完整扫描中的每个 Fingerprint 保存 NSE 与 nuclei 的实际执行记录，并在 Run 页面实时展示六种状态计数。

**Blocked by:** None — can start immediately.

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] 新 migration 和 store 公共 API 支持 DetectionCheck upsert、列表和按状态聚合。
- [x] 自然键为 Run + IP + port + protocol + engine；历史记录不从当前规则反推。
- [x] 正常路径在调用前写 running，成功后写 completed；Finding 为零仍是 completed。
- [x] 不适用路径为 skipped，并使用 `no_matching_rule`、`tool_unconfigured`、`missing_target` 等稳定原因码。
- [x] NSE 与 nuclei 独立形成事实；httpx 不计作 DetectionCheck。
- [x] Run 状态 HTTP 响应以兼容字段返回 running/completed/skipped/failed/canceled/interrupted 计数，页面轮询显示紧凑摘要。
- [x] store/app/Web 测试只观察公共行为，不断言 SQL 文本或页面 CSS 类。
