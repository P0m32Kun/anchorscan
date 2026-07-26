# 06 — 记录 Run provenance 与制品完整性

**What to build:** 让每次 Run 能追溯 AnchorScan、扫描器、模板、规则、Scope、脱敏参数和 Artifact 哈希。

**Blocked by:** 05 — 让默认自动探测保持 safe。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- Run manifest 记录 AnchorScan/工具版本、nuclei template 标识、规则哈希、Scope snapshot 和执行时间。
- Artifact 保存 SHA-256；报告与 Store 读取相同历史事实，不按当前规则反推。
- 已知 secret 参数、URL userinfo 和凭据值在面向客户输出中脱敏。
- DetectionCheck 继续表示执行事实，报告不得输出漏洞覆盖率百分比或安全保证。
- README/报告只描述实际执行的引擎，不声称每个服务固定双引擎。

## 测试 seam

- App aggregation unit/integration：fake version provider、固定规则与 Artifact。
- Store migration/integration：manifest 和 hash 往返。
- Report unit：历史事实、脱敏与兼容输出。

## 验收

- [ ] 先写相同 Run 在规则文件变化后仍保留原 provenance 的失败测试。
- [ ] 为 Artifact 增加独立已知 SHA-256 断言。
- [ ] 报告展示必要 provenance 和实际 DetectionCheck，不暴露 raw secret。
- [ ] 旧数据库 migration 后仍可读取历史 Run。
- [ ] JSON 新字段保持向后兼容。
- [ ] 聚焦测试、`make test`、`go vet ./...`、报告 smoke 通过。

## 非目标

- 不签名 Artifact，不增加密钥管理或远程证明服务。
