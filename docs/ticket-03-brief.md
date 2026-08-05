# 任务书：canonical command 校验与 Nuclei -code 绑定（Ticket 03）+ source 契约修正 + 真实产物复验

**本文件即任务书。** 先读仓库根 AGENTS.md，再执行本文件。

## 背景

知识库 v2 对接的 spec 与 ticket 在 `docs/plans/catalog-json-knowledgebase/`。Ticket 02（双格式 loader）已完成。上游 Pentest-Playbook 的 catalog v2 **已真实发布**（commit 57d739e）：`version: 2`、`source: "handbook-v3"`、188 条目、145 条 command 投影、2 条 `nuclei -code` 真实条目（cve-2026-24061、cve-2017-7529）。

**已拍板的契约修正**：spec 4.1 节示例中的 `source: "handbook-v2"` 是笔误，正式值为 **`"handbook-v3"`**（与上游条目库目录及真实产物一致）。本任务须同步修正 consumer 常量、fixture、spec 文本。

## 必读

1. `docs/plans/catalog-json-knowledgebase/spec.md`（重点 4.1/4.2/5 节）
2. `docs/plans/catalog-json-knowledgebase/tickets/03-canonical-command-and-code-binding.md`（本任务的行为契约与验收，逐条满足）
3. `internal/knowledgebase/json.go`、`parse.go`、`catalog.go` 及对应测试
4. `internal/report/` 中 Nuclei 单条/批量命令绑定代码（BuildNucleiCommand 等）
5. 真实产物：`~/DEV/Pentest-Playbook/handbook-v3/dist/catalog.json`（只读，禁止修改上游仓库任何文件）

## 范围

1. **source 契约修正**：loader 接受的 source 从 `handbook-v2` 改为 `handbook-v3`；更新 `internal/knowledgebase/testdata/catalog-v2.json` 及所有引用；把 spec.md 4.1 的示例值改为 `handbook-v3` 并加一行修正说明；把 ticket 01 状态改为 done。
2. **canonical command 校验**（ticket 03 行为契约）：
   - loader 不从 `verify` 拼接字符串，只校验 `command` 受限语法及其与 `verify.tool`/`verify.code` 一致性；
   - Nuclei 只允许 `nuclei [-code] -t ... -u {{url|host:port}}`；`-code` 只能出现一次且紧跟 `nuclei`；`verify.code: true` 与 `-code` 双向一致；Nmap/MSF 不得携带 code；
   - `BuildNucleiCommand`、批量 Nuclei 命令与项目候选命令保留 `-code`，仍拒绝残留占位符与不一致批次；Nmap/MSF 既有严格绑定不放宽。
3. **真实产物复验（防漂移）**：把上游真实 `dist/catalog.json` 拷入 `internal/knowledgebase/testdata/`（如 `catalog-v2-real.json`），在 `testdata/README.md` 记录来源仓库、commit（57d739e）、协议版本与获取日期；新增测试断言真实 188 条目全部被 loader 接受，且 2 条真实 `-code` 条目通过一致性校验。**运行时不得依赖另一个工作区的路径**，只在测试 fixture 中固化副本。

## 铁律

- 禁止修改 `~/DEV/Pentest-Playbook` 任何文件。
- 禁止 git commit/push。
- 不增加 UI 确认逻辑（那是 Ticket 04）。
- 不为未来里程碑做占位实现；Nmap/MSF 行为不变。
- legacy Markdown 路径行为不变（仍 fail-closed legacy 条目）。

## 验收（全部实测，报告给命令与输出摘要）

```bash
go test ./internal/knowledgebase/... ./internal/report/...
go build ./...
```

- [ ] loader 接受真实 `-code` 条目；拒绝重复、错位、与 verify 不一致的 `-code`（正反例）。
- [ ] 单条与批量 Nuclei 输出保留 `-code`，目标替换后无未替换占位符。
- [ ] 真实 188 条目 fixture 全量被接受；2 条真实 `-code` 条目断言通过。
- [ ] Nmap/MSF 既有正反例不回归；legacy Markdown 测试不回归。
- [ ] spec 4.1 与 ticket 状态修正完成。
- [ ] 报告实测/fake 分列，写到 `docs/reports/ticket-03-report.md`。
