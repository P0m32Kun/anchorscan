# 12 — 删除确认无引用的代码与资产

**What to build:** 在全仓引用和测试证据支持下删除无行为价值的代码、参数与资产，不进行顺手架构重写。

**Blocked by:** 11 — 收敛文档与 Agent 执行契约。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`（仅行为变化）、`code-review`、`ponytail`。

## 候选清单

- 空文件 `internal/app/report.go`。
- 无引用 `config/ports-top100.txt`。
- 无调用 `internal/report.WriteProjectHTML` 及专用 import。
- `internal/ports.Resolve` 未使用的 `presetDir` 和无效目录重试。
- 重复的 `parsePort` / `parsePortNumber`。
- 只接收 `*sql.Rows` 的匿名 rows 接口。
- CONTEXT 中不影响领域定义的过程史。

## 测试 seam

- 删除前使用 CodeGraph、LSP references 和文本检索三者中的适用证据确认无生产调用。
- 端口解析复用现有 public unit seam；纯空文件/无引用删除不新增 tautological test。

## 验收

- [ ] 每个删除项在实施时重新确认引用；出现新调用即从 ticket 移除而不是强删。
- [ ] Resolve 签名简化后所有调用者编译，端口行为测试不变。
- [ ] 不删除 `ports-top1000.txt`、`ports-highrisk.txt`、`tools.Runner`、`app.Progress` 或 DOCX sidecar。
- [ ] 不按行数拆 `style.css`/Workbench，不批量清空 archive。
- [ ] `make test`、`go vet ./...`、`make pr-check` 通过。
- [ ] 双轴 review 无阻断项后将本计划归档。

## 非目标

- 不追求固定删行指标，不替换当前有实际用途的依赖。
