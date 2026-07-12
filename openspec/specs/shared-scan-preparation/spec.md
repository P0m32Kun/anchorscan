# shared-scan-preparation Specification

## Purpose
TBD - created by archiving change unify-scan-use-case. Update Purpose after archive.
## Requirements
### Requirement: 扫描入口共享同一准备边界
系统 MUST 让 CLI 与 Web 的完整扫描入口通过同一个应用层准备边界完成配置与规则加载、目标和端口规范化、扫描档位解析、预检以及执行选项构造，不得在两个入口分别维护等价流程。

#### Scenario: 对等输入生成一致扫描计划
- **WHEN** CLI 与 Web 使用相同配置，并提交语义相同的目标、端口、Profile、Workers 和工具额外参数
- **THEN** 两个入口生成的目标、端口、工具路径、Profile、Workers、额外参数、NSE 规则和 tag 规则 MUST 一致，入口特有的 run ID、项目 ID 和输出路径除外

#### Scenario: 配置默认值一致生效
- **WHEN** 某入口未覆盖端口、Profile、Workers 或工具额外参数
- **THEN** 共享准备边界 MUST 使用同一份配置解析规则补全对应扫描计划字段

#### Scenario: 复合无效输入采用统一首错顺序
- **WHEN** 一个扫描请求同时包含多个准备阶段错误
- **THEN** 共享准备边界 MUST 按同一固定顺序返回首个普通错误，且每个单项错误的消息以及 CLI 与 Web 的呈现协议 MUST 保持不变

### Requirement: 项目扫描规则在共享准备中保持语义
系统 MUST 在 Web 入口选择好表单值与项目默认值之后，通过共享准备边界应用项目目标和端口排除规则；CLI 未提供排除项时 MUST 保持现有行为。

#### Scenario: Web 项目默认值和排除项
- **WHEN** Web 扫描使用项目默认目标或端口，并配置了目标或端口排除项
- **THEN** 最终扫描计划 MUST 与当前先选择默认值、再解析并应用排除项的结果一致

#### Scenario: 端口排除保持当前预检与执行字段
- **WHEN** 项目端口排除规则改变了最终执行端口
- **THEN** 预检摘要 MUST 继续使用排除前的已选端口串，而执行选项 MUST 使用解析并排除后的最终端口串

#### Scenario: CLI 不应用项目规则
- **WHEN** CLI 发起普通扫描且没有项目排除输入
- **THEN** 共享准备边界 MUST 保留全部已解析目标和端口

### Requirement: 预检诊断保持结构化且阻止无效扫描
系统 MUST 从共享准备边界返回完整的预检摘要、警告和错误；存在预检错误时 CLI 与 Web 均不得启动扫描，并 MUST 按各自现有协议呈现诊断。

#### Scenario: CLI 预检失败
- **WHEN** CLI 扫描准备产生一个或多个预检错误
- **THEN** CLI MUST 继续输出现有预检日志并返回现有 `preflight failed` 失败语义，且不得调用扫描运行时

#### Scenario: Web 预检失败
- **WHEN** Web 扫描准备产生一个或多个预检错误
- **THEN** Web MUST 返回当前使用的 HTTP 400，并用完整预检结果重绘项目扫描表单，且不得启动 Manager run

### Requirement: 共享准备重构严格保持外部契约
系统 MUST 保持现有 CLI 参数、帮助与输出，Web 路由、状态码、表单与模板数据，数据库 schema，JSON/HTML 报告格式，以及扫描工具顺序、并发、取消、事件和产物行为。

#### Scenario: 成功扫描经共享准备后执行
- **WHEN** 有效的 CLI 或 Web 请求通过共享准备并进入现有扫描运行时
- **THEN** 相同请求产生的工具参数、运行阶段、事件、持久化记录和报告内容 MUST 与重构前一致

#### Scenario: 不新增运行依赖
- **WHEN** 该 change 完成
- **THEN** 项目生产依赖列表 MUST 保持不变，且实现不得要求新的服务、框架或构建步骤
