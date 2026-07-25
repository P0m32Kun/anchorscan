# 09 — 扩展真实工具实验室并以证据修正规则

**What to build:** 扩展 build-tag/Docker E2E 到 MySQL、SMB、SSH、未知 TCP 和混合目标，使用真实工具验证 DetectionCheck 与部分失败语义，并让发布流程留下通过记录。

**Blocked by:** `07-deliver-detection-coverage-reports.md`。

**Status:** done

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

- [x] Docker 实验室可重复启动 MySQL、SMB、SSH、未知 TCP 服务和混合目标场景。
- [x] 真实 rustscan、nmap、httpx、nuclei 测试验证 Fingerprint、DetectionCheck、报告覆盖和局部失败保留。
- [x] workflow 支持人工触发和定期运行，不进入每个 PR 的硬门禁。
- [x] 发布 workflow 在制品生成前运行同一实验室 job，并保存提交、日期、工具版本、日志和结果。
- [x] 只修改实验室实际复现缺口对应的 `service-tags.yaml`、`nse.yaml` 或相关稳定规则。
- [x] 删除未被读取且重复代码内稳定别名的 `service-aliases.yaml` 及误导文档，不实现配置加载器。
- [x] 测试失败能区分环境问题、工具失败、规则跳过和产品回归。
