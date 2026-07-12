# Brainstorm Summary

- Change: unify-scan-use-case
- Date: 2026-07-11

## 确认的技术方案

- 在 internal/app 新增单一具体入口 PrepareScan(request) (PreparedScan, error)，配套普通请求值和结果值；不新增接口、Factory、DI、scanprep 或 common 包。
- 统一准备顺序为：加载配置 → 目标解析/排除 → 端口默认值、解析/排除 → Profile/工具参数解析 → NSE/tag 规则加载 → 预检 → 组装 ScanOptions。
- 请求复用现有 config.Overrides，并携带配置路径、目标/端口及排除串、DB/JSON/artifact 路径，以及可选的 run/project 元数据、配置快照和 Logf。
- 结果只包含 ScanOptions 和结构化 preflight.Result。普通解析错误返回 error 和零值结果；预检错误返回完整预检结果、nil error 和零值 ScanOptions；预检通过或只有 warning 时返回完整可执行选项。
- CLI 继续负责 flag/帮助、--target 必填、默认输出路径、Store、同步 RunScan、日志和 HTML 后处理。为保持现有时序，CLI 在预检成功后生成 run ID 并填入返回选项。
- Web 继续负责项目查询、表单值与项目默认值选择、托管路径、HTTP 状态、Manager.Start 和重定向；项目排除串传给共享准备边界。
- PrepareScan 不打开 Store、不启动扫描、不写 HTTP、不打印日志。工具路径和额外参数只构造一次，同一值同时用于预检和最终扫描。
- 目标排除归入现有 target 包，端口展开、排除和压缩归入现有 ports 包。
- 将配置中的匿名工具路径结构命名为 config.ToolPaths，复用现有 config.ToolArgs；app 保留 ToolPaths/ToolExtraArgs 类型别名，preflight 直接依赖 config，解除 preflight -> app。
- 不保留 feature flag、新旧双路径或入口顺序模式；迁移完成后删除重复字段映射和 Web 旧排除 helper。

## 关键取舍与风险

- 选择一个统一错误顺序，不为 CLI/Web 保留两套分支。每种单独错误的文本、CLI/Web 协议及所有有效请求行为保持不变；多个字段同时无效时，首个错误优先级允许统一。
- 预检继续使用排除前的已选端口串，最终 ScanOptions.Ports 使用解析并排除后的端口串，以保持当前 Web 行为；本 change 不顺手修复该不一致。
- preflight.Run 当前会通过 MkdirAll 创建目录，保留其副作用和时机，不为文件系统新增接口或 mock 层。
- 类型别名控制 ToolRunOptions、扫描运行时和现有调用方的编译影响；迁移期间不复制等价工具类型。
- 规则继续保持“配置同目录优先、仓库 config/ 兜底、首个成功文件整份生效”，不改为合并规则。
- 明确延期：Web 预检端口不一致、303/落库竞态、CLI 秒级 run ID 冲突、无效目标语义校验、后台早期错误可观察性等另立 change/hotfix。

## 测试策略

- 先新增 internal/app/scan_prepare_test.go 的表驱动契约测试，再写实现；使用 t.TempDir()、临时配置和临时可执行文件，不新增 mock 框架。
- 覆盖配置默认值/覆盖、规则加载、目标/端口排除、Profile/workers/工具参数、普通错误顺序、预检 warning/error、预检失败时 Options 为零值，以及成功时的完整 ScanOptions。
- CLI 特征测试继续固定帮助、stdout/stderr、预检日志、preflight failed、同步执行、run ID 时机、JSON/HTML 和未启动条件。
- Web 特征测试继续固定项目必填/default/exclusion、400 表单重绘、409 冲突、303 跳转、托管路径和未启动条件。
- 迁移顺序：契约测试 → 工具值类型与别名 → target/ports 排除 helper → app.PrepareScan → CLI → Web → 删除旧路径。
- 每步保持 go test ./... 通过；最终运行现有 make package 和扫描冒烟检查，并确认依赖文件未变化。

## Spec Patch

- 在 delta spec 中增加复合无效输入场景：共享边界采用统一首错顺序；单项错误消息及 CLI/Web 呈现协议保持不变。
- 在 delta spec 中增加项目端口排除边界：预检摘要继续使用排除前的已选端口串，执行选项使用解析并排除后的最终端口串。
