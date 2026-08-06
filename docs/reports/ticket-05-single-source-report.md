# Ticket 05 返工验收报告：catalog 单源架构（设计反转）

- 日期：2026-08-05
- 范围：`docs/ticket-05-single-source-brief.md`（本文件即任务书）；反转对象为已验收 Ticket 05（commit ce6af34）的"发行物内置 catalog + 默认指向包内路径"设计
- 决策：用户拍板方案 A 彻底单源——catalog 只在知识库仓库（Pentest-Playbook `handbook-v3/dist/catalog.json`）更新；Anchorscan 发行物不带副本；默认配置 `knowledge_base.path` 留空（禁用 + 清晰诊断）；README/配置页指引 clone + 手动配路径
- 风险等级：中（发行物组装语义反转 + 测试契约反转 + 文档/规格一致性；无新扫描执行路径）

## 改动点清单

**发行物与默认配置**
- 删除 `config/catalog.json`（`rm` 删除，未做任何 git 操作）。
- `Makefile`：`RUNTIME_CONFIG` 移除 `catalog.json`（cp 列表与归档内非空存在双重校验随之不再涉及它）。
- `internal/config/init.go`：`defaultConfig()` 不再写 `KnowledgeBase.Path`（默认为空），首次自动生成的配置 `knowledge_base: path: ""`，不指向任何包内文件；doc 注释改写为单源说明。
- `config/default.yaml.example`：knowledge_base 注释块改写为单源说明（克隆仓库示例路径 `~/Pentest-Playbook/handbook-v3/dist/catalog.json`、协议要求 v2/source handbook-v3、更新方式 = 克隆仓库内 git pull、留空=禁用）；`path: ""`。
- `internal/knowledgebase/parse.go`：disabled 分支（空 path）增加明确诊断消息（"未配置 knowledge_base.path，无知识库文件可加载；发行包不再附带 catalog，请克隆知识库仓库…并重启"），使 `/kb` 页在禁用时也显示原因。

**测试调整**
- `internal/knowledgebase/catalog_drift_test.go`：原"config/catalog.json 与 fixture 字节一致 + 固定 checksum"双锁测试改为三条：
  - `TestNoShippedCatalogCopy`：断言 `config/catalog.json` 不存在（防止副本悄悄回归，含 Makefile cp 列表或新文件两种回归路径）；
  - `TestFrozenProducerFixtureChecksum`：`testdata/catalog-v2-real.json` 仍锁定 producer SHA-256（`7d8ce203…`，仅测试 fixture，不进发行物）；
  - `TestEmptyPathDisablesKnowledgeBase`：空 path → `StatusDisabled`、零条目、且带非空诊断。
- `scripts/package_smoke_test.go`：删除"归档必须含 catalog + checksum 匹配 + 默认配置 ready 188"断言，改为：
  - `assertNoPackagedCatalog`：解包目录与 tar 成员清单双重断言**不含** `config/catalog.json`；
  - `assertDefaultConfigDisablesKnowledgeBase`：默认配置 `path` 为空 → `knowledgebase.Load` disabled 且带诊断 → `web.NewServer` 发 GET `/kb` 断言徽章 `disabled` + "未配置 knowledge_base.path" + 零条目。
- `internal/config/init_test.go`：`TestInitDefaultKnowledgeBasePointsAtPackagedCatalog` → `TestInitDefaultKnowledgeBasePathIsEmpty`（默认 path 为空）。
- 保留未动的既有主路径测试：外部 JSON 覆盖（`TestKnowledgeBaseLoadsExternalJSONCatalog`）、legacy Markdown 回退（`TestKnowledgeBaseLegacyMarkdownLoadsAsLegacyUnknown`）、缺失（`TestKnowledgeBaseMissingPathShowsUnavailableDiagnostic`）、不兼容（`TestKnowledgeBaseIncompatibleFileShowsUnavailableDiagnostic`）——这些正是单源模式的主路径；`TestReportPageVulnerabilityAggregateExplainsDisabledCatalog` 显式空 path 分支维持不变。
- `internal/knowledgebase/testdata/catalog-v2-real.json` 保留（测试 fixture，不进发行物）。
- 未触碰：legacy Markdown loader（非目标，铁律）、`~/DEV/Pentest-Playbook`。

