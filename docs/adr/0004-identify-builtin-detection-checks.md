# ADR-0004：用 check_id 区分内置探针检测事实

- 状态：Accepted
- 日期：2026-07-20
- 关联计划：`docs/plans/add-builtin-vulnerability-probes/`

同一 Fingerprint 可能适用多个 Builtin Probe，仅以 engine 标识 DetectionCheck 会使探针结果互相覆盖，也无法审计历史 Run 实际执行了哪些检查。因此 DetectionCheck 增加可选 `check_id`，事实键扩展为 Run、Fingerprint 自然键、engine 与 `check_id`；既有 NSE/nuclei 记录保留空 `check_id`。这项决策细化 ADR-0003 的历史事实原则，而不改变 DetectionCheck 只记录实际执行、不表达漏洞覆盖保证的语义。

拒绝把全部内置探针聚合成一条 `builtin` 检查，因为它会丢失单探针失败、超时和阴性结论；也拒绝把每个漏洞 ID 伪装成 engine，因为那会破坏引擎维度的报告与筛选。
