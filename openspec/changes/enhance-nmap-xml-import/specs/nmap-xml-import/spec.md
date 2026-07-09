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
