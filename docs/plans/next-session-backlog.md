# 跨会话后续工作清单

> 最后更新：2026-07-29
>
> 用途：记录已规划但尚未交付的工作顺序及前置条件。开始任何一项前，先检查对应 Trellis task、`origin/main` 与此清单是否一致。

## 已确认产品交付

- Spark Web UI/API 的安全 tags 路由已通过 PR #13 合并：<https://github.com/P0m32Kun/anchorscan/pull/13>

## 任务跟踪说明

- Spark 任务的归档、merge evidence 与 developer journal 由当前维护 PR 同步到 `main`。
- 默认 Nuclei 精确模板规则任务仍为 `in_progress`；在它完成完整交付与归档闭环前，不得将其或 PR #12 记录为已完成产品变更。

## 后续交付顺序

1. [ ] **修复 v2.0.2 Tag Release 构建失败**
   - Trellis：`07-29-fix-v2-0-2-release-build`
   - 优先级：P2，但属于发布阻断，应在普通功能修改前处理。
   - 已知计划：修复 release workflow 的 archive 绝对路径和 Windows `.exe` 命名；运行 package/release 验证。
   - 约束：先修复发布链路；不得重写 `v2.0.2` tag，补发/重建策略须经用户明确批准。

2. [ ] **增强报告服务筛选与未识别服务排除**
   - Trellis：`07-29-enhance-report-service-filters`
   - 优先级：P1。
   - 契约：筛选仅是报告/视图投影，默认仍显示完整数据；“未识别服务”仅为服务名为空、`unknown` 或 `tcpwrapped`。

3. [ ] **研究未识别服务通用指纹增强**
   - Trellis：`07-29-research-unknown-service-enrichment`
   - 优先级：P1。
   - 安全边界：分别处理空服务名、`unknown` 和 `tcpwrapped`；不得因服务未知而泛化执行 Nuclei，也不得扩大 Dameng 探针。
   - 产物：以证据支持的后续独立任务，不直接混入规则修改。

4. [ ] **调查 SSL/TLS 检测覆盖与缺口**
   - Trellis：`07-29-research-ssl-tls-coverage`
   - 优先级：P2。
   - 产物：识别、配置、证书、漏洞检测的覆盖矩阵；每项应有代码/配置位置、工具或模板版本、可复现验证和安全限制。

5. [ ] **统一 Web Console 上海时区显示**
   - Trellis：`07-29-fix-console-shanghai-time`
   - 优先级：P2。
   - 契约：只在 UI 以 `Asia/Shanghai` 显示 `YYYY-MM-DD HH:mm:ss.SSS UTC+8`；存储/API 保持 UTC RFC3339。

6. [ ] **完成运行版本、控制台输出与异常完成状态任务树**
   - Trellis 父任务：`07-29-fix-runtime-version-console-status`
   - 剩余子项：`07-29-fix-console-version-contract`
   - 优先级：P2。
   - 契约：版本显示由 release tag 注入并去除前导 `v`；Console 事件仅摘要化持久化消息而不改日志/artifact；`completed_with_errors` 仅反映已定义的部分失败路径。

## 开始下一项前的检查

1. 从 `origin/main` 创建专用分支，避免携带已合并 PR 后的本地元数据提交。
2. 阅读目标 Trellis task 的 `prd.md`、`design.md`、`implement.md`，确认 ready gate。
3. 行为变更遵循：TDD Red -> Green -> self-check -> Standards review -> Spec/AC review -> full verification -> PR。
4. 研究任务先产出可复现证据和边界，不直接引入猜测性的检测规则。
