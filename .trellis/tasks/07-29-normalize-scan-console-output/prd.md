# 规范扫描控制台工具输出

## Goal

让扫描 Console 保留阶段、进度与可行动错误，同时从用户事件流排除工具 banner、ANSI、版本噪声和未命中 Dameng 探测细节。

## Confirmed Facts

- `storeProgress.Emit` 目前将同一字符串写入日志和 `ScanEvent`；nuclei 的失败错误会带回完整工具输出。
- `scanTarget` 为每个可选工具阶段先写 artifact，再发出失败进度；`dameng-probe` 每次候选探测都会发一条 info。
- 历史 `tool run` 已有 ANSI 归一化，但扫描 Run 的 `ScanEvent` 没有该保护。

## Requirements

- 日志和 artifact 保持原始诊断；ScanEvent 只存可读摘要。
- 摘要必须移除 ANSI 与 banner/版本/模板加载噪声，保留工具、目标、失败类型和可行动终因（例如 `Could not run nuclei: no templates provided for scan`）。
- 未命中的 Dameng 候选探测不产生 Console 事件；匹配结果仍可见。
- 不把 `-silent` 作为修复手段，也不吞掉真实失败。

## Acceptance Criteria

- [ ] 失败 Nuclei 输出不会将 banner/ANSI/重复文本写入 ScanEvent，日志/原始 artifact 仍可用于取证。
- [ ] ScanEvent 仍保留简洁的可行动错误与关键阶段进度。
- [ ] 未命中 Dameng 探测不再污染事件流，匹配事件保留。
- [ ] 覆盖该 seam 的单元或集成回归测试通过。

## Out of Scope

- Web UI 原始 stdout/stderr 展开、权限或脱敏设计。
- 改变 Nuclei 模板选择或扫描授权。
