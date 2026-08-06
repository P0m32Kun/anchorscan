# 任务书：fathom 集成 M4.1 — runner + JSONL 解析 + 归一化别名表 + Finding 映射

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。
> Spec：`docs/plans/fathom-integration/spec.md`（v1.0，4 决策点已由用户拍板 ✅）

## 背景

fathom（自研 Rust 侦察工具，~/DEV/fathom，M1-M7 已完成，零依赖）将接入 anchorscan 作为侦察引擎，替代 rustscan+nmap(-sn/-sV)。**本任务只做 M4.1**：fathom runner、JSONL 解析、服务名归一化别名表、checks→Finding 映射。**不做 scan_target 集成（M4.2）**。

## 必读

1. `AGENTS.md`（仓库根，编排约定与硬约束）
2. `docs/plans/fathom-integration/spec.md`（含已拍板的 4 决策点：达梦接受/TLS 缓解/CPE 降级/IPv6 legacy）
3. **fathom 仓库 `~/DEV/fathom`**（只读）：
   - `fathom scan --json` 的 JSONL 输出 schema——**必须从 fathom 源码/测试 fixture 确认字段，禁止猜测**（重点：`src/` 下的输出结构、`docs/m1-report.md`~`m7-report.md`、fathom 自带的测试 fixture）
   - 服务名枚举（fathom 输出的 service 名与 nmap 风格名的差异，如 mssql vs ms-sql）
4. `internal/tools/` 现有工具接缝（nmap.go 等：runner 接口、执行模式、超时）
5. `internal/fingerprint/normalize.go`（归一化语义）与 `internal/report/` 的 `Finding`/`DetectionCheck` 结构
6. `internal/config/` 的 ToolPaths / Timeouts 结构（加 fathom 字段）

## Scope（只做 M4.1）

**要做**：
1. **config**：`tools.fathom` 路径（PATH 自动探测，与其它工具一致）+ `timeouts.fathom`；不新增 fathom_args（fathom 无需额外参数；ports 由 scan.ports 传入）
2. **`internal/tools/fathom.go`**：`RunFathomScan(ctx, runner, binary, targets, ports)` 执行 `fathom scan --json <targets> -p <ports>`，解析 JSONL → `[]fingerprint.ServiceFingerprint`（service/product/version/IsWeb/URL/CPE 等按 spec 决策映射）+ checks 载荷
3. **归一化别名表**：normalize.go 增加 fathom 服务名→归一化名（nmap 风格键，如 mssql→ms-sql）映射；附**双跑平价测试**（同一数据分别喂 nmap XML 解析与 fathom JSONL 解析，断言归一化结果一致——参考 internal/tools 的 nmap XML 解析 fixture）
4. **Finding 映射**：fathom checks → `report.Finding{Source: "fathom", ID: check-id, Severity: 按 CHECKS.md 映射（未授权/弱口令=high）, Output: proof}`；DetectionCheck 记录 engine="fathom"（按 spec 决策：达梦检出时跳过 nuclei dameng-identify 的衔接留到 M4.2，本阶段只保证 Finding 结构正确）
5. **TLS 缓解**（spec 决策 2）：仅实现"未知服务 + TLS web 候选端口集（443/8443/9443…）→ 标记待 httpx 增强"的数据结构预留（枚举+注释），**不实现 httpx 触发**（M4.2）
6. **CPE/IPv6**（spec 决策 3/5）：fathom 指纹的 CPE 字段按降级处理（允许为空）；IPv6 不涉及本阶段（M4.2 处理 legacy 回退）

**不要做**：
- **不做 scan_target 集成**（M4.2：profile 门控、前段切换、双跑）
- 不做 fathom-dual profile（M4.3）
- 不实现 fathom discover 段、不写 IPv6 处理逻辑
- 不做 git 操作（commit/push/checkout 一律禁止）
- 不引入新依赖（Go std + 既有依赖）

## 铁律

1. 零新依赖；纯 Go（沿用现有 runner/exec 模式）
2. **fathom JSONL 字段以 fathom 仓库源码/测试为准**，不得猜测；报告中列出你确认 schema 的来源文件与行号
3. JSONL fixture 必须是真实 fathom 输出的形态（可从 fathom 测试 fixture 复制/裁剪，注明来源）；不得自造不符合 schema 的假字段
4. 诚实报告：报告分列「实测」与「静态推断」；fathom binary 无法在本环境跑通时如实标注（如可行，尝试用 ~/DEV/fathom 构建产物跑一次真输出）
5. 已知未跟踪文件（spikes/ 等）为既往产物，不得删除或修改，无需确认
6. 完成后不得自行 commit；报告文件：`docs/reports/fathom-m41-report.md`（含：fathom JSONL schema 确认来源、改动文件清单、单测/平价测试证据、TLS 预留结构说明、遗留风险）

## 验收

1. `go build ./...` 通过
2. `go test ./internal/tools/ ./internal/fingerprint/ ./internal/config/ -count=1` 全过（新增 fathom 相关测试）
3. 平价测试：nmap XML 解析 vs fathom JSONL 解析，同一语义数据归一化结果一致（断言在测试里）
4. JSONL fixture 来自 fathom 仓库真实输出形态（报告注明来源）
5. 报告含 spec 4 决策点在本阶段的落地说明（哪些做了、哪些留给 M4.2）
