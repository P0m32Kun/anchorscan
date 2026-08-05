# 05 — 发布默认 catalog、文档与迁移验收

**What to build:** 将匹配 catalog 纳入 AnchorScan 发行物，配置可用默认路径，并完成双格式迁移的端到端验收与运维文档。

> **2026-08-05 返工（设计反转，单源模式）**：本票原验收（commit ce6af34）把 catalog v2 冻结副本打进发行物并作为默认配置；用户拍板反转——发行物不再携带 catalog，默认配置留空（禁用 + 明确诊断），README/配置页指引 clone 知识库仓库（Pentest-Playbook）并把 `knowledge_base.path` 指向 `handbook-v3/dist/catalog.json`。以下行为契约已按单源模式重写；返工验收见 `docs/reports/ticket-05-single-source-report.md`。

**Blocked by:** 04 — 在所有命令出口强制 safety 与 review 门禁。

**Status:** done

**Execution skills:** `implement`、`code-review`、`ponytail`。

## 行为契约（单源模式，2026-08-05 修订）

- release/package **不包含** catalog；默认配置 `knowledge_base.path` 为空（知识库禁用），首次自动生成的配置不指向任何包内文件。
- 外部 catalog JSON/Markdown 路径可配置；缺失或不兼容文件显示明确 unavailable diagnostic，不回退到未知副本。
- 配置页、默认样例、部署文档说明 clone 知识库仓库 → 配置路径 → 重启的流程、协议版本（v2 / source handbook-v3）、git pull 更新步骤、safety/status/legacy 的行为边界和恢复方式。
- 测试 fixture 锁定 producer artifact checksum（仅用于测试，不进发行物）；运行时只读取操作者显式配置的路径，不要求访问 Pentest-Playbook。

## 测试 seam

- package smoke：archive **不含** catalog，解压后的默认配置启动时知识库 disabled 且诊断清晰；
- handler/config 测试：外部 JSON 覆盖、legacy Markdown 回退和 unavailable diagnostic；
- `make pr-check`。

## 验收

- [x] package smoke 证明发行包**不含** catalog 且默认启动知识库 disabled、诊断清晰（返工验收：`docs/reports/ticket-05-single-source-report.md`）。
- [x] 默认样例、配置 UI placeholder、README/deploy 文档不再指向只有开发机存在的 Playbook 路径，且无“开箱自带 catalog”表述。
- [x] 外部路径和 legacy Markdown 的迁移/回退说明可复现。
- [x] `make test`、`go vet ./...`、`make pr-check` 通过。
- [x] 手工验收记录覆盖 `/kb`、报告 enrich、safe 命令、受控命令和缺失 catalog 诊断；不运行真实扫描器。

## 非目标

- 不移除 Markdown loader；该决策等待 v2 部署迁移证据后另行立项。
