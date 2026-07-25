# 06 — 交付负向验证候选与结论

**What to build:** 从双引擎 completed 且无非 info Finding 的指纹端点派生待负向验证队列，并把缺失/失败检查分流到无法判定。

**Blocked by:** 03 — Project 聚合读模型；04 — Verification 与 Evidence。

**Status:** done

- [x] 覆盖 DetectionCheck completed/info-only/non-info/missing/failed/skipped/canceled/interrupted 矩阵。
- [x] 候选只说明可人工验证，不自动创建 not_observed。
- [x] 用户必须选择明确验证项，可一次勾选多个同类候选端点，保存一张共享截图并显式提交单条本次验证未发现；该 Verification 关联全部选中端点。
- [x] 同一类验证项只要求一份截图证据，不要求每个 IP 单独截图。
- [x] 旧甘肃 Runs 因无 DetectionCheck 进入历史事实不可用，不生成负向候选。
- [x] inconclusive 不进入正式已存在或未发现统计。
