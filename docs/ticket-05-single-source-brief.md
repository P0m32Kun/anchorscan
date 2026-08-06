# 任务书：Ticket 05 返工——catalog 单源架构（设计反转）

**本文件即任务书。** 先读仓库根 AGENTS.md，再执行本文件。

## 背景与决策

已验收的 Ticket 05（commit ce6af34）把 catalog v2 冻结副本打进发行物并作为默认配置。**用户拍板反转该设计**：

> catalog.json 只在知识库仓库（Pentest-Playbook，`handbook-v3/dist/catalog.json`）更新；Anchorscan 发行物不带副本；用户下载 Anchorscan 后自行 clone 知识库仓库，在配置文件里手动配置 `knowledge_base.path` 指向克隆仓库的 catalog。

选定方案 **A：彻底单源**——发行包不带 catalog.json，默认配置留空（禁用 + 清晰诊断），README/配置页指引 clone + 手动配路径。

## 必读

1. `docs/ticket-05-brief.md` 与 `docs/reports/ticket-05-report.md`（原验收口径，理解要改什么）
2. `docs/plans/catalog-json-knowledgebase/spec.md`（目标 5/6 与 §6 涉及"发行物带 catalog"的段落需勘误）
3. `internal/config/init.go`（defaultConfig 的 knowledge_base.path）、`config/default.yaml.example`
4. `Makefile`（package 步骤 cp 列表）、`scripts/` 下 package smoke 测试
5. `internal/knowledgebase/catalog_drift_test.go`（锁死 config/catalog.json 的测试，需处理）
6. README.md、docs/deploy.md、`internal/web/templates/config.html`（含克隆指引的改写点）

## 范围

1. **移除发行副本**：删除仓库内 `config/catalog.json`（直接 rm 文件即可，不要 git 操作）；Makefile 打包 cp 列表去掉它；`config/default.yaml.example` 的 knowledge_base 注释块改为单源说明（指向克隆仓库的 dist/catalog.json，示例路径 + 协议要求 + 更新方式 = git pull）；`defaultConfig()` 的默认 path 改为空（禁用），确保首次自动生成的配置**不指向任何包内文件**。
2. **测试调整**：
   - `catalog_drift_test.go`：删除或改为"不再存在 config/catalog.json"的断言（防止副本悄悄回归）；
   - package smoke：改断言为**归档不含 catalog.json**，且默认配置启动时 KB 为 disabled/unavailable 且诊断清晰；
   - 保留并跑通既有"外部 JSON 路径加载 / legacy Markdown 回退 / 缺失与不兼容 unavailable"测试（这些正是单源模式的主路径）。
   - `internal/knowledgebase/testdata/catalog-v2-real.json` **保留**（测试 fixture，不进发行物）。
3. **文档与 UI**：README KB 章节、deploy.md、config.html 改为"clone Pentest-Playbook → 把 knowledge_base.path 指向 handbook-v3/dist/catalog.json → 重启"流程；删除一切"开箱自带 catalog"表述；保留协议版本（v2/source handbook-v3）、safety/status/legacy 行为边界、unavailable 诊断说明。
4. **规格勘误**：spec.md 中"发布归档带有与程序版本匹配的 catalog / 默认配置指向包内路径"的目标与条款，改写为单源模式并加"2026-08-05 设计反转"勘误说明；ticket 05 行为契约同步修订并注明返工。
5. 报告写 `docs/reports/ticket-05-single-source-report.md`。

## 铁律

- 禁止 git commit/push/reset/checkout；删除 config/catalog.json 用 rm，留待编排方提交。
- 禁止修改 ~/DEV/Pentest-Playbook 任何文件。
- 不运行真实扫描器；legacy Markdown loader 不移除。
- 报告实测/fake 分列，环境阻塞与代码失败区分，禁止伪造。

## 验收（全部实测，报告给命令与输出摘要）

```bash
make test
go vet ./...
make pr-check
```

- [ ] 仓库与发行归档均无 config/catalog.json（grep + 解包清单证据）。
- [ ] 首次启动自动生成的默认配置 `knowledge_base.path` 为空；/kb 显示 disabled/unavailable 且诊断明确（无文件可加载）。
- [ ] 配置指向外部克隆仓库 catalog.json 后 /kb ready 188 条（用 testdata 或任意本地副本路径实测即可，不用真克隆）。
- [ ] 外部 JSON 覆盖、legacy Markdown 回退、缺失/不兼容诊断测试全部通过。
- [ ] README/deploy/config UI 无残留"自带 catalog"表述（grep 证据）；spec 勘误完成。
- [ ] ticket 05 与 spec 的修订在报告中列出改动点。
