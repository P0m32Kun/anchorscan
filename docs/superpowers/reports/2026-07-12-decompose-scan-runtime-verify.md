# 扫描运行时职责拆分验证报告

## 结论

`decompose-scan-runtime` 已通过完整验证，可以进入归档前的分支处理决策。

## 验证摘要

| 维度 | 结果 | 证据 |
| --- | --- | --- |
| 完整性 | 通过 | OpenSpec `tasks.md` 为 12/12 完成，Superpowers plan 为 30/30 完成。 |
| 正确性 | 通过 | `RunScan` 保持稳定入口；生命周期、调度和单目标流水线均按规格拆分，组合与边界测试覆盖完成、失败、取消、并发和工具顺序。 |
| 一致性 | 通过 | 实现与 OpenSpec design、技术设计及 proposal 一致；没有新增依赖、导出入口、子包或动态流水线。 |
| 审查 | 通过 | 独立只读审查未发现 CRITICAL、IMPORTANT 或 MINOR 的正确性、安全或边界问题。 |

## 规格与实现对照

- [稳定入口] `internal/app/scan.go` 继续保留 `RunScan` 作为唯一完整扫描入口；CLI 和 Manager 调用方未变化。
- [生命周期] `RunScan` 仍拥有 artifact 目录、run 状态写入与 defer、最终 JSON 报告提交；`scan_lifecycle_test.go` 覆盖完成和报告写入失败。
- [调度] `internal/app/scan_targets.go` 保留存活探测、worker 上下限、无缓冲 `targetCh`、结果关闭顺序、取消优先级、部分失败事件和全部失败首错包装。
- [单目标流水线] `internal/app/scan_target.go` 保留 RustScan、Nmap、HTTPX、NSE、Nuclei、事件、心跳和 artifact 写入的既有顺序与条件。
- [兼容性] `go.mod`、`go.sum`、`cmd/anchorscan/main.go` 与 `internal/app/manager.go` 相对计划基线没有变化；未新增生产依赖或公开 API。

## 新鲜验证证据

执行日期：2026-07-12。

| 命令 | 结果 |
| --- | --- |
| `rtk make test` | 通过：`go test ./...` 全仓通过，Web Node 测试 1/1 通过。 |
| `rtk go test -race ./internal/app` | 通过：50 项应用层测试通过，未报告 data race。 |
| `rtk go vet ./internal/app` | 通过。 |
| `VERSION=v1.7.1-plan-check rtk make package` | 通过：生成 Darwin ARM64 二进制与 `.tar.gz`，并已执行 `rtk make clean` 清理产物。 |
| `rtk git diff --check` | 通过，无空白错误。 |

## 已知工作流限制

Comet 的通用 build 守卫仅自动识别 npm、Maven 和 Cargo 项目，不能识别本仓库的 Go/Makefile 构建，因此会在没有这些清单文件时误报 build 失败。本次已先用 `rtk make build` 和上述 package 命令取得实际构建证据，再使用守卫提供的 `COMET_SKIP_BUILD=1` 仅跳过其不适用的自动推断；这不跳过项目测试、竞态、静态检查或打包验证。

## 问题

- CRITICAL：无。
- WARNING：无。
- SUGGESTION：无。
