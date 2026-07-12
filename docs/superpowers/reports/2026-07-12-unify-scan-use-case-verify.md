# unify-scan-use-case 验证报告

日期：2026-07-12  
变更：`unify-scan-use-case`  
验证级别：完整验证

## 摘要

| 维度 | 结果 | 证据 |
| --- | --- | --- |
| 完整性 | 通过 | OpenSpec 10/10 任务已勾选；1 个新增 capability 已实现。 |
| 正确性 | 通过 | CLI 与 Web 均通过 `app.PrepareScan` 生成扫描计划；预检、项目排除、错误优先级和入口生命周期均有回归测试。 |
| 一致性 | 通过 | 实现符合设计文档的包归属、依赖方向和入口职责；未新增依赖或第二条完整准备路径。 |

## 需求与实现映射

| Delta requirement | 实现证据 | 测试/验证证据 |
| --- | --- | --- |
| 共享同一准备边界 | `internal/app/scan_prepare.go`；CLI 与 Web 分别在 `cmd/anchorscan/main.go`、`internal/web/server.go` 调用 | `internal/app/scan_prepare_test.go` 的对等计划和错误顺序测试；全量测试通过。 |
| Web 项目规则保持语义 | `internal/target/parse.go` 的精确排除、`internal/ports/resolve.go` 的端口排除 | 领域包测试与共享准备契约测试覆盖排除、预检端口和执行端口分离。 |
| 预检结构化且阻止启动 | `PreparedScan.Preflight`；CLI/Web 按各自协议处理 | CLI 预检阻断测试、Web 400/无 run 测试与最终全量测试。 |
| 外部契约和依赖稳定 | 入口仅保留协议适配与运行时启动；`preflight` 依赖 `config` 而非 `app` | `go.mod`、`go.sum` 与前端锁文件无差异；`make test`、`make package` 通过。 |

## 设计一致性

- `config.ToolPaths` 承担工具路径类型；`app` 仅保留兼容类型别名，消除了 `preflight -> app` 反向依赖。
- `target` 与 `ports` 分别承担项目排除规则；未创建额外的通用包、接口或依赖。
- 唯一生产 `preflight.Run` 位于 `internal/app/scan_prepare.go`；CLI 与 Web 的完整扫描准备均委托 `app.PrepareScan`。
- Web 仍处理 HTTP、项目读取、默认值、托管路径、Manager 与响应。坏配置优先于缺失或不存在项目的既有错误顺序已由两条 HTTP 回归分支固定。
- 手动工具路径保持独立，未被纳入本次完整扫描准备收敛。

## 执行验证

| 命令 | 结果 |
| --- | --- |
| `rtk go test ./...` | 通过：202 项测试、14 个 Go 包。 |
| `make test` | 通过：Go 测试与前端 `internal/web/static/app.test.mjs`。 |
| `make package` | 通过：生成 darwin/arm64 打包产物。 |
| `openspec validate unify-scan-use-case --strict` | 通过。 |
| `git diff --check` | 通过。 |
| 依赖差异检查 | 通过：`go.mod`、`go.sum`、前端锁文件无改动。 |
| 生产路径检索 | 通过：仅 `internal/app/scan_prepare.go` 调用 `preflight.Run`；CLI 与 Web 均调用 `PrepareScan`。 |
| 凭据检索 | 通过：变更的生产文件未发现硬编码 API key、secret、password 或 token。 |

## 审查结论

- 标准任务级审查全部通过。
- 最终审查首轮发现 Web 在坏配置与项目错误并存时改变错误优先级；已由 `8f9fbaf` 修复。
- 复审要求补齐不存在项目 ID 的组合测试；已由 `5e4b7a5` 补齐并通过最终复审。
- 最终复审无 Critical、Important 或 Minor 发现。

## 结论

无 CRITICAL、WARNING 或 SUGGESTION。实现、OpenSpec delta spec 和技术设计一致，已通过验证；分支处理状态待用户选择。
