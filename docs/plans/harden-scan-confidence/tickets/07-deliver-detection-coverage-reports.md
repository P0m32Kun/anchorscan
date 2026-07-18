# 07 — 交付检测执行覆盖报告

**What to build:** 在 terminal Run 的 Web、JSON 和独立 HTML 报告中展示逐 Fingerprint DetectionCheck，并提供保守的双引擎、单引擎、未覆盖数量汇总。

**Blocked by:** `06-preserve-partial-results.md`。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [ ] Web 报告按 Fingerprint 显示 NSE/nuclei 状态、原因和详情，运行中页面仍只显示摘要。
- [ ] 仅 completed 计入成功执行覆盖：两个 completed 为双引擎，一个为单引擎，零个为未覆盖。
- [ ] failed/canceled/interrupted/skipped 另行显示数量和逐项事实，不被算成成功覆盖。
- [ ] JSON 增加可选 `detection_checks` 字段；旧字段、顺序和消费者兼容。
- [ ] 独立 HTML 消费同一数据并保持自包含。
- [ ] 现有 IP、IP:PORT、URL、CSV 导出行为完全不变。
- [ ] Web HTTP、JSON 和独立报告测试证明分类一致且不输出百分比或安全保证。

