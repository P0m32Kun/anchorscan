# 02 — 用 harness 锁定交付自治契约

**What to build:** 扩展 AI workflow harness 和 fixture 测试，阻止非递归交付与持续自治规则回退。

**Blocked by:** 01 — 明确非递归交付与持续自治。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`。

## 测试 seam

Node `scripts/check_ai_workflow.mjs --root <fixture>` 是唯一测试 seam。fixture 删除关键契约后
必须失败；不调用网络、不读用户目录、不依赖 runtime state。

## 实施

1. 先在 fixture 测试中加入会失败的 `merged_at`、非递归 delivery、持续自治和外部升级断言。
2. 以最小静态锚点扩展 checker，使新增测试转绿。
3. 运行聚焦测试、`make harness-check`、`make pr-check`，再完成双轴审查。

## 验收

- [ ] completed task 缺 commit/PR 仍失败，缺 `merged_at` 不失败。
- [ ] 删除 delivery PR 闭环或持续自治声明会使 checker 失败。
- [ ] 删除上游/全局/npm 显式授权边界会使 checker 失败。
- [ ] fixture 测试、harness-check 和 pr-check 均通过。

## 非目标

- 不新增远端 API 调用或 CI secret。
