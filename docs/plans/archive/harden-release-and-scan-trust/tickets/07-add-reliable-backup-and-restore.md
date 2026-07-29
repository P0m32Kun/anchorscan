# 07 — 提供可靠备份与恢复

**What to build:** 创建包含一致 SQLite snapshot、Evidence、必要配置和完整性 manifest 的本地备份，并在恢复前验证内容。

**Blocked by:** 06 — 记录 Run provenance 与制品完整性。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 活动 Run Lease 存在时备份明确拒绝，不产生看似成功的不一致归档。
- SQLite 使用受支持的一致 snapshot，不依赖在线复制单个 WAL 主文件。
- 备份包含项目 Evidence、必要 config 和相对路径 manifest；Artifacts 由显式选项控制。
- 恢复先在临时目录校验路径、大小和 SHA-256，再替换目标数据。
- 归档路径不得越出受管目录或通过 symlink 注入任意文件。

## 测试 seam

- Store/integration：WAL 中存在未 checkpoint 数据的 snapshot。
- CLI command：backup/verify/restore 使用临时数据根。
- Project report integration：恢复后重新导出含 Evidence 的报告。

## 验收

- [x] 先证明只复制主 DB 会遗漏状态，并为新 backup 行为写失败测试。
- [x] 带 Evidence 的项目恢复后可生成相同聚合内容。
- [x] 缺文件、哈希不一致、路径穿越和活动 lease 都明确失败。
- [x] 更新 deploy 的备份/升级步骤。
- [x] 聚焦测试、`make test`、`go vet ./...` 通过。

## 非目标

- 不实现增量、加密、云端、在线连续备份或跨版本自动降级。
