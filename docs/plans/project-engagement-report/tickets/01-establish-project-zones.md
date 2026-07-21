# 01 — 建立任务型 Project 与 Network Zones

**What to build:** 让新 Project 只保存测试任务/报告元数据，并默认创建 I、II、III Zones；旧扫描默认字段保持可读但不再出现在新建/编辑 UI。

**Blocked by:** None — first frontier after spec approval.

**Status:** done

- [x] 为 project report metadata 与 project_zones 增加向前兼容迁移和 store 行为。
- [x] 新建 Project 自动创建 I、II、III，允许添加其他 Zone 并稳定排序。
- [x] Project 表单移除目标、端口、排除项和扫描档位，增加被测单位、报告标题、测试对象、日期、测试人员。
- [x] 有 Runs/Verifications 的 Zone 不可删除。
- [x] 旧 Project 仍能打开，不丢失旧默认字段。
- [x] 以 store 与 Web HTTP seam 完成一个 red-green 垂直切片。
