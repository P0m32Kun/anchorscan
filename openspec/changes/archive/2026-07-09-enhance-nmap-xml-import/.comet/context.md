# Comet Design Handoff

- Change: enhance-nmap-xml-import
- Phase: design
- Mode: compact
- Context hash: 1763bb505da82e51162b2547483767f9b40ac82cec52acd0272122e7f0f42acf

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/enhance-nmap-xml-import/proposal.md

- Source: openspec/changes/enhance-nmap-xml-import/proposal.md
- Lines: 1-27
- SHA256: 7dd92a62b939473f903129df8b0e982dd907c471d1bfe3d3cc726c9ed4c36758

```md
## Why

AnchorScan 已能运行 Nmap 并解析服务指纹，但当前 XML 解析只覆盖最小字段，且缺少把外部已有 Nmap XML 导入现有数据库/报告链路的入口。`/Users/kun/Downloads/nmap-viewer/` 证明了离线 XML 查看和分析有实际价值；本变更吸收其中有用的解析与导入思路，但不引入 Python/FastAPI 或独立前端。

## What Changes

- 增强 Nmap XML 解析，保留协议维度，避免同一端口的 `tcp`/`udp` 结果互相混淆。
- 解析并持久化对现有资产和报告有直接价值的 Nmap 字段：服务协议、CPE、NSE 脚本输出及脚本作用域。
- 新增导入已有 Nmap XML 的 CLI 命令，将导入结果保存为一个 AnchorScan run，并复用现有 SQLite、JSON/HTML 报告和 Web Console 查看链路。
- 对非法 XML、空文件、非 Nmap XML 给出清晰错误，并避免写入半截数据。
- 不引入 Python 运行时、不复制 `nmap_viewer.db`、不迁移 `nmap-viewer/static/` UI、不一次性导入完整风险规则引擎。

## Capabilities

### New Capabilities
- `nmap-xml-import`: 导入已有 Nmap XML 文件并生成可由 AnchorScan 查看和报告的扫描运行。

### Modified Capabilities
- 无；当前仓库尚无已归档 OpenSpec capability，本次新增 capability 覆盖导入行为及相关解析要求。

## Impact

- 影响 CLI：新增 `anchorscan import-nmap` 命令及帮助文本。
- 影响 SQLite schema：扩展 fingerprints/findings 或新增轻量字段，以保存协议、CPE、NSE 脚本作用域等导入所需数据。
- 影响报告模型：JSON/HTML 输出需要区分 `port/protocol`，并展示导入来的服务和脚本结果。
- 影响解析模块：Nmap XML 解析从一次性最小结构解析升级为可处理更完整字段、较大 XML 和错误边界的实现。
- 影响测试：新增解析、导入命令、落库和报告回归测试。

```

## openspec/changes/enhance-nmap-xml-import/design.md

- Source: openspec/changes/enhance-nmap-xml-import/design.md
- Lines: 1-51
- SHA256: 489efbba6d2879ad40a78b4911ec52f6c3fe6fd7afc9043f87215156ffff6a30

