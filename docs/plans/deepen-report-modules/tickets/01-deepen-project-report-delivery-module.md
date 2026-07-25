# 01 — 深化 Project Report 交付组装 module

**What to build:** 将 Project Report 的 Store/Evidence 读取、领域映射与交付验证从 Web helper 移入 `internal/app` 的单函数 module；HTML 与 DOCX handler 只保留 adapter 职责。

**Blocked by:** none.

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`update-spec`、`ponytail`。

- [x] 先以当前 `main` 记录 code-review fixed point。
- [x] 在 `internal/app` 的 interface seam 写失败测试，覆盖成功、Project 不存在、invalid Project Report 与 Evidence 不可读。
- [x] 实现 `app.BuildProjectDeliverable(*store.Store, projectID, now)`，直接使用 SQLite Store 与本地 Evidence 文件系统。
- [x] 引入最小错误分类，使 adapter 保持 404 / 400 / 500 映射，不建立错误层级。
- [x] HTML 与 DOCX adapter 改为消费同一 `ProjectDeliverable`；文件名从 `deliverable.Project.Name` 派生。
- [x] 删除 `internal/web.buildProjectDeliverable`，不保留 pass-through module。
- [x] 按 `error-inventory.md` 保持现有中文错误、触发条件和 DOCX 503 行为。
- [x] 保持 Evidence eager assembly、排序、Data URI、文件路径与 Network Zone 推断不变。
- [x] 运行聚焦测试、全量 Go 测试与静态检查。
- [x] 以 fixed point 和 `spec.md` 运行 Standards/Spec 双轴 code review，修正发现后将 ticket 标为 done。
