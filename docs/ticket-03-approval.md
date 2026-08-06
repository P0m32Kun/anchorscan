# 批准书：Ticket 03 实施批准

本文件与 `docs/ticket-03-brief.md` 一起阅读，任务书内容不变。

**批准计划。** 你 Phase 1 的调研结论与实施计划全部认可，按你列的 4 步进入 Phase 2 直接实施，不再中途停下确认：

1. source 契约修正（consumer 常量、fixture、拒绝例、spec 4.1 勘误、ticket 01 标 done）——注意 ticket 01 的状态文件在 `docs/plans/catalog-json-knowledgebase/tickets/01-publish-catalog-v2-contract.md`，改 **Status:** 字段为 done。
2. 真实产物固化：`~/DEV/Pentest-Playbook/handbook-v3/dist/catalog.json` 只读复制到 `internal/knowledgebase/testdata/catalog-v2-real.json`，README 记录来源/commit 57d739e/SHA-256/日期。
3. TDD：loader 正反例 + report 层单条/批量/项目候选 `-code` 绑定测试，最小改动 `BuildNucleiCommand`，不放宽 Nmap/MSF。
4. 报告写 `docs/reports/ticket-03-report.md`，实测/fake 分列。

验收命令不变：

```bash
go test ./internal/knowledgebase/... ./internal/report/...
go build ./...
```

授权范围：你为本任务创建/修改的所有文件均视为任务范围内，不再逐项确认。禁令不变：禁止 git commit/push；禁止修改 ~/DEV/Pentest-Playbook 任何文件；不加 UI 确认逻辑（Ticket 04 的事）；不做占位实现。
