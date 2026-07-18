# ADR-0003：持久化 DetectionCheck 历史执行事实

- 状态：Accepted
- 日期：2026-07-17
- 关联计划：`docs/plans/harden-scan-confidence/`

## 背景（Context）

现有报告保存 Fingerprint 和 Finding，但无法区分“检测执行完成且零发现”“规则不适用”“工具失败”或“进程中断”。如果报告根据当前 NSE/nuclei 规则反推覆盖，后续配置变化会改写历史 Run 的含义。

## 决策（Decision）

为每个 Run、Fingerprint 和检测引擎持久化实际 DetectionCheck：

- 本期引擎为 NSE 和 nuclei；httpx 是服务增强，不计作检测引擎。
- 状态为 `running`、`completed`、`skipped`、`failed`、`canceled`、`interrupted`。
- 跳过和失败保存稳定机器原因码与可读详情。
- 检测开始前写 running，结束后更新终态；取消和租约过期分别收敛为 canceled 与 interrupted。
- 报告只读取当次 Run 保存的 DetectionCheck，不使用当前规则重建历史。
- 覆盖汇总使用双引擎、单引擎、未覆盖数量，不输出漏洞覆盖率、百分比或安全保证。

## 后果（Consequences）

- 报告可以明确区分零发现、未执行和执行失败。
- 历史 Run 在规则变化后保持可审计。
- 数据库和 JSON 报告增加兼容字段；旧导出格式不改变。
- 每个 Fingerprint 固定产生两个检查事实，数据量线性增长，符合单机产品规模。
- 未来增加检测引擎时扩展 engine 值和展示逻辑，不预先建设插件系统。

