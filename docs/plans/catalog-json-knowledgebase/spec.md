# Spec — Playbook catalog v2 知识库消费与安全门禁

**Status:** implemented

> **2026-08-05 设计反转勘误（单源模式）**：此前“发布归档带有与程序版本匹配的 catalog、默认配置指向包内路径”的目标与条款已全部作废。catalog 只在知识库仓库（Pentest-Playbook `handbook-v3/dist/catalog.json`）更新；Anchorscan 发行物不再携带 catalog 副本，默认配置 `knowledge_base.path` 留空（知识库禁用 + 明确诊断），由操作者 clone 知识库仓库并手动配置路径。涉及段落：§2 目标 5/6、§7、§8(6)、§9、§10，已逐处改写并标注。

本文定义 AnchorScan 以 Pentest-Playbook `handbook-v3` 的机器消费 catalog 为知识库输入的迁移。它替换已失效的“只加 JSON loader、保持旧 `Entry` 和命令调用链不变”的方案。

## 1. 背景与结论

Pentest-Playbook 已发布 handbook-v3 条目库。每个条目的 `verify` 与 `safety` 共同定义验证命令及其授权边界；生成 Markdown 和 `dist/catalog.json` 都是条目源数据的投影。

当前已发布 catalog v1 的真实形状包含：

- 188 个条目；145 个有 `verify`（nuclei 125、nmap 17、msf 3）；
- `safety` 为所有条目的必填字段：safe 119、optional 15、manual-gated 54；
- `status` 为 stable 84、needs-review 104；
- `verify.code` 已用于两个 Nuclei 条目，生成 `nuclei -code ...`；
- `sources`、`generated`、`safety.effects` 和条件必填的 `safety.cleanup` 已是生产数据的一部分。

旧 AnchorScan 模型只保存三个命令字符串，既不能携带安全门禁，也在 loader 和命令绑定路径中拒绝 `-code`。因此 JSON 迁移不得仅视为格式转换，也不能静默丢弃 v3 的安全与审计语义。

## 2. 目标

1. **安全消费**：AnchorScan 仅消费发布并校验过的 catalog v2；JSON 条目的 `safety`、`status`、`sources` 和 `generated` 被保留到知识库模型。
2. **单一命令构造源**：Pentest-Playbook 从 `verify` 生成 canonical `command` 投影；AnchorScan 只验证、解析、绑定目标，不重写 Python `render_command`。
3. **服务端门禁**：safe、optional、manual-gated、legacy/unknown 和 needs-review 条目在所有命令暴露路径上遵循可验证的授权规则；不能只依赖浏览器 UI。
4. **双格式迁移**：`knowledge_base.path` 以扩展名分派 catalog JSON 与旧 Markdown。旧 Markdown 仍可阅读和匹配，但其命令不能被当作已声明安全级别。
5. **可部署默认值（单源）**：catalog 只在知识库仓库（Pentest-Playbook `handbook-v3/dist/catalog.json`）维护与发布；发布归档**不携带** catalog，默认配置 `knowledge_base.path` 为空（知识库禁用，`/kb` 显示 disabled 与明确诊断）。操作者 clone 知识库仓库并配置外部路径后启用。
6. **可审计兼容性**：生产仓库与 AnchorScan fixture 的来源、协议版本和校验值可追溯（fixture 锁定 producer artifact checksum，仅用于测试）；运行时只读取操作者显式配置的克隆仓库路径，不访问未配置的外部仓库，也不依赖开发机工作区。

## 3. 非目标

- 不实现知识库热加载；仍在 `NewServer` 启动时加载。
- 不让 AnchorScan 解释或执行 Playbook 条目正文中的扩展知识小节。
- 不把 `safety` 当作权限系统；本项目没有身份/审批服务。它是服务器强制的显式操作者确认与风险呈现边界。
- 不自动执行 optional 或 manual-gated 检测。
- 不在本特性中移除旧 Markdown loader；移除须在确认无部署依赖后另行立项。

