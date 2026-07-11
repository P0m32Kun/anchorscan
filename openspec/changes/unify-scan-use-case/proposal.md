## Why

CLI 与 Web 当前分别完成配置和规则加载、目标与端口解析、扫描档位覆盖、预检参数构造以及 `app.ScanOptions` 组装。同一业务入口存在两套实现，修改一侧时容易产生扫描计划和错误行为漂移，也让后续扫描能力迭代必须重复修改。

## What Changes

- 在现有应用层建立一个共享的扫描准备用例，将原始扫描请求标准化为可执行扫描计划及预检结果。
- CLI 与 Web 仅保留协议适配职责：采集各自输入、提供入口特有默认值、调用共享用例并呈现结果。
- 收敛扫描准备过程中工具路径、额外参数和预检数据的所有权，消除 `preflight` 对应用层类型的反向依赖，但不新增通用 `common` 包或推测性抽象。
- 用共享契约测试固定 CLI 与 Web 对等输入的标准化结果和失败语义。
- 严格保持 CLI、Web、数据库、报告以及扫描执行行为兼容。

## Capabilities

### New Capabilities

- `shared-scan-preparation`: 规定所有扫描入口必须通过同一准备边界生成标准化扫描计划，并保持入口间一致的校验和预检语义。

### Modified Capabilities

- 无。现有用户可见需求不变，本次新增的是约束既有行为一致性的内部架构能力。

## Impact

- 主要影响 `cmd/anchorscan`、`internal/web`、`internal/app`，以及必要的 `internal/config`、`internal/ports`、`internal/target`、`internal/preflight` 类型归属和相关测试。
- 不修改 CLI 参数或输出、Web 路由或表单、SQLite schema、JSON/HTML 格式、工具执行顺序和运行时语义。
- 不新增生产依赖、单实现接口、Repository/Service 层或面向未来的扩展框架。
