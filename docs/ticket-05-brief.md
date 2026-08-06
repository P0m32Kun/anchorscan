# 任务书：发布默认 catalog、文档与迁移验收（Ticket 05，收官票）

**本文件即任务书。** 先读仓库根 AGENTS.md，再执行本文件。

## 背景

知识库 v2 对接四票已闭环：Ticket 01 上游 catalog v2 已发布（Pentest-Playbook commit 57d739e，sha256 7d8ce203…）；Ticket 02 双格式 loader；Ticket 03 canonical command + -code 绑定；Ticket 04 服务端五档门禁。这是收官票：把 catalog 纳入发行物、配好默认路径、补齐文档与端到端验收。

真实产物防漂移 fixture 在 `internal/knowledgebase/testdata/catalog-v2-real.json`（来源与 checksum 已记录在同目录 README.md）。

## 必读

1. `docs/plans/catalog-json-knowledgebase/tickets/05-package-default-docs-and-acceptance.md`（行为契约与验收，逐条满足）
2. `docs/plans/catalog-json-knowledgebase/spec.md`（重点 4、6 节）
3. Makefile / release 打包脚本（搞清发行归档如何组装）
4. 配置加载与 `knowledge_base.path` 处理代码、配置页 UI、README、docs/deploy.md
5. `docs/reports/ticket-04-report.md`（前票验收口径参考）

## 行为契约（逐条实现）

- release/package 包含与程序版本匹配的 catalog v2，默认配置指向包内路径。
- 外部 catalog JSON/Markdown 路径仍可配置；缺失或不兼容文件显示明确 unavailable diagnostic，**不回退到未知副本**。
- 配置页、默认样例、部署文档说明：协议版本（v2 / source handbook-v3）、外部更新步骤、safety/status/legacy 行为边界和恢复方式。
- 发行 catalog 来源可追溯到 producer artifact checksum（用 testdata 已固化的 7d8ce203… 这份），运行时不得访问 Pentest-Playbook。

## 铁律

- 不移除 Markdown loader（ticket 非目标，另行立项）。
- 禁止 git commit/push；禁止修改 ~/DEV/Pentest-Playbook。
- 不运行真实 nuclei/nmap/msf 扫描器；手工验收用 fake runner 或既有测试链路。
- 默认样例、配置 UI placeholder、README/deploy 文档不得再指向只有开发机存在的 Playbook 路径。
- 报告实测/fake 分列；环境阻塞与代码失败明确区分，禁止伪造。

## 验收（全部实测，报告给命令与输出摘要）

```bash
make test
go vet ./...
make pr-check
```

- [ ] package smoke：发行归档实际包含 catalog；解压后默认配置启动加载，ready/degraded 状态符合预期（给出归档清单与启动日志证据）。
- [ ] handler/config 测试：外部 JSON 覆盖生效、legacy Markdown 回退生效、缺失/不兼容文件显示明确 unavailable diagnostic。
- [ ] 文档核查：README、docs/deploy.md、配置样例、配置 UI placeholder 全部不再引用开发机 Playbook 路径（附 grep 证据）。
- [ ] 手工验收记录覆盖：/kb 列表与详情、报告 enrich、safe 命令直通、受控命令门禁确认流、缺失 catalog 诊断；留截图或 curl 证据。
- [ ] ticket 05 状态改 done；spec.md Status 从 proposed 改为已落地状态（如 accepted/implemented，措辞与仓库惯例一致）。
- [ ] 报告写 `docs/reports/ticket-05-report.md`。
