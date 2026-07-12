---
comet_change: unify-scan-use-case
role: technical-design
canonical_spec: openspec
archived-with: 2026-07-12-unify-scan-use-case
status: final
---

# 统一扫描准备用例技术设计

## 1. 设计目的

CLI 的 `runScan` 与 Web 的 `scanCreate` 当前分别编排配置、目标、端口、Profile、规则、预检和 `ScanOptions`。底层函数大多已经共享，真正重复的是步骤顺序和字段组装，因此本设计只收敛这一条用例，不重写扫描运行时，也不建立新的服务层。

OpenSpec delta spec 是需求事实源；本文只描述实现边界、数据流、兼容策略、风险和测试方法。

## 2. 目标与非目标

### 2.1 目标

- CLI 和 Web 的完整扫描入口都调用同一个 `app.PrepareScan`。
- 工具路径、工具额外参数、规则和最终执行字段只组装一次。
- 普通解析错误与结构化预检结果保持两个失败通道。
- 解除 `preflight -> app` 的反向类型依赖。
- 保持现有 CLI、Web、数据库、报告和扫描运行时行为。

### 2.2 非目标

- 不合并 CLI 与 HTTP 协议。
- 不修改 `RunScan`、`Manager`、Store、报告生成或工具执行顺序。
- 不修复 Web 预检端口不一致、303/落库竞态、CLI run ID 冲突或目标语义校验。
- 不引入 Builder、Factory、DI、Repository/Service 接口、feature flag 或新生产依赖。
- 不保留新旧两条扫描准备路径。

## 3. 目标架构

```text
CLI flags                         Web form + project
    |                                      |
    | 入口特有默认值与协议校验             | 项目读取与默认值选择
    +------------------+-------------------+
                       |
                       v
             internal/app.PrepareScan
                       |
          +------------+-------------+
          |            |             |
        config       target         ports
          |                            |
          +---------- preflight -------+
                       |
                       v
              PreparedScan
              - ScanOptions
              - preflight.Result
                 |             |
                 v             v
           CLI RunScan    Web Manager.Start
```

依赖方向固定为：

```text
cmd/anchorscan ----\
                    -> app -> config / target / ports / preflight
internal/web ------/               |
                                    -> config / ports

app -> tools / store / report      （现有运行时依赖）
```

`preflight` 不再导入 `app`，因此 `app` 可以安全编排 `preflight`。

## 4. 文件职责

| 文件 | 职责 |
| --- | --- |
| `internal/config/config.go` | 配置结构；将匿名工具路径结构命名为 `ToolPaths`，保留已有 `ToolArgs` |
| `internal/preflight/preflight.go` | 预检诊断；使用 `config.ToolPaths` 和 `config.ToolArgs` |
| `internal/target/parse.go` | 目标解析、去重和精确排除 |
| `internal/ports/resolve.go` | 端口解析、展开、排除和压缩 |
| `internal/app/scan_prepare.go` | 唯一扫描准备用例、请求/结果类型和步骤编排 |
| `internal/app/scan.go` | `ScanOptions`、类型别名和扫描运行时；不接收准备逻辑 |
| `cmd/anchorscan/main.go` | CLI 参数、输出路径、Store、同步执行、日志和 HTML 后处理 |
| `internal/web/server.go` | HTTP、项目读取、托管路径、Manager 启动和响应 |

不创建 `internal/scanprep`、`internal/common` 或仅包含类型的额外包。

## 5. 类型归属

`config.Config.Tools` 当前是匿名结构，而 `config.ToolArgs` 已存在并与运行时额外参数同构。最小调整是：

```go
package config

type ToolPaths struct {
    Rustscan string `yaml:"rustscan"`
    Nmap     string `yaml:"nmap"`
    Httpx    string `yaml:"httpx"`
    Nuclei   string `yaml:"nuclei"`
}

type ToolArgs struct {
    Rustscan []string `yaml:"rustscan_args"`
    Nmap     []string `yaml:"nmap_args"`
    Httpx    []string `yaml:"httpx_args"`
    Nuclei   []string `yaml:"nuclei_args"`
}

type Config struct {
    Tools ToolPaths `yaml:"tools"`
    // 其余字段不变
}
```

`app` 在 `ScanOptions` 附近保留兼容别名：

```go
type ToolPaths = config.ToolPaths
type ToolExtraArgs = config.ToolArgs
```

这样可以同时满足：

- `preflight` 直接依赖低层 `config`，不再依赖 `app`。
- `ToolRunOptions` 和现有调用方无需一次性改名。
- 不复制第二套等价工具类型。
- 不让 `config` 反向依赖执行层 `tools`。

## 6. PrepareScan API

建议的具体值结构如下；实现阶段允许调整字段排列，不改变职责：

