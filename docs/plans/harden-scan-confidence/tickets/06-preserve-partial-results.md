# 06 — 保留部分结果并收敛带错误运行

**What to build:** 改变扫描错误边界，使 httpx、NSE、nuclei 的局部失败不会终止独立工作或丢弃已取得事实，并完整实现 completed_with_errors/canceled/interrupted 的 DetectionCheck 终态。

**Blocked by:** `04-reconcile-interrupted-runs.md`、`05-record-detection-checks.md`。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] Fingerprint 在基础识别后立即保存，httpx 通过自然键更新增强字段；Finding、Artifact、DetectionCheck 在阶段边界增量保存。
- [x] httpx、单个 NSE 或 nuclei 的命令/解析/制品失败被记录并继续其他独立引擎、Fingerprint 和 Target。
- [x] 基础阶段失败只结束对应 Target；至少一个 Target 有有效结果且存在局部失败时 Run 为 `completed_with_errors`。
- [x] 全部基础 Target 失败或无法保存有效报告时 Run 为 `failed`。
- [x] 操作者取消把 Run 和仍 running 的检查收敛为 `canceled/run_canceled`；租约过期收敛为 `interrupted/lease_expired`。
- [x] completed_with_errors 提供与 interrupted 相同的预填确认重跑，不自动执行。
- [x] fake Runner + 临时 SQLite 测试证明先前事实保留、后续工作继续、状态计数不互相污染。
- [x] 不增加异步写队列、通用 repository 或事件总线。
