# Brainstorm Summary

- Change: enhance-nmap-xml-import
- Date: 2026-07-09

## 确认的技术方案

- 导入能力保持在 Go/AnchorScan 现有架构内实现，不引入 Python/FastAPI 或独立前端。
- 导入命令复用现有 SQLite、run、report 和 Web Console 查看链路。
- 采用方案 B：在现有链路上做轻量增强，补足 `protocol`、`cpe` 和 script scope 等关键结构，而不是新建一套独立导入模型。
- 模块边界确认：
  - `cmd/anchorscan` 负责 `import-nmap` CLI 参数解析和命令入口。
  - `internal/app` 负责导入编排：解析、事务落库、生成报告。
  - `internal/fingerprint` 负责 Nmap XML 解析和导入结构。
  - `internal/store` 负责 schema migration 与持久化。
  - `internal/report` 负责把 `port/protocol` 和导入结果带入 JSON/HTML。
- `hostscript`、`prescript`、`postscript` 默认采用混合策略：
  - 能自然映射为 finding 的脚本结果进入 findings 视图。
  - 其余脚本结果保留为原始输出，不强行转成 finding。
- 数据模型确认：
  - 不新增完整 `nmap_import_*` 表族，优先复用现有 `fingerprints`、`findings`、`scan_runs`、report 模型。
  - `fingerprints` 持久化 `protocol`，使 `ip + port + protocol` 成为导入服务身份。
  - `fingerprints` 增加轻量 `cpe` 字段，先以字符串保存 Nmap `<cpe>` 值。
  - `ServiceFingerprint` 同步增加 `CPE string`，报告层输出 `protocol`，并在详情或 fingerprint 输出中带出 CPE。
  - port-level NSE script 转为带 host/port/protocol 的 finding。
  - hostscript、prescript、postscript 转为无端口或 `port=0` 的 finding/原始输出，并在 `Source` 或 `ID` 中标明 scope，例如 `nmap-import:hostscript:ssh-hostkey`。
  - structured script output 先合并进 finding `Output`，后续如需可查询结构化 NSE 再单独开 change。
- 错误处理确认：
  - `import-nmap` 先校验文件存在、非空、XML 根节点为 `nmaprun`。
  - 导入写库采用事务式编排，解析或落库失败时不留下半截 run。
  - CLI 输出保持直接可理解，例如 empty XML file、root element is not nmaprun、invalid Nmap XML。

## 关键取舍与风险

- 需要在“尽量复用现有 findings 模型”和“不要误报/过度结构化脚本输出”之间平衡。
- 需要保留 `port/protocol` 身份，避免 `53/tcp` 与 `53/udp` 在导入后被合并。
- 需要控制 schema 扩张，只保存对导入和报告直接有价值的字段。
- 使用 `findings` 承载部分导入脚本输出会牺牲一点语义纯度，但能直接复用现有报告链路。

## 测试策略

- 解析测试覆盖 TCP/UDP 同端口、CPE、port script、host script、postscript、空 XML、非 `nmaprun` XML。
- 集成测试覆盖导入命令落库、生成 JSON/HTML 报告、失败时不落半截数据。
- CLI 集成测试使用临时 SQLite 和最小 XML fixture，检查 run 创建、两条 53 服务都在、JSON/HTML 报告文件存在。

## Spec Patch

- 已确认：在 delta spec 中补充非 port 级 script 的 finding/original-output 混合处理说明。
