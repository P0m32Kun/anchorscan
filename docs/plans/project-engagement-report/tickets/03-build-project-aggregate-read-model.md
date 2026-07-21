# 03 — 构建跨 Runs 的 Project 聚合读模型

**What to build:** 从 SQLite 一次读取 Project 已纳入 Runs 的 Finding、Fingerprint 与 DetectionCheck 上下文，构建按 Zone、漏洞和端点稳定去重的纯 Project Report 候选模型。

**Blocked by:** 02 — 创建归属 Zone 的 Scan Runs。

**Status:** draft

- [ ] 查询结果保留 RunID/ZoneID，不从 report.json 或 handler 递归聚合。
- [ ] 复用 Catalog.Match、ObservationFromFinding 和现有漏洞分组身份。
- [ ] 同一漏洞同一 IP:port 跨重复 Runs 只出现一次，来源 Runs 仍可审计。
- [ ] info 不进入正向漏洞候选。
- [ ] 未匹配/歧义项不丢失且不能猜选知识库条目。
- [ ] `BuildProjectReport` 是 handler 与测试共用的唯一聚合 seam。