## 4. 跨仓库 catalog 协议

### 4.1 catalog v2 是实施前置条件

Pentest-Playbook 必须先发布 `catalog.json` **version: 2**，并同时发布/校验 `handbook-v3/schema/catalog.schema.json`。当前 v1 虽有 `safety` 和 `code`，但没有正式的 consumer 协议边界；AnchorScan 不应通过“继续接受 v1 并忽略新字段”完成迁移。

勘误：本节先前将正式 `source` 写为 `"handbook-v2"`；应为 `"handbook-v3"`。

catalog v2 顶层必须包含：

```json
{
  "version": 2,
  "source": "handbook-v3",
  "entry_count": 188,
  "entries": []
}
```

consumer 必须检查：JSON 语法、`version == 2`、`source == "handbook-v3"`、`entry_count == len(entries)`、条目 ID 唯一且至少有一个可展示条目。顶层或条目新增的**不影响既有语义**字段可忽略；影响消费、命令、授权、匹配或审计含义的变化必须提升 catalog 版本，并同步 producer schema、fixture 和 consumer。

### 4.2 条目最小消费契约

每个 catalog v2 条目必须投影以下字段：

| 字段 | AnchorScan 语义 |
|---|---|
| `id`、`title`、`severity` | 现有 `Entry` 标识、显示与报告严重性 |
| `match` | nuclei/nse/manual-review/cve 匹配键 |
| `sections["漏洞描述"]`、`sections["修复建议"]` | 报告 enrich 与 KB 详情 |
| `status` | `stable` 或 `needs-review`；影响命令确认和 UI 标记 |
| `safety.mode`、`safety.effects`、可选 `safety.cleanup` | 命令风险门禁与审计提示 |
| `sources`、`generated` | 追溯信息；至少在 KB 详情和诊断中保留 |
| `verify` | 结构化来源，用于确认命令工具与安全语义一致 |
| `command` | 由 producer 从 `verify` 生成的 canonical 命令字符串；无 `verify` 时两者均不存在 |

`command` 是**派生投影**，不是第二真相源。producer 的 `verify`/`safety` 仍是唯一编辑输入；构建器必须保证 `command` 与生成 Markdown 中的命令块逐字节一致。

consumer 对每个有命令的条目必须验证：

- `verify.tool` 与 `command` 的工具一致；
- `command` 只能是 Nuclei、Nmap NSE 或 MSF 的允许形状；
- `verify.code == true` 当且仅当 Nuclei 命令在 `nuclei` 后带单个 `-code`；
- `safety`、`status` 与 `verify` 符合 producer schema；
- 验证失败时保留可展示条目，但清空其命令并产生 degraded diagnostic。

JSON 条目的安全字段缺失/非法不得退回为 safe。

### 4.3 command 允许形状

AnchorScan 只接受 producer 已构造且本地复核通过的命令：

- Nuclei：`nuclei [-code] -t <template> -u {{url}}` 或 `nuclei [-code] -t <template> -u {{host}}:{{port}}`；`-code` 只允许与 `verify.code: true` 配对。
- Nmap NSE：`nmap [-sU] -p {{port}} --script <script> [--script-args <args>] {{host}}`。
- MSF：四行 `use <module>`、`set RHOSTS {{host}}`、`set RPORT {{port}}`、`run|check`，并复用现有模块/动作白名单。

AnchorScan 不根据 JSON 中的 template、flags 或 args 重新拼接上述字符串；它只用受限 parser 读取 `command`，在运行时替换目标占位符，并再次检查没有残留占位符。

## 5. AnchorScan 模型和加载设计

### 5.1 领域模型

`knowledgebase.Entry` 新增但不混淆 catalog 可用性状态的字段：

- `ReviewStatus`：`stable` / `needs-review` / `legacy-unknown`；
- `Safety`：`Mode`（safe / optional / manual-gated / legacy-unknown）、`Effects`、`Cleanup`；
- `Sources` 与 `Generated`：producer 的审计数据；
- 现有 `Commands` 保留为 canonical command 的工具分发结果。

