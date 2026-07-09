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
