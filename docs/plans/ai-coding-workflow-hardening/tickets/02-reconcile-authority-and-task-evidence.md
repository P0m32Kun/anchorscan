# 02 — 对齐权威计划并定义任务证据

**What to build:** 消除 `docs/plans/` 与 Trellis 的状态分叉，定义 source-of-truth 和质量证据的最小持久格式。

**Blocked by:** 01 — 保护 main 并暂停自动 bookkeeping commit。

**Status:** done

**Execution skills:** `implement`、`code-review`。

## 行为契约

- 达梦默认口令检测的权威 spec/ticket/实现状态一致；不再显示为未排期 backlog。
- 新 Trellis task 可通过 `task.json.meta.source_of_truth` 指向唯一 spec/ticket。
- 行为任务的 `quality-evidence.json` 有稳定 schema，包含批准、TDD、验证、评审与交付引用。
- 不修改当前活动的 release 修复 Trellis task。

## 实施

1. 对照达梦 spec、已归档 Trellis task、提交和测试结果，更新权威 ticket/状态并写明尚未观察到的证据。
2. 修正达梦计划内 `12345` 端口保留/移除冲突，保持与实际产品配置一致。
3. 写入 task metadata 与 evidence schema 文档/fixture，不改 task lifecycle 行为。
4. 选择一个新建的 docs-only fixture task 验证 metadata 可被解析。

本 ticket 同时将已批准的工作流审查报告、spec、技术设计和全部依赖 ticket 纳入
`docs/plans/ai-coding-workflow-hardening/`。这是建立跨会话唯一权威来源的必要前置，
不是对后续实现的提前交付。

## 验收

- [x] 达梦计划状态、交付 ticket、归档 task 与实现提交 `e69b8cd` 已对齐。
- [x] 未观测的 review/TDD/PR/真实环境证据均明确标为 `unobserved`，未补写为已执行。
- [x] `source_of_truth` 与 `quality-evidence.json` v1 有规范文档、历史迁移样本和 docs-only fixture。
- [x] 当前 release 修复任务未被修改。

## 验证记录

- Fixed point：`0f107087`。
- `git diff --cached --check`：通过。
- `jq`：达梦迁移样本和 docs-only fixture 的全部 JSON 可解析。
- Node 元数据检查：两个 fixture 的 source ticket、spec 路径和 evidence v1 形状均通过。
- `node scripts/check_markdown_links.mjs`：通过。
- `task.py` 当前因既有 `.trellis/scripts/common/task_context.py` 语法错误不可运行；本 ticket
  未修改 lifecycle，fixture 使用独立 JSON 解析验证。Ticket 03 必须在严格 gate 实施前处理该基线错误。

## 非目标

- 不追溯重写所有历史 archive task。
- 不在本 ticket 强制 start/archive gate。