```md
## Context

AnchorScan 当前扫描链路使用 `rustscan -> nmap -sV -> httpx/NSE/nuclei`，并通过 SQLite、JSON/HTML 报告和 Web Console 展示结果。现有 `internal/fingerprint/nmap_xml.go` 只用 `encoding/xml.Unmarshal` 解析最小服务指纹字段，`store.fingerprints` 与 `report.PortReport` 也未完整保留协议和 CPE；因此外部 Nmap XML 无法直接进入 AnchorScan，且 `53/tcp` 与 `53/udp` 这类同端口多协议结果在报告语义上不够稳。

`/Users/kun/Downloads/nmap-viewer/` 的价值主要在流式 XML 导入、协议/CPE/NSE scope 保留和离线查看；其 Python Web 服务、SQLite 数据库和前端不适合并入 Go 项目。

## Goals / Non-Goals

**Goals:**

- 增加 `anchorscan import-nmap`，把已有 Nmap XML 导入为 AnchorScan run。
- 增强解析器，保留 `port/protocol`、服务字段、CPE、NSE script output 和 script scope。
- 导入结果复用现有 SQLite、JSON/HTML 报告和 Web Console，不新增服务进程。
- 对非法、空、非 Nmap XML 输入返回清晰错误，并保证失败不落半截数据。

**Non-Goals:**

- 不引入 Python/FastAPI 或 `nmap-viewer` 的 `static/` UI。
- 不复制 `nmap_viewer.db`。
- 不一次性迁移 `risk_rules.json` 完整规则引擎。
- 不把导入功能扩展到 Web 上传入口；后续有需求再做。

## Decisions

1. **在 Go 内吸收解析逻辑，而不是嵌入 Python 应用。** 这样保持单二进制交付和现有 CLI/Web Console 形态。替代方案是把 `nmap-viewer` 作为子服务，但会引入双栈依赖和重复 UI。

2. **新增导入命令复用现有 run/store/report 模型。** `import-nmap` 创建一个完成态 scan run，保存 fingerprints/findings，再按需输出 JSON/HTML 报告。替代方案是新增独立 `nmap_imports` 表和页面，但当前需求只需要“看已有 XML”，复用现有链路更小。

3. **以 `ip + port + protocol` 作为端口身份。** schema 和报告需要保留 protocol，避免 TCP/UDP 同端口合并。替代方案是继续只用 `ip + port`，但会丢失 Nmap 的真实扫描结果。

4. **解析器支持较大 XML 的增量处理，但只持久化当前产品需要的字段。** Nmap host OS/MAC/runstats 可先解析到内部结构或后续扩展，不为未展示字段扩大 schema。若未来资产画像需要，再单独建 capability。

5. **NSE script 输出先落为 findings/output，structured output 存为字符串或合并到 output。** 这样现有报告能立即展示脚本结果。完整可查询脚本表或规则引擎后续再做。

## Risks / Trade-offs

- **Schema 迁移影响旧库** → 新字段使用默认值，迁移保持向后兼容，并补迁移测试。
- **报告模板未展示协议导致用户仍看不出 TCP/UDP** → JSON/HTML 模型和模板一起改，测试检查协议字段。
- **大 XML 流式解析更复杂** → 只实现 Nmap `host/ports/script` 必需路径，避免通用 XML 抽象。
- **导入的 NSE 结果不等同于主动运行 NSE** → finding source 标记为 `nmap-import` 或 `nse`，保留原始 output，避免误导。

## Migration Plan

1. 添加 SQLite migration，为现有表补协议/CPE/NSE 导入所需字段或最小附表。
2. 新导入命令只写新增 run；旧 run 和旧报告继续可读。
3. 回滚时可停止使用 `import-nmap`；旧库新增列不影响旧查询。

## Open Questions

- Web Console 上传入口是否需要作为后续 change 添加。
- 完整 `risk_rules.json` 是否需要迁移为 AnchorScan 风险规则包。

```

## openspec/changes/enhance-nmap-xml-import/tasks.md

- Source: openspec/changes/enhance-nmap-xml-import/tasks.md
- Lines: 1-23
- SHA256: 16a4084e9a87d5b344ae3ae54075b447d9baa0a7d41d3085fe40d12c6eca5819

```md
## 1. Parser and Model

- [ ] 1.1 补充解析测试，覆盖 TCP/UDP 同端口服务、CPE 值、port script、host script、postscript 输出、空输入和非 `nmaprun` XML。
- [ ] 1.2 扩展 Nmap XML 解析器，保留 protocol、服务字段、CPE 值、script 输出和 script scope，且不引入 Python viewer。
- [ ] 1.3 更新 fingerprint/report 数据模型，使 `port/protocol` 身份能在 JSON 和 HTML 报告输出中保留下来。

## 2. Persistence

- [ ] 2.1 为导入 run 所需的 protocol/CPE/script-scope 数据添加向后兼容的 SQLite migration。
- [ ] 2.2 更新 store 的保存/读取方法及测试，确保同一端口上的导入 TCP 和 UDP 服务保持独立。
- [ ] 2.3 将导入的 NSE script 输出持久化为可报告的 findings 或 script 衍生输出，并避免把 host/global script 归到错误端口。

## 3. Import Command

- [ ] 3.1 为 `anchorscan import-nmap` 增加 CLI 参数解析、帮助文本，以及 `--xml`、`--db`、可选 `--run-id`、`--project`、`--json`、`--html` 的校验。
- [ ] 3.2 实现导入编排：创建完成态 scan run，以事务方式导入解析数据，并按需写出报告。
- [ ] 3.3 确保非法 XML、空文件和非 Nmap XML 都会以清晰错误失败，且不会留下部分 run 数据。

## 4. Verification and Docs

- [ ] 4.1 增加 CLI/import 集成测试，使用包含 `53/tcp`、`53/udp`、CPE 和 NSE scripts 的最小 Nmap XML fixture。
- [ ] 4.2 更新 README 或命令帮助文档，说明导入流程和明确的非目标。
- [ ] 4.3 运行 `go test ./...`，并用该 fixture 实跑一次 `anchorscan import-nmap` 示例命令。

```

