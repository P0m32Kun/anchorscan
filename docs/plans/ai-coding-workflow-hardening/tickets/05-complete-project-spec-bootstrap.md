# 05 — 完成项目 spec bootstrap

**What to build:** 将 backend/frontend Trellis spec 从模板补全为 AnchorScan 的实际编码与质量约束。

**Blocked by:** 04 — 对齐 TDD、独立评审、workflow 与代理。

**Status:** ready-for-agent

**Execution skills:** `implement`、`code-review`。

## 行为契约

- backend/frontend index 都包含 Pre-Development Checklist 与 Quality Check。
- quality guideline 不含 `(To be filled by the team)` 或等价模板内容。
- 规范引用实际项目文档、命令和代码样例，不发明抽象或理想流程。
- `00-bootstrap-guidelines` 只在所有清单实际完成后归档。

## 实施

1. 从 `AGENTS.md`、`CONTEXT.md`、ADR、testing strategy、Makefile 和代表性代码提炼真实约束。
2. 填充 backend 的目录、数据库、错误、日志、质量规范。
3. 填充 frontend 的组件、hook、状态、类型、质量规范。
4. 更新 index checklist 并链接具体指南。
5. 完成 bootstrap task 的真实验收，不伪造历史证据。

## 验收

- [ ] 两个 index 均有可执行 checklist。
- [ ] 质量文档引用 `make test`、`go vet ./...`、`make pr-check` 和适用的 `make e2e`。
- [ ] 规范清楚规定 TDD、双轴 review 与最低充分 seam。
- [ ] 每项关键规则至少有一个真实代码/文档引用。
- [ ] bootstrap task 的未勾选项归零后才归档。

## 非目标

- 不把所有历史经验复制进 spec；只记录会影响未来实现/评审的持久契约。
