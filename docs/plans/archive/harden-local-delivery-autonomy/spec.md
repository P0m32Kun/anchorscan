# 本地交付自治与非递归收尾加固

**Status:** completed

## 背景

已合并的交付 PR 之后，Agent 曾把任务归档、会话日志或 `merged_at` 补录误解为新的
远端交付，导致出现纯元数据后续 PR，并反复向用户询问常规 push/PR/合并授权。
这既不能增加产品质量，也把已经完成的交付重新打开成递归流程。

## 目标

在 AnchorScan 仓库内，使任务的有效交付在同一个 delivery PR 中闭环；在用户已经授予
持续自治时，Agent 可自行完成常规 Git 交付操作。用本地 harness 防止 workflow、skill
和 Pi prompt 再次漂移。

## 已批准的范围

- 仅修改本仓库中的 `.trellis/`、`.agents/`、`.pi/`、`docs/` 与本地 harness。
- 不修改 Trellis 上游、用户全局 skill、npm 包或任何外部仓库设置。
- 用户的持续自治授权覆盖常规分支、提交、push、PR、合并、归档和会话记录。

## 行为契约

### 1. 单 PR 交付闭环

- complete gate 的最小交付证据是 `delivery.commit` 与 `delivery.pr`。
- `delivery.merged_at` 是可为空的观测字段，不是 complete、archive 或 journal 的前置条件。
- 任务证据、ticket 状态和归档必须在 delivery PR 合并前完成并随该 PR 交付；PR 的合并状态
  本身是远端观测证据，不得为补写该观测创建后续纯元数据 PR。
- 合并后只允许本地、无提交的观测记录；如确有新的产品/行为变更，必须建立独立 ticket。

### 2. 持续自治边界

- 在明确的持续自治授权下，创建分支、提交、push、创建/合并 PR、归档和 journal 都是常规
  交付步骤，不需要重复征询。
- 仍须向用户升级的事项仅限：产品行为或范围的实质选择、权限/安全风险、未知的并行工作
  冲突，以及 Trellis 上游、全局安装、npm 发布等外部持久变更。

### 3. 可执行回归保护

- `make harness-check` 必须拒绝把 `merged_at` 变为 complete gate 条件、恢复“合并后补
  元数据 PR”或恢复常规 Git 操作的一次性确认。
- `.trellis/workflow.md`、本地 Trellis skill 与 Pi prompt 必须表达同一自治边界。

## 非目标

- 不追溯清理历史纯元数据 PR。
- 不改动产品扫描、报告或发布行为。
- 不替用户处理无法归属的并行工作树改动。

## 实施顺序

1. 更新证据文档、workflow、local skill 和 Pi prompt，明确非递归交付与自治边界。
2. 先写失败 fixture/assertion，再扩展 `check_ai_workflow`，锁定上述契约。
3. 按 ticket 完成验证与独立双轴审查；只在发现新的实质行为需求时开启后续 ticket。

## 总体验收

- 已合并 delivery PR 不会因 `merged_at`、archive 或 journal 补录而要求新 PR。
- 持续自治的 Agent 不会为常规 Git 交付再次询问用户。
- 外部/全局操作和真正未知的并行改动仍被明确升级。
- `make harness-check` 和相关 fixture 测试可在离线本地 checkout 中验证契约。

## 执行规则

- 本目录的 ticket 是本工作的唯一权威；每次只实施一个已解除阻塞的 frontier ticket。
- 交付前记录 `origin/main` 作为 review fixed point，并按
  [`docs/agents/issue-tracker.md`](../../../agents/issue-tracker.md) 完成 TDD（适用时）、验证和双轴审查。
