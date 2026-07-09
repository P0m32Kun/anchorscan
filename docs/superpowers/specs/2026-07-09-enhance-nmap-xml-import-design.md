---
comet_change: enhance-nmap-xml-import
role: technical-design
canonical_spec: openspec
archived-with: 2026-07-09-enhance-nmap-xml-import
status: final
---

# Enhance Nmap XML Import 技术设计

## 背景

AnchorScan 现有扫描链路已经能通过 `rustscan -> nmap -sV -> httpx/NSE/nuclei` 生成 SQLite、JSON/HTML 报告和 Web Console 可读的数据，但 Nmap XML 解析目前只覆盖最小服务指纹字段。外部已有的 Nmap XML 还不能直接导入 AnchorScan，且同一主机的 `53/tcp` 与 `53/udp` 这类结果需要明确保留 protocol 才能避免报告语义混淆。

`/Users/kun/Downloads/nmap-viewer/` 的可吸收价值主要是离线 Nmap XML 导入、protocol/CPE/NSE scope 保留，以及较大 XML 的增量解析思路。本设计吸收这些能力，不引入 Python/FastAPI、独立 SQLite 数据库或另一套前端。

## 目标

- 新增 `anchorscan import-nmap`，把已有 Nmap XML 导入为完成态 AnchorScan run。
- 在现有 Go 架构内增强 Nmap XML 解析和导入编排。
- 保留 `ip + port + protocol` 服务身份、CPE、NSE script 输出和 script scope。
- 复用现有 SQLite、JSON/HTML 报告和 Web Console 查看链路。
- 对空文件、非法 XML、非 `nmaprun` XML 和落库失败给出清晰错误，并避免写入半截 run。

## 非目标

- 不引入 Python/FastAPI 或 `nmap-viewer/static/` UI。
- 不复制或读取 `nmap_viewer.db`。
- 不一次性迁移 `risk_rules.json` 完整风险规则引擎。
- 不在本 change 内增加 Web Console 上传入口。
- 不建立完整可查询的 NSE structured output 模型。

## 架构方案

采用轻量增强方案：导入能力沿用现有 AnchorScan 模块边界，只补必要字段和导入编排。

- `cmd/anchorscan` 负责 `import-nmap` CLI 参数解析、帮助文本和错误输出。
- `internal/app` 新增导入编排：校验输入、调用解析器、开启事务写入、生成报告。
- `internal/fingerprint` 扩展 Nmap XML 解析结构，输出服务、CPE、script 输出和 scope。
- `internal/store` 增加向后兼容 migration，并提供导入所需的保存方法。
- `internal/report` 输出 `protocol`，并让导入 findings 继续进入现有 JSON/HTML 报告。

导入命令的用户路径是：

```text
anchorscan import-nmap --xml sample.xml --db data/scans.sqlite --json reports/import.json --html reports/import.html
  -> 校验 XML 文件
  -> 解析 Nmap XML
  -> 创建完成态 scan_run
  -> 保存 fingerprints 和 findings
  -> 写出 JSON/HTML 报告
```

## 数据模型

`fingerprints` 继续作为服务资产的主表，但需要补足导入需要的最小字段。

- `protocol` 必须持久化，并参与服务身份判断。
- `cpe` 使用字符串字段保存一个或多个 Nmap `<cpe>` 值，先不建 CPE 关系表。
- `ServiceFingerprint` 增加 `CPE string`，并确保 `Protocol`、`ExtraInfo`、`Tunnel` 在导入和报告路径中保留。
- JSON/HTML 报告输出 `protocol`，让 `53/tcp` 和 `53/udp` 在用户视角中明确区分。

NSE script 输出复用 `findings` 承载可报告内容。

- port-level script 转为带 `IP`、`Port`、`Protocol` 语义的 finding。
- hostscript、prescript、postscript 转为无端口或 `port=0` 的 finding/原始输出。
- 非端口级 script 的 `Source` 或 `ID` 必须标明 scope，例如 `nmap-import:hostscript:ssh-hostkey`。
- structured output 先合并进 `Output`，后续如需可查询结构化 NSE，再单独设计新 capability。

## 错误处理

导入过程按事务式处理：输入校验和 XML 解析先完成，再开始写入 run 数据；落库中任一步失败都回滚，不留下半截 run。

错误分层如下：

- 文件不存在或不可读：CLI 返回文件读取错误。
- 空文件：返回 `empty XML file`。
- XML 语法错误：返回 `invalid Nmap XML` 并包含解析器错误。
- 根节点不是 `nmaprun`：返回 `root element is not nmaprun`。
- 落库或报告写出失败：返回对应底层错误，事务回滚已写入数据。

## 测试策略

测试使用一个最小 Nmap XML fixture，内容包含：

- 同一主机的 `53/tcp` 与 `53/udp`。
- service 下的 CPE。
- port-level `<script id="http-methods">`。
- hostscript。
- postscript。

测试层次：

- parser 单元测试验证字段、protocol、CPE、script scope 和错误输入。
- store 测试验证 `53/tcp` 与 `53/udp` 持久化后仍是两条服务。
- CLI/import 集成测试使用临时 SQLite，运行 `anchorscan import-nmap --xml fixture --db temp.sqlite --json temp.json --html temp.html`，检查 run 创建、报告文件存在、两条 53 服务都能读出。
- 失败路径测试空 XML 和非 `nmaprun` XML，确认命令失败且数据库无新增 run。

## 迁移策略

SQLite migration 必须向后兼容旧库：

- 新字段使用默认值，旧 run 继续可读。
- 旧查询路径不依赖新字段时行为保持不变。
- 回滚时可以停止使用 `import-nmap`；新增列不影响旧数据读取。

## 后续能力

这个 change 刻意不把风险规则和 Web 上传一起做进去。后续如果需要，可以基于本次导入编排继续增加：

- Web Console 上传/导入 Nmap XML。
- `risk_rules.json` 到 AnchorScan 规则包的迁移。
- 可查询的 NSE structured output 存储和筛选。

