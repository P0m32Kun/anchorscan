# 10 — 显式归并甘肃历史 Runs

**What to build:** 在新模型稳定后，用默认 dry-run、显式参数、`--apply` 前强制备份的一次性本地命令，把现有 4 个甘肃 Projects 的 Runs 归并到一个任务 Project 并分配 I/III Zones；不建设通用 Web 合并 UI。

**Blocked by:** 02 — Zoned Runs；03 — Project 聚合；08 — Project HTML。

**Status:** done

- [x] 默认只输出预览；`--apply` 前强制备份 `data/scans.sqlite` 和对应 Project 数据目录。
- [x] 由用户显式选择来源 Runs、目标 Project 和 Zone，不按名称自动归并。
- [x] 预览 ProjectID/ZoneID/include_in_report、Artifact 路径和受管理目录变化。
- [x] I区三个旧 Runs 与 III区一个旧 Run 按用户确认结果迁移；II区保持空。
- [x] 迁移后 Project 聚合数量与迁移前四个 Runs 的事实总量一致。
- [x] 不为缺失 DetectionCheck 的旧 Runs 伪造负向验证。
- [x] 旧 Project 仅在为空且用户再次确认后删除。
