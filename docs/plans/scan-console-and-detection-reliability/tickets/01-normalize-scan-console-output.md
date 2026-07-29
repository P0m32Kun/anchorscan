# 01 - 规范扫描 Console 工具输出

**What to build:** 让实时 ScanEvent 保留可行动的进度、匹配和错误，同时把原始工具 banner、ANSI 控制符、版本/模板加载噪声及未匹配 Dameng 候选探测留在日志或 artifact，不进入用户事件流。

**Blocked by:** 无。

**Status:** done.

## 行为契约

- `storeProgress.Emit` 的日志输出保持原始消息；持久化 `ScanEvent.Message` 使用确定性、可读摘要。
- 摘要移除 ANSI、空白、banner、版本与模板加载噪声；对嵌入多行工具输出的失败，保留工具/目标上下文和最终可行动原因。
- Nuclei 的真实失败不得被吞掉。例如 `Could not run nuclei: no templates provided for scan` 必须仍可见。
- 未匹配 Dameng 候选端口不产生 Console 事件；匹配结果仍产生可见事件，且实际探测、DetectionCheck、状态和 artifact 均不变。
- 不使用 `-silent` 作为修复，不改变 Nuclei 的模板选择、授权边界或 Web 的原始 stdout/stderr 展开能力。

## 测试 seam

- Go unit：ANSI、多行 Nuclei banner/FTL、单行进度的摘要行为。
- App/Store 聚焦测试：原始日志/artifact 与事件摘要分离；Dameng 未匹配/匹配事件行为。

## 验收

- [x] 新增的摘要回归测试先以旧行为失败，再以最小实现转绿。
- [x] 失败 Nuclei 输出不再将 ANSI、banner 或重复文本持久化为 ScanEvent；日志和原始 artifact 保持取证用途。
- [x] ScanEvent 仍保留简洁的可行动错误及关键阶段进度。
- [x] 未匹配 Dameng 候选探测不污染事件流，匹配事件仍可见。
- [x] 聚焦测试、self-check、Standards/Spec 双轴只读评审和 `make pr-check` 全部通过；PR、合并与 Trellis complete gate 证据已记录。

## 非目标

- 改变历史 ScanEvent、日志或 artifact。
- 新增浏览器 stdout/stderr 展开、脱敏或权限模型。