`Catalog.Status` 继续表达 loader 状态（disabled / unavailable / degraded / ready），不可与条目 `ReviewStatus` 混用。

### 5.2 JSON loader

在 `internal/knowledgebase/` 引入 catalog v2 JSON decoder。`Load` 负责路径解析、读文件和基于 `.json` 的分派；其余后缀继续进入 Markdown loader。新增公开 `LoadJSON` 只用于清晰的单元测试，调用方仍使用 `Load`。

诊断规则：

| 情形 | 结果 |
|---|---|
| 读文件失败、JSON 无效、顶层协议无效、重复 ID、零可展示条目 | `StatusUnavailable` |
| 单条缺展示字段、severity/status/safety 不合法、缺固定 sections | `StatusDegraded`，跳过该条目 |
| 单条 `verify` / `command` 不一致或命令不合法 | `StatusDegraded`，保留文本与匹配键但移除命令 |
| 正常条目 | `StatusReady`；任一条目诊断使 catalog 为 `StatusDegraded` |

### 5.3 Markdown 兼容

旧 Markdown loader 继续读现有 anchorscan-catalog v1 格式，并允许在三个固定章节之间存在其他四级知识小节；只要求“漏洞描述 → 验证命令 → 修复建议”各出现一次且顺序正确。

Markdown 没有 v3 `safety`、`status` 和来源声明，加载后必须标记为：

```text
ReviewStatus = legacy-unknown
Safety.Mode = legacy-unknown
```

它的命令可以显示、匹配与报告 enrich，但任何 Web/CLI 命令暴露需走 legacy 明示确认，不能继承 safe 默认值。

## 6. 命令绑定与安全门禁

### 6.1 运行时命令绑定

Nuclei binder 和批量 Nuclei builder 必须接受可选 `-code`，保留其位置和参数序列；Nmap/MSF 继续使用已有严格绑定规则。批量命令必须将带 `-code` 的参数视为同一 canonical 形状的一部分。

`report`、项目工作台、单漏洞命令、批量命令和会预填 `/tools/{tool}` 的路径必须经同一门禁入口。首先以代码图/引用搜索列举 `Entry.Commands` 的全部外部输出点；新增出口必须复用该入口。

### 6.2 服务器判定规则

| 条目条件 | 返回/预填命令前的要求 |
|---|---|
| `stable + safe` | 正常返回 |
| `needs-review + safe` | 显示待复核来源状态，并提交服务器校验的显式 acknowledgement |
| `optional` | 展示 `authentication-attempt`，要求 explicit confirmation |
| `manual-gated` | 展示所有 effects、cleanup（如有），要求 explicit confirmation |
| `legacy-unknown` | 以不低于 manual-gated 的确认强度处理，并说明旧手册没有安全声明 |
| safety/command 缺失或不合法 | 不返回命令；文本条目仍可浏览 |

确认字段必须由服务器根据当前 catalog 条目重新计算并校验，不接受客户端传来的 mode/effects/cleanup 作为事实。该确认是每次命令请求的显式操作者意图，不是可复用的权限凭据。

前端只负责解释风险、展示 cleanup 和提交确认。禁止通过隐藏按钮、前端条件或直接构造 query 参数绕过服务端。

## 7. 分发、配置和迁移

发布归档**不包含** `catalog.json`（2026-08-05 设计反转）；默认 `knowledge_base.path` 为空，知识库禁用且诊断明确，不指向任何包内文件。操作者按 README/配置页指引 clone 知识库仓库（Pentest-Playbook）并把 `knowledge_base.path` 指向其 `handbook-v3/dist/catalog.json`（相对路径相对配置文件目录解析，示例见 `config/default.yaml.example`）。外部绝对/相对 JSON 路径仍可配置；失效外部路径显示 unavailable diagnostic，不静默回退到另一份知识库。

迁移阶段：

