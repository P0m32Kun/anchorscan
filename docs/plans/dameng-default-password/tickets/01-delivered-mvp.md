# 01 — 达梦默认口令检测 MVP（已交付）

**Status:** done

**Spec:** [`../spec.md`](../spec.md)

## 交付引用

- 实现提交：`e69b8cd feat: detect Dameng DB default password with active protocol fingerprinting`。
- 执行记录：`.trellis/tasks/archive/2026-07/07-28-01-dameng-mvp/`。
- 产品状态：`internal/fingerprint/probes/dameng.go`、`internal/tools/dameng.go` 和
  `internal/app/scan_target.go` 均在当前 `main`。

## 已观察证据

- 达梦主动指纹、默认口令 verdict 和调度条件均有仓库内 Go 测试。
- 研究文档的达梦端口记录为 `5236`；高危端口表保留 `5236` 与无关的 `12345`。

## 未观察证据

- 原始用户批准、Red/Green 命令输出、独立 Standards/Spec 评审和 PR URL 未保存在归档 task。
- 未观察到真实达梦实例的端到端验证记录。

这些缺失只作为历史事实记录；不得补写为已执行。新行为任务从
[`docs/agents/task-evidence.md`](../../../agents/task-evidence.md) 的证据格式开始记录。