## openspec/changes/enhance-nmap-xml-import/specs/nmap-xml-import/spec.md

- Source: openspec/changes/enhance-nmap-xml-import/specs/nmap-xml-import/spec.md
- Lines: 1-56
- SHA256: 8c51d342464dc9e2efaa7319d146724dd8a020e899f504aa1e11f542f4f2e74b

```md
## ADDED Requirements

### Requirement: 导入已有 Nmap XML 为 AnchorScan run
系统 MUST 提供一个 CLI 命令，把合法的 Nmap XML 文件导入指定的 AnchorScan SQLite 数据库，并记录为一个已完成的 scan run。

#### Scenario: 成功导入 XML
- **WHEN** 用户运行 `anchorscan import-nmap --xml sample.xml --db data/scans.sqlite`
- **THEN** 系统创建一个已完成的 run，其中包含从 `sample.xml` 解析出的主机、开放端口、服务和 script 输出

#### Scenario: 拒绝空 XML 输入
- **WHEN** 用户导入一个空 XML 文件
- **THEN** 命令以清晰的校验错误失败，且不会创建 run

#### Scenario: 拒绝非 Nmap XML 输入
- **WHEN** 用户导入的 XML 根节点不是 `nmaprun`
- **THEN** 命令以清晰的校验错误失败，且不会创建 run

### Requirement: 保留端口协议身份
系统 MUST 把 protocol 作为每个导入服务身份的一部分保存下来，使同一个数字端口可以分别以 TCP 和 UDP 形式共存。

#### Scenario: 同一端口同时存在 TCP 和 UDP
- **WHEN** 导入的 Nmap XML 在同一主机上同时包含 `53/tcp` 和 `53/udp`
- **THEN** 两个服务都会被独立存储并作为不同条目出现在报告中

### Requirement: 保留服务增强字段
系统 MUST 保留 AnchorScan 报告所需的 Nmap 服务字段，包括 service name、product、version、extra info、tunnel，以及存在时的 CPE 值。

#### Scenario: 服务包含 CPE 值
- **WHEN** 导入端口的 `<service>` 下包含一个或多个 `<cpe>` 子节点
- **THEN** 导入后的服务记录会保留这些 CPE 值，用于报告或 finding 输出

### Requirement: 保留 NSE script 输出及作用域
系统 MUST 导入 port-level、host-level、prescript 和 postscript 中的 NSE script 输出，并保留足够的作用域信息以区分脚本来源。能自然映射为风险或线索的脚本输出 MUST 进入 findings；其余脚本输出 MUST 保留原始内容，且不得强行归属到错误端口。

#### Scenario: 导入 port script
- **WHEN** 导入端口中包含 `<script id="http-methods" output="...">` 元素
- **THEN** 生成的 run 中会包含与该主机、端口和协议关联的 finding 或 script 衍生输出

#### Scenario: 导入 host 或全局 script
- **WHEN** Nmap XML 包含 hostscript、prescript 或 postscript 的 script 输出
- **THEN** 生成的 run 会保留这些 script 输出，且不会把它们错误地归属到某个端口

#### Scenario: 非端口级 script 保留原始输出
- **WHEN** hostscript、prescript 或 postscript 的输出无法自然映射为某个端口 finding
- **THEN** 系统会保留原始 script 输出并标明 scope，而不会伪造端口归属

### Requirement: 导入 run 可生成现有报告
系统 MUST 允许导入的 Nmap XML run 复用现有的 JSON 和 HTML 报告生成路径。

#### Scenario: 导入时请求 JSON 报告
- **WHEN** 用户在导入时传入 `--json reports/import.json`
- **THEN** 系统使用导入的服务和 findings 写出 JSON 报告

#### Scenario: 导入时请求 HTML 报告
- **WHEN** 用户在导入时传入 `--html reports/import.html`
- **THEN** 系统使用导入的服务和 findings 写出 HTML 报告

```