1. Playbook 发布 catalog v2 协议、schema、产物和生成器测试。
2. AnchorScan 实现 v2 loader、legacy 映射、canonical command 验证与 `-code` 绑定。
3. AnchorScan 在所有外部命令路径落实服务端 safety/status gate。
4. 单源默认发行：移除包内 catalog 副本，默认禁用 + 文档指引 clone 与手动配置路径（2026-08-05 返工）。
5. Playbook 决定 v2 Markdown 退役后，另行评估删除 Markdown loader。

## 8. 验收与测试策略

测试遵循 `docs/testing-strategy.md` 的最低充分 seam：

1. **Playbook producer contract**：schema、生成器、`--check`；真实 `-code`、optional、manual-gated、cleanup、needs-review、无 verify 条目均被覆盖，catalog command 与 Markdown 命令块一致。
2. **knowledgebase unit**：catalog v2 顶层及每类条目诊断、`command`/`verify` 一致性、`-code`、legacy Markdown、扩展小节、分派与重复 ID。
3. **runtime unit**：单条和批量 Nuclei `-code` 参数绑定；拒绝非法或残留占位符；既有 Nmap/MSF 行为不回归。
4. **HTTP handler**：每种 safety/status/legacy 组合的未确认拒绝、确认成功和 invalid-command 拒绝；测试直接访问 handler，不以 UI 代替。
5. **Playwright smoke**：一个 safe 正常流、一个 optional 或 manual-gated 确认流、一个 needs-review 流，断言风险内容、确认动作和未确认时不产生 tool link。
6. **package/config**：归档**不包含** catalog，默认 `knowledge_base.path` 为空且知识库禁用、诊断清晰；外部 JSON 路径配置后仍可覆盖加载（单源模式主路径）。

AnchorScan fixture 必须内嵌，不在测试运行时读取 Playbook 工作区；fixture 中记录 catalog protocol version、producer artifact checksum 和生成日期。同步 fixture 的变更必须同时说明 producer schema/生成器的对应变更。

最终检查：受影响包聚焦测试、`make test`、`go vet ./...`、`make pr-check`。Playwright 仅在 UI gate ticket 后运行；真实扫描器不因本特性而自动运行。

## 9. 风险与控制

| 风险 | 控制 |
|---|---|
| producer/consumer 命令规则漂移 | v2 canonical command 投影 + 双端 schema/fixture/protocol 测试；Anchor 不重写 renderer |
| 新安全语义被旧 consumer 静默忽略 | catalog v2 显式版本门控，JSON safety 缺失 fail closed |
| 前端绕过门禁 | 所有命令输出点复用服务器 gate，HTTP 直接测试 |
| legacy Markdown 被误判为 safe | 显式 `legacy-unknown`，至少 manual-gated 强度确认 |
| needs-review 条目被误当稳定事实 | 保留状态并要求 acknowledgement；不隐藏其审计状态 |
| 默认配置指向不存在的外部文件 | 默认 `knowledge_base.path` 留空不指向任何文件；配置外部路径后由 unavailable 诊断与 package smoke 验证（2026-08-05 返工：发行物不再内嵌 catalog） |

## 10. 实施状态

本 feature 的 tickets 位于 `docs/plans/catalog-json-knowledgebase/tickets/`，已全部完成（01–05 均为 `done`）：catalog v2 协议与 producer 契约（01）、v2 模型与双格式 loader（02）、canonical command 与 `-code` 绑定（03）、服务端五档门禁（04）、默认发行、文档与迁移验收（05）。

2026-08-05 设计反转（单源模式）：ticket 05 返工后重新验收，发行物不再携带 catalog 副本；测试 fixture（`testdata/catalog-v2-real.json`）保留并锁定 producer artifact checksum（commit 57d739e，SHA-256 `7d8ce203…`），仅用于测试、不进发行物。运行时不依赖 Pentest-Playbook 工作区，只读取操作者显式配置的克隆仓库路径。