```go
type PrepareScanRequest struct {
    ConfigPath string

    TargetSpec     string
    PortSpec       string
    ExcludeTargets string
    ExcludePorts   string
    Overrides      config.Overrides

    DBPath         string
    JSONReportPath string
    ArtifactRoot   string

    RunID          string
    ProjectID      string
    Logf           func(format string, args ...any)
}

type PreparedScan struct {
    Options   ScanOptions
    Preflight preflight.Result
}

func PrepareScan(req PrepareScanRequest) (PreparedScan, error)
```

字段边界：

- `ConfigPath`、`DBPath` 和 `JSONReportPath` 是预检必需输入。
- `RunID` 可为空。CLI 为保持现有时间调用顺序，在预检成功后再填入返回的 `Options`；Web 继续在调用前生成 run ID。
- `ProjectID` 保持入口当前传入值，不在本 change 顺手规范化。
- `HTMLPath` 不进入请求，因为它只属于 CLI 扫描后的 HTML 后处理。
- `store.Project` 不进入应用层请求。Web 先选择项目默认值，再传递普通字符串和排除串。

## 7. 准备算法

`PrepareScan` 严格按以下顺序执行：

1. 调用 `config.Load(req.ConfigPath)`。保留配置缺失时自动初始化文件的副作用。
2. 调用 `target.Parse(req.TargetSpec)`，再调用目标排除 helper。目标仍只做分隔、trim、去重和精确排除，不新增 IP/CIDR 校验。
3. 选择端口串：只有 `req.PortSpec == ""` 时才使用 `cfg.Scan.Ports`，不提前 trim，保持现有空值语义。
4. 调用 `ports.ResolveForConfig` 得到已解析端口，再调用端口排除 helper 得到最终执行端口。
5. 调用 `config.ResolveScan(cfg, req.Overrides)` 得到 Profile、Workers 和工具参数。
6. 依次调用 `LoadNSERulesForConfig` 和 `LoadTagRulesForConfig`。继续保持配置同目录优先、仓库 `config/` 兜底、首个成功文件整份生效。
7. 只构造一次 `toolPaths` 和 `extraArgs`。
8. 调用 `preflight.Run`：
   - Targets 使用排除后的目标。
   - PortSpec 使用排除前的已选端口串。
   - Tools、Profile、Workers、ExtraArgs 和规则数量使用最终共享值。
   - ConfigDir、ReportDir 从已有路径派生。
9. 若预检存在错误，返回完整 `Preflight`、零值 `Options` 和 nil error。
10. 若预检通过或只有 warning，组装并返回完整 `ScanOptions`；`Ports` 使用解析并排除后的最终端口。

`PrepareScan` 不打开 Store、不调用 `RunScan` 或 `Manager.Start`、不写日志、不生成 HTTP 响应。

## 8. 目标与端口排除

### 8.1 目标排除

将 Web 当前的精确字符串排除移动到 `target` 包。行为保持：

- 排除串继续通过 `target.Parse` 解析。
- 以精确字符串匹配，不展开 CIDR 或 IP range。
- 保留原目标顺序。
- 不存在排除项时原样返回。

### 8.2 端口排除

将 Web 当前的端口展开、排除和压缩逻辑移动到 `ports` 包。行为保持：

- `top1000` 只有在存在排除项时才读取当前 preset 文件并展开。
- 单一范围和纯 CSV 继续使用当前规则。
- 混合 CSV+range、`full` 和 `highrisk` 继续被现有解析逻辑拒绝。
- 排除后按当前压缩规则生成最终端口串。

这些 helper 是现有行为的归位，不建立新的端口表达式框架。

## 9. 返回与错误契约

| 情况 | error | Preflight | Options | 调用方行为 |
| --- | --- | --- | --- | --- |
| 配置、目标、端口、Profile、参数或规则错误 | 非 nil | 零值 | 零值 | 按入口现有协议呈现 |
| 预检存在 error | nil | 完整结果 | 零值 | 必须停止，不得启动扫描 |
| 只有 warning 或全部通过 | nil | 完整结果 | 完整选项 | 可以启动扫描 |

入口映射保持：

- CLI：先调用 `logPreflight`；若 `HasErrors()`，返回现有 `preflight failed`。
- Web：若 `HasErrors()`，返回 HTTP 400 并用完整结果重绘项目扫描表单。
- Web 的 Manager 冲突仍映射为 HTTP 409。
- CLI 的 Store、目录和 HTML 错误继续按当前普通 error 返回。

统一函数采用 `config → target → ports → profile → rules → preflight` 的首错顺序。单个错误的文本和入口呈现不变；多个错误同时存在时，不再保留 CLI/Web 两套优先级。

## 10. 入口迁移

### 10.1 CLI

CLI 继续完成：

