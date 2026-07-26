# 09 — 强化发布与 CI 门禁

**What to build:** 让 release 验证实际解包归档并生成校验和，同时加入最低成本、可复现的依赖与 workflow 供应链检查。

**Blocked by:** 08 — 改善发现策略、进度与取消。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- Release smoke 使用最终 `.tar.gz`，不只运行仓库源码或 staging 文件。
- 归档 smoke 检查版本、配置 loader、端口预设、DOCX sidecar 和最小启动行为。
- Release 为每个归档生成 SHA-256 checksum。
- PR 执行确定性测试；真实工具实验室继续用于定时/release，并记录 commit、日期和工具版本。
- GitHub Actions 和实验室镜像采用可审计固定版本；依赖检查不依赖新的 task runner。

## 测试 seam

- Packaging integration：真实归档。
- Workflow static/command smoke：固定 fixture 校验 job wiring。
- Existing lab：只验证真实扫描器协作。

## 验收

- [ ] 先写归档缺资源或版本不匹配时 release smoke 失败的测试。
- [ ] Release workflow 在发布前执行 package smoke。
- [ ] 生成并上传 checksum。
- [ ] 加入 `govulncheck`、npm 高危审计和 Python 锁文件检查的稳定入口。
- [ ] Actions 固定 SHA，lab 镜像至少固定受控版本；变更有维护说明。
- [ ] `make pr-check` 和 workflow 校验通过。

## 非目标

- SBOM、签名和 provenance attestation 留作后续独立需求。
