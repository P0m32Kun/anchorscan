# 04 — 持久化 Verification 与 Evidence

**What to build:** 保存人工验证结论、报告字段快照、关联资产/来源 Runs 与有序截图 Evidence，并提供受控上传/读取/排序/删除行为。

**Blocked by:** 01 — 建立任务型 Project 与 Network Zones；03 — 构建 Project 聚合读模型。

**Status:** draft

- [ ] 建立 report_verifications、verification_assets、verification_sources、verification_evidence。
- [ ] outcome 只允许 confirmed/not_observed/inconclusive。
- [ ] Evidence 文件位于 Project 受管理目录，SQLite 不存 BLOB。
- [ ] 只接受经过签名和尺寸验证的 PNG/JPEG，限制请求大小并生成服务端文件名。
- [ ] 上传失败不留下数据库记录或孤儿临时文件；删除不触碰 Artifact。
- [ ] confirmed/not_observed 缺少 Evidence 时保持未完成状态。