- flag 定义、帮助和 `--target` 精确空值检查。
- 默认 JSON 路径生成。
- 预检日志呈现。
- 预检成功后的目录确保、Store 打开和 run ID 生成。
- 同步 `RunScan`、可选 HTML 报告和 stdout 输出。

CLI 删除：

- 配置、规则、目标、端口和 Profile 的重复编排。
- 两套工具路径/额外参数字面量。
- 本地 `ScanOptions` 公共字段组装。

### 10.2 Web

Web 继续完成：

- HTTP 方法和表单解析。
- 项目查询与必填校验。
- 表单值、项目默认值的优先级选择。
- run ID、托管 report/artifact 路径。
- 预检通过后的托管 report 目录确保。
- 400 表单重绘、409 冲突和 303 重定向。
- `Manager.Start` 异步启动。

Web 删除：

- 配置之后的目标、端口、Profile、规则和预检编排。
- 两套工具路径/额外参数字面量。
- 迁移到 `target`/`ports` 的旧排除 helper。

## 11. 兼容性约束

实现必须保持：

- CLI 参数、帮助、stdout/stderr、退出语义和 HTML 后处理。
- Web 路由、方法、状态码、表单字段、模板数据和重定向。
- SQLite schema、迁移、run/project 关联和现有数据。
- JSON/HTML 格式、工具参数、扫描阶段、并发、取消、事件和 artifact 路径。
- `go.mod`、`go.sum` 和前端依赖不变。

允许的唯一错误顺序变化是：同一请求同时包含多个准备错误时，CLI 与 Web 使用统一首错顺序。

## 12. 测试策略

### 12.1 共享准备契约

先创建 `internal/app/scan_prepare_test.go`，以表驱动方式覆盖：

- 配置端口、Profile、Workers 和工具参数默认值。
- 显式 Profile、Workers 和四类工具参数覆盖。
- NSE/tag 规则加载与当前 fallback 语义。
- 目标去重、目标排除、端口排除和 `top1000`。
- 普通错误的统一顺序。
- 必需工具错误、可选工具 warning、目录预检和规则数量。
- 预检失败时 `Options` 为零值。
- 预检通过时 `ScanOptions` 所有公共字段正确。
- 语义相同的 CLI/Web 请求生成相同公共扫描计划。

测试使用 `t.TempDir()`、临时 YAML、临时规则文件和带执行位的临时工具文件，不新增 loader、filesystem 或 runner mock 接口。

### 12.2 领域 helper

- `target` 测试固定精确排除、顺序和空排除。
- `ports` 测试固定 `top1000` 展开、CSV/range 排除、全部排除及错误格式。

### 12.3 入口特征测试

- CLI：帮助、artifact root、预检摘要、预检阻断、Profile/参数传递、JSON/HTML、stdout/stderr、run ID 生成时机。
- Web：项目必填、默认值、排除项、400 表单、端口格式、409、303、托管路径和扫描未启动条件。

### 12.4 最终验证

```text
go test ./...
make package
现有 CLI/Web 扫描冒烟检查
确认 go.mod、go.sum 和前端依赖无变化
```

## 13. 迁移顺序

1. 先写共享准备、目标排除和端口排除的失败测试。
2. 命名 `config.ToolPaths`，复用 `config.ToolArgs`，增加 `app` 类型别名并迁移 `preflight`。
3. 将 Web 排除 helper 原样迁入 `target` 和 `ports`，保持测试通过。
4. 实现 `app.PrepareScan`，只让新契约测试通过。
5. 迁移 CLI，运行 CLI、app、config、ports、target、preflight 测试。
6. 迁移 Web，运行 Web 和 Manager 测试。
7. 删除重复组装与旧 helper，确认 CLI/Web 不存在第二条完整准备路径。
8. 运行完整验证。

不使用运行开关或双写。失败时整体回退该 change。

## 14. 风险与延期

| 风险 | 控制措施 |
| --- | --- |
| 类型移动影响单工具运行 | app 类型别名；运行现有 ToolRun 测试 |
| 统一顺序改变复合错误首错 | delta spec 明确允许；增加顺序测试 |
| 预检副作用时机漂移 | 保留 `preflight.Run` 原调用阶段并做入口特征测试 |
| Web 默认值被应用层感知 | 项目读取与值选择继续留在 Web |
| 规则加载被误改为合并 | 使用现有 loader，不修改规则实现 |
| 迁移后残留第二条路径 | 删除旧 helper/字面量并做调用路径检查 |

延期项不属于本设计的验收范围：

- Web 预检端口与最终端口不一致。
- Web 303 后运行记录尚未落库。
- CLI 秒级 run ID 冲突及 JSON/run ID 跨秒。
- 目标字符串缺少 IP/CIDR 语义校验。
- 后台扫描在保存 run 前失败时缺少可观察记录。

这些问题需要独立 change/hotfix，避免在无行为变更重构中混入修复。