**文档与 UI**
- `README.md`：KB 章节"发行自带 catalog"段删除，改为"获取知识库（catalog 单源）"clone 流程（clone 知识库仓库 → `knowledge_base.path` 指向 `handbook-v3/dist/catalog.json` → 重启）；协议版本、unavailable 诊断不回退、safety/status/legacy 边界保留；恢复方式改为还原/重新 clone 知识库仓库。
- `docs/deploy.md`：知识库（catalog）运维章节改写为单源模式（归档不含 catalog、clone+配置路径+重启、git pull 更新、诊断与恢复、safety/status/legacy 边界、升级时旧配置无 path 保持禁用）。
- `internal/web/templates/config.html`：placeholder 改为 `~/Pentest-Playbook/handbook-v3/dist/catalog.json`，帮助文本改写为单源 clone 流程 + git pull 更新 + 协议要求 + legacy 边界 + unavailable 诊断与恢复。
- `CHANGELOG.md`：[Unreleased] Added/Changed 改写为单源模式表述。
- `scripts/web-smoke.mjs` 无需改动（其 config 注入正则仍兼容新的注释块 + `path: ""` 结构，pr-check 实测通过）。

**规格勘误（spec.md）**
- 文首新增"2026-08-05 设计反转勘误（单源模式）"说明，列出受影响段落。
- §2 目标 5 改为"可部署默认值（单源）"（发行物不带 catalog、默认空路径禁用）；目标 6 改为"只读取操作者显式配置的克隆仓库路径，不访问未配置的外部仓库"。
- §7 分发、配置和迁移：改"发布归档应包含版本匹配 catalog"为"发布归档不包含 catalog.json"，迁移阶段第 4 步改为"单源默认发行：移除包内 catalog 副本，默认禁用 + 文档指引 clone 与手动配置路径（2026-08-05 返工）"。
- §8 验收第 6 条：package/config 改为"归档不包含 catalog，默认 path 为空且禁用、诊断清晰；外部 JSON 路径配置后仍可覆盖加载"。
- §9 风险表"默认配置指向不存在的外部文件"一行控制措施改为"默认 path 留空不指向任何文件；配置外部路径后由 unavailable 诊断与 package smoke 验证"。
- §10 实施状态：注明 2026-08-05 设计反转，fixture 保留并锁定 producer checksum、不进发行物。
- 文档链接全部通过 `scripts/check_markdown_links.mjs`（doc-check）。

**ticket 05 契约修订**
- `tickets/05-package-default-docs-and-acceptance.md`：文首新增"2026-08-05 返工（设计反转，单源模式）"说明；行为契约 4 条全部按单源重写；测试 seam 与验收清单同步修订并勾选；Status 保持 `done`（返工后重新验收）。

## 实测：必需验收

```bash
make test
```

结果：通过（Go 全部包 ok；node --test 19/19）。

```bash
go vet ./...
```

结果：`EXIT=0`（无输出）。

```bash
make pr-check
```

结果：`EXIT=0`（test、doc-check `Markdown local links and focused documentation contracts are valid.`、docx-test 5/5、build vite ✓、package-test `ok .../scripts 5.875s`、web-smoke `Web browser smoke test passed.`）。

### 仓库与发行归档均无 config/catalog.json（grep + 解包清单证据）

```text
$ ls config/
default.yaml.example  nse.yaml  ports-highrisk.txt  ports-top1000.txt  service-tags.yaml

$ tar -tzf dist/anchorscan-v2.0.5-10-g4a771ab-dirty-darwin-arm64.tar.gz | grep -E "catalog|default.yaml"
anchorscan-v2.0.5-10-g4a771ab-dirty-darwin-arm64/config/default.yaml.example
$ tar -tzf ...tar.gz | grep -c "config/catalog.json"
0
```

归档唯一与 catalog 相关的成员是 `default.yaml.example`（含单源说明注释）。旧归档（`v2.0.5-8-gdaf7064-dirty`）为过期构建产物，已随本次 `make package` 一并清理（dist 为 gitignore 构建输出）。代码中残余 `catalog.json` 引用均为：测试临时目录自建 fixture（`safety_gate_test.go`、`knowledgebase_test.go`、`parse_test.go`、`ticket-04-web-smoke.mjs`）、单源断言/注释（`catalog_drift_test.go`、`package_smoke_test.go`、`init.go`）——非发行副本。

### 首次启动自动生成配置 path 为空；/kb disabled 诊断明确（实测，包内二进制）

```text
$ ./anchorscan web --config /tmp/fresh/default.yaml --db /tmp/fresh/scan.db --listen 127.0.0.1:18091
listening on http://127.0.0.1:18091
$ grep -A1 "^knowledge_base:" /tmp/fresh/default.yaml
knowledge_base:
    path: ""
$ curl -s http://127.0.0.1:18091/kb | grep -oE 'knowledgebase-status">[a-z]+|未配置[^<]*'
knowledgebase-status">disabled
未配置 knowledge_base.path，无知识库文件可加载；发行包不再附带 catalog，请克隆知识库仓库（Pentest-Playbook）后将路径指向其 handbook-v3/dist/catalog.json 并重启
```

