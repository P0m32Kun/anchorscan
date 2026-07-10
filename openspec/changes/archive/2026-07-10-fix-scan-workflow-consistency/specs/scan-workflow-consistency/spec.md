## ADDED Requirements

### Requirement: 端口输入格式收敛
系统 MUST 仅接受 `top1000`、`full` 或逗号分隔的 1-65535 数字端口作为扫描端口输入，并 MUST 拒绝 `top100`、`highrisk`、范围和非法端口。

#### Scenario: 项目模板使用 top1000 与排除端口
- **WHEN** 项目默认端口为 `top1000` 且配置了数字排除端口
- **THEN** 系统先解析 `top1000` 为数字 CSV，再应用排除端口并启动扫描

#### Scenario: 插入高危端口
- **WHEN** 用户点击高危端口快速插入按钮
- **THEN** 表单写入高危端口的数字 CSV，而不是 `highrisk` 关键字

#### Scenario: 拒绝已移除格式
- **WHEN** 用户提交 `top100`、`highrisk` 或 `100-1000`
- **THEN** 系统返回清晰的端口格式校验错误且不启动扫描

### Requirement: run 文件统一存储
系统 MUST 将一次 Web 扫描的 JSON 报告和所有扫描过程文件保存到同一个 run 目录。

#### Scenario: 项目扫描生成文件
- **WHEN** 项目扫描创建 run
- **THEN** `report.json`、RustScan、Nmap、HTTPX、NSE 和 nuclei 输出均位于该 run 的 artifact directory

### Requirement: 扫描历史显示关联项目
系统 MUST 在扫描历史中显示 run 关联的项目名称，并在存在项目时提供项目链接。

#### Scenario: 项目扫描出现在历史中
- **WHEN** 带 `project_id` 的 run 出现在扫描历史
- **THEN** 历史行显示对应项目名称并链接到项目详情

### Requirement: 验证器按服务类型路由
系统 MUST 使用 NSE 检查 Redis、MySQL/MariaDB、SMB 和 SSH 等非 Web 协议服务，并 MUST 使用 nuclei 检查 HTTP/Tomcat/Nginx 等 Web 服务，避免同一服务在自动流水线中重复路由到两类验证器。

#### Scenario: 非 Web 服务路由
- **WHEN** 指纹归一化为 redis、mysql、smb 或 ssh
- **THEN** 自动流水线匹配 NSE 脚本且不匹配 nuclei tags

#### Scenario: Web 服务路由
- **WHEN** HTTP 指纹或 Web 技术匹配 Tomcat 或 Nginx
- **THEN** 自动流水线匹配 nuclei tags 且不匹配 NSE 脚本
