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