### 配置指向外部 catalog 后 /kb ready 188 条（实测，用 testdata fixture 代替真克隆）

```text
$ cat external.yaml
knowledge_base:
  path: /private/tmp/anchorscan-catalog-json/internal/knowledgebase/testdata/catalog-v2-real.json
$ curl -s http://127.0.0.1:18092/kb | grep -oE 'knowledgebase-status">[a-z]+'
knowledgebase-status">ready
$ curl -s http://127.0.0.1:18092/kb | grep -oE 'href="/kb/[^"]+"' | sort -u | wc -l
188
```

### 外部路径 / 回退 / 诊断测试（handler 级自动断言，pr-check 全链路覆盖）

- 外部 JSON 覆盖：`TestKnowledgeBaseLoadsExternalJSONCatalog`（ready + 外部条目 + 详情页审计块）通过；
- legacy Markdown 回退：`TestKnowledgeBaseLegacyMarkdownLoadsAsLegacyUnknown`（legacy-unknown + "旧 Markdown 未声明 safety"）通过；
- 缺失文件：`TestKnowledgeBaseMissingPathShowsUnavailableDiagnostic`（unavailable + no such file + 零条目）通过；
- 不兼容文件：`TestKnowledgeBaseIncompatibleFileShowsUnavailableDiagnostic`（unavailable + "catalog JSON 无效" + 零条目）通过；
- 手工 curl 补充：配置指向不存在文件 → `knowledgebase-status">unavailable` + `no such file or directory`。

### 文档无残留"自带 catalog"表述（grep 证据）

```text
$ grep -rn "自带 catalog\|内置 catalog\|开箱可用\|包内 config/catalog\|包内路径\|包内默认" README.md docs/deploy.md CHANGELOG.md internal/web/templates/config.html config/default.yaml.example
（无输出，exit=1）
```

README/deploy/config UI/默认样例中仅存的 "Pentest-Playbook" 表述为单源指引（clone 知识库仓库），非开发机路径配置。spec.md 与 tickets/05 中的旧表述仅作为"设计反转勘误/返工说明"保留。

## 模拟 / 未实测分列

**已实测（自动断言或真实命令驱动）：** `make test`、`go vet ./...`、`make pr-check` 全链路；package smoke（归档无 catalog + 默认 disabled 诊断）；fresh 自动生成配置 path 为空 + `/kb` disabled 诊断（真实二进制 + curl）；外部 catalog 路径 ready 188 条（真实二进制 + curl，fixture 即冻结 producer artifact）；缺失路径 unavailable 诊断（真实二进制 + curl）；README/deploy/CHANGELOG/配置 UI grep 证据。

**未实测（需编排方终验）：**
- 真实 clone Pentest-Playbook 后的端到端配置流程（按任务书用 testdata 本地副本实测；真实克隆路径的协议与 checksum 已由 fixture 锁定，配置加载语义与本地副本完全一致）。
- 归档在 linux/windows 平台的解压启动（本环境 darwin/arm64；package-smoke 的归档组装与解包断言与平台无关）。
- 截图/trace 的人工目视复核（模型不支持读图；内容由 smoke 的 DOM 断言间接保证，同 ticket 04/05 口径）。
- 真实扫描器执行（本票非目标；未运行任何扫描器）。

**说明与降级：** 无环境阻塞、无代码失败。`make pr-check` 会重跑 `ticket-04-web-smoke.mjs`（`npm run test:web` 串行执行两个 smoke），按仓库既有行为重新生成 `docs/reports/ticket-04-playwright/` 下已提交的截图/server.log/trace.zip（与 ticket 04/05 报告同口径），工作区 diff 中可见这些产物字节变化，非本票有意改动。

## 审查与剩余项

- 未执行 git commit/push/reset/checkout；`config/catalog.json` 用 `rm` 删除，留待编排方提交；未修改 `~/DEV/Pentest-Playbook`；未移除 Markdown loader。
- 行为契约（单源修订版）逐条核对：发行物与仓库无 catalog 副本 ✓；默认 path 为空且 /kb disabled 诊断明确 ✓；外部 JSON/Markdown 可配置、缺失/不兼容 unavailable 不回退 ✓；文档与配置页为 clone → 配置路径 → 重启流程，无"开箱自带"表述 ✓；fixture 锁定 producer checksum 且运行时只读显式配置路径 ✓。
- 残余风险：升级用户旧配置无 `knowledge_base.path` 时知识库保持禁用（deploy.md 已说明）；`TestNoShippedCatalogCopy` 防止发行副本回归（若未来 reintroduce，测试立即失败）。
