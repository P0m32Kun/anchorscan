# 01 — 保护 main 并暂停自动 bookkeeping commit

**What to build:** 为 `main` 建立不可绕过的 GitHub 合并门禁，并在本地严格 gate 完成前停止 Trellis archive/journal 自动提交。

**Blocked by:** None — can start immediately.

**Status:** done

**Execution skills:** `implement`、`code-review`。

## 行为契约

- `main` 只接受 PR 合并，要求 `quality-gate`；当前单协作者例外的必需审批数为 0。
- 直接 push、force push、删除被拒绝。
- `.trellis/config.yaml` 显式关闭 `session_auto_commit`；archive/journal 仍写文件但不自动产生 Git commit。
- 设置变更前后都有可重复的 GitHub API 读取证据。

## 实施

1. 记录现有 ruleset 与 branch protection 的只读输出。
2. 向用户展示将要写入的 GitHub 规则并取得一次明确确认；此步骤需要仓库管理员权限，不能隐式执行。
3. 创建/更新 ruleset，绑定 `main`，要求 PR、`quality-gate`、approval，并禁用 force push/delete。
4. 显式写入 `session_auto_commit: false`。
5. 用非破坏性 API 查询和一个临时分支/PR 验证规则。

## 验收

- [x] GitHub API 能读取生效的 main ruleset：`19911787`，`Require PR quality gate on main`。
- [x] ruleset 包含 PR、`quality-gate`、禁止 force push/delete；必需审批数为单协作者例外的 0。
- [x] `.trellis/config.yaml` 显式为 `session_auto_commit: false`（PR #2，merge commit `0f107087`）。
- [x] `trellis update --dry-run` 未覆盖该用户选择。

`make harness-check` 的 ruleset 配置检查是 Ticket 06 的独立验收项，不阻塞本 ticket 的完成。

## 非目标

- 不在此 ticket 增加签名、SBOM、release attestation 或组织级规则。
