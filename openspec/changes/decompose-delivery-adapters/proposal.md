## Why（为什么）

CLI 与 Web 交付层目前把命令分派、HTTP 路由、业务输入适配、报告查询和视图数据组装集中在少数大文件中，导致任一职责变化都扩大阅读、测试和回归范围。完成共享扫描用例与扫描运行时拆分后，应按真实修改原因重组交付适配器，使后续改动能落在明确边界内，同时严格保持现有外部契约。

## What Changes（变更内容）

- 按现有 CLI 命令拆分 `cmd/anchorscan/main.go`，保留一个薄入口负责启动和分派，命令实现各自持有参数解析、校验和输出适配职责。
- 按 Web 资源和职责拆分 `internal/web/server.go`，集中保留服务器装配与路由注册，把项目、扫描、工具、报告等处理逻辑放到对应适配器文件。
- 将报告筛选、分页、导出和视图数据组装拆成边界清晰的现有 `internal/web` 组件，避免 Handler 同时承担查询规则和展示转换。
- 按相同职责重组测试，使每组测试与其交付适配器相邻且可独立运行。
- 保持 CLI 参数、帮助文本、输出和退出行为，以及 Web 路由、状态码、表单字段、模板数据和 DOM 行为不变。
- 保持数据库 Schema、Migration、已有数据、JSON/HTML 报告格式和扫描行为不变。
- 该变更依赖 `unify-scan-use-case` 与 `decompose-scan-runtime` 先完成，以已有共享用例和运行时边界为拆分基础。
- 不新增路由框架、Service/Repository 抽象或第三方依赖；不按文件行数机械拆分。

## Capabilities（能力）

### New Capabilities（新增能力）

- `delivery-adapter-modularity`：规定 CLI、Web 与报告交付适配器的职责边界、兼容约束和可独立验证要求。

### Modified Capabilities（修改能力）

- 无。

## Impact（影响）

- 主要影响 `cmd/anchorscan`、`internal/web` 及其测试文件；必要时仅调整这些适配器引用既有应用层入口的方式。
- 不改变公开 CLI/HTTP 契约、数据库与报告格式，也不改变扫描流程、并发、取消、事件或产物。
- 不增加生产或开发依赖；继续使用 Go 标准库、现有模板和现有测试工具。
