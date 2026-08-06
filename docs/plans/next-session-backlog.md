# 跨会话后续工作清单

> 最后更新：2026-08-06（PR #36 合并后）
>
> 用途：记录已规划但尚未交付的工作顺序及前置条件。开始任何一项前，先检查 `origin/main`、相关 `docs/plans/` ticket 与此清单是否一致。

## 已确认产品交付

- v2.0.2 发布链路修复已由 PR #3 合并；release run `30383917510` 成功并发布三平台归档与 checksums。
- Console 输出规范化已由 PR #9/#10 合并。
- 默认 Nuclei 模板规则清理与 SSH tags 路由已由 PR #11/#12/#23 合并。
- Spark Web UI/API 的安全 tags 路由已由 PR #13 合并。
- 报告服务筛选已由 PR #17/#18 合并。
- Web Console 上海时区显示已由 PR #20/#24 合并。
- 运行版本契约已由 PR #25 验证并归档。
- 知识库 v2 catalog 对接已由 PR #34/#35 合并（catalog v2 协议 + 五档门禁 + 单源分发；catalog 单源在 Pentest-Playbook，发行包零副本）。
- nmap-viewer 功能收敛已由 PR #36 合并（t02–t09 + known-issues tracker + scroll-spy CI 修复）。AnchorScan 声明为唯一维护产品（nmap-viewer 退役）。

## 后续交付顺序

1. [ ] **修 ISSUE-001：sqlite DSN 拼接产生垃圾文件**
   - 优先级：P1（每次 `make pr-check` 都产生垃圾文件）。
   - 位置：`internal/store/sqlite.go` 约第 27 行，`path + "?_pragma=..."` DSN 拼接。
   - 方向：确认 modernc sqlite 的 `?_pragma=` 是否应改用 `?` query 参数或其他 DSN 格式。
   - 验收：`make pr-check` 后工作目录无 `?_pragma=...` 垃圾文件。
   - 参考：`docs/known-issues.md` ISSUE-001。

2. [ ] **修 ISSUE-002：internal/app 租约竞争测试 CI 偶发失败（flaky）**
   - 优先级：P2（CI 偶发红、rerun 可恢复）。
   - 位置：`internal/app/run_lease.go`（reserveRunLease → ReconcileInterruptedRuns / AcquireRunLease）。
   - 方向：先稳定复现（`-race` 或隔离 attempt），疑似两个独立 store 连接对同一 db 文件的锁时序竞态。
   - 参考：`docs/known-issues.md` ISSUE-002。

3. [ ] **fathom 集成（M4）：替代存活/端口/指纹三段 + 权限级检测**
   - 优先级：P1。
   - Spec（草案 v1.0）：`docs/plans/fathom-integration/spec.md`；fathom 仓库 ~/DEV/fathom（M1–M7 已完成，五段流水线 + 10 checks + 三平台 + 零依赖）。
   - **前置：用户拍板 4 个决策点**——达梦权威（fathom 协议握手替代 nuclei dameng-identify）、TLS 缓解（候选端口集触发 httpx 增强）、CPE 降级（报告 CPE 字段为空）、IPv6 走 legacy。
   - 分期：M4.1 runner+JSONL+归一化+Finding 映射 → M4.2 scan_target 前段切换（profile 门控）→ M4.3 fathom-dual 双跑对比 → M4.4 默认切换+文档。
   - 验收门槛：lab 指纹平价（fathom ⊇ nmap）+ 真实 /24 双跑对比。

4. [ ] **研究未识别服务通用指纹增强**
   - 优先级：P1。
   - 安全边界：分别处理空服务名、`unknown` 和 `tcpwrapped`；不得因服务未知而泛化执行 Nuclei，也不得扩大 Dameng 探针。
   - 产物：以证据支持的后续独立任务，不直接混入规则修改。

5. [ ] **调查 SSL/TLS 检测覆盖与缺口**
   - 优先级：P2。
   - 产物：识别、配置、证书、漏洞检测的覆盖矩阵；每项应有代码/配置位置、工具或模板版本、可复现验证和安全限制。

## 开始下一项前的检查

1. 从 `origin/main` 创建专用分支。
2. 若工作需要跨会话保存或拆分多个 ticket，先在 `docs/plans/` 建立唯一权威 spec/ticket。
3. 行为变更遵循：明确验收标准 -> 聚焦实现与测试 -> full verification -> PR。
4. 研究任务先产出可复现证据和边界，不直接引入猜测性的检测规则。
