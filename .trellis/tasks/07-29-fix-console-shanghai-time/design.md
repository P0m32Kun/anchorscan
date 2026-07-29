# Console 上海时间设计

## 边界

仅在 Vue 文本投影层格式化 `ScanEvent.time`。新增纯 TypeScript formatter，由 `RunDetail.vue` 和 `ToolRunFeedback.vue` 共同调用；Store/API 继续返回 UTC RFC3339。

## 契约

有效值使用 `Intl.DateTimeFormat` 的 `Asia/Shanghai` 时区，拼成 `YYYY-MM-DD HH:mm:ss.SSS UTC+8`，不得依赖浏览器本地时区。无效或无法解析的输入返回明确的原始值回退（如 `invalid time: <value>`），不抛异常。只替换输出文本，事件数组顺序和 after_id 轮询不变。

## 风险与回滚

无持久化/API 变更；删除 formatter 调用即可回退。
