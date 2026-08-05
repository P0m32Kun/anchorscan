# 05 — 发布默认 catalog、文档与迁移验收

**What to build:** 将匹配 catalog 纳入 AnchorScan 发行物，配置可用默认路径，并完成双格式迁移的端到端验收与运维文档。

**Blocked by:** 04 — 在所有命令出口强制 safety 与 review 门禁。

**Status:** done

**Execution skills:** `implement`、`code-review`、`ponytail`。

## 行为契约

- release/package 包含与程序版本匹配的 catalog v2，默认配置指向包内路径。
- 外部 catalog JSON/Markdown 路径仍然可配置；缺失或不兼容文件显示明确 unavailable diagnostic，不回退到未知副本。
- 配置页、默认样例、部署文档说明：协议版本、外部更新步骤、safety/status/legacy 的行为边界和恢复方式。
- fixture 及发布 catalog 的来源可追溯到 producer artifact checksum，不要求运行时访问 Pentest-Playbook。

## 测试 seam

- package smoke：archive 包含 catalog，解压后的默认配置可加载；
- handler/config 测试：外部 JSON 覆盖、legacy Markdown 回退和 unavailable diagnostic；
- `make pr-check`。

## 验收

- [ ] package smoke 证明发行包实际包含 catalog 且默认启动加载 ready/degraded 的预期状态。
- [ ] 默认样例、配置 UI placeholder、README/deploy 文档不再指向只有开发机存在的 Playbook 路径。
- [ ] 外部路径和 legacy Markdown 的迁移/回退说明可复现。
- [ ] `make test`、`go vet ./...`、`make pr-check` 通过。
- [ ] 手工验收记录覆盖 `/kb`、报告 enrich、safe 命令、受控命令和缺失 catalog 诊断；不运行真实扫描器。

## 非目标

- 不移除 Markdown loader；该决策等待 v2 部署迁移证据后另行立项。
