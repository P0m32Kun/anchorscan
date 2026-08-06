# Fathom 集成：替代存活/端口/指纹三段 + 权限级检测

> 状态：v1.0 已拍板（2026-08-06，用户确认 4 决策点全部按推荐方案；M4.1 已委派）
> 前置证据：fathom spikes/001（nmap 发包量基线）、spikes/002（fathom 指纹引擎验证）、
> fathom M1-M7 验收报告（~/DEV/fathom/docs/）

## Problem Statement

当前流水线 rustscan→nmap(-sn/-sV)→httpx→NSE/nuclei 中，nmap -sV 是时间与发包瓶颈：
lab 9 端口实测 146s / 117 连接，占整扫 90%+ 时间（spike 001）。fathom（自研 Rust 工具）
已完成五段能力：存活+端口+指纹+高危检测，同场景 4.2s / 22 连接、9/9 识别（spike 002），
M1-M7 全部实测验收。集成后 anchorscan 外部依赖从 rustscan+nmap+httpx+nuclei 收敛为
nmap（仅 NSE）+nuclei+httpx。

## Solution

fathom 作为**侦察引擎**接入，替代 nmap -sn（存活）、rustscan（端口）、nmap -sV（指纹），
其 checks 产出 `Source="fathom"` 的 Finding。保留：httpx（tech-detect 喂 nuclei tags +
TLS web 增强）、nmap（NSE 引擎角色）、nuclei、rdpscan、dameng 默认口令检查。

### 集成形状（对齐既有工具接缝）

- config：`tools.fathom` 路径（PATH 自动探测）+ `timeouts.fathom`；不新增 fathom_args
  （fathom 无需额外参数；ports 由 scan.ports 传入）
- `internal/tools/fathom.go`：`RunFathomScan(ctx, runner, binary, scope, ports)` 执行
  `fathom scan --json <targets> -p <ports>`，解析 JSONL → `[]ServiceFingerprint` + checks
- scan_target.go 前段重写：每 target 一次 fathom 调用完成 alive→port→fingerprint，
  替代 rustscan+nmap -sV 两次外部调用；fathom JSONL 落 artifactDir
- checks → `report.Finding{Source: "fathom", ID: check-id, Severity: 按 CHECKS.md 映射
  （未授权/弱口令=high），Output: proof}`；DetectionCheck 记录 engine="fathom"

### 关键映射决策（2026-08-06 用户已全部拍板）

1. **服务名归一化**：fathom 输出的 service 名需对齐 normalize.go 语义
   （nse.yaml 键为 ms-sql/domain/amqp 等 nmap 风格）。方案：normalize.go 增加
   fathom 名→归一化名别名表（mssql→ms-sql 等），附双跑平价测试
2. **IsWeb/Tunnel/URL**：fathom http 产品识别 → IsWeb；**TLS 是已知缺口**（fathom
   当前仅明文 HTTP）。M4 缓解：未知服务且端口在 TLS web 候选集（443/8443/9443…）
   时仍触发 httpx 增强；根治列为 fathom 后续里程碑（TLS ClientHello 探测）——✅ 已拍板
3. **CPE**：fathom 不产 CPE；报告依赖 CPE 的字段降级为空，评估影响后接受
   （指纹核心字段 service/product/version 齐全）——✅ 已拍板（接受降级）
4. **达梦**：fathom dameng 探针是协议级握手（提取版本 8.1.2.128），权威性与
   nuclei dameng-detect 相当——fathom 检出 dameng 时跳过 nuclei dameng-identify，
   直接进入默认口令检查（改变"nuclei 是达梦唯一协议权威"的既有约定）——✅ 已拍板（接受）
5. **IPv6**：fathom 当前仅 IPv4；IPv6 target 走既有 rustscan/nmap 路径（降级明示）——✅ 已拍板
6. **discover 段**：集成中保持关闭（Scope 是授权边界，职责不重叠）

### 灰度与验收

- profile `fathom-dual`：双跑模式——fathom 与 rustscan+nmap 都执行，产出对比报告
  （服务清单 diff、耗时、连接数），Finding 只记一边（nmap 侧）避免重复
- 验收门槛：lab 指纹平价（fathom ⊇ nmap 且识别一致）+ 一个真实 /24 双跑对比；
  达标后 profile 默认切换到 fathom，旧路径保留一个版本周期
- Detection Coverage 汇总自然覆盖新引擎（fathom 计入每指纹的检测执行记录）

## Testing Decisions

- internal/tools seam：fake Runner + fathom JSONL fixture（含 checks、unknown、
  多服务多端口）TDD
- scan_target 集成测试：fake Runner 覆盖门控（fathom 未配置时回退 legacy 路径 or
  skipped/tool_unconfigured——二选一，倾向显式回退并记录）
- 平价测试：同一 fixture 分别喂 nmap XML 解析与 fathom JSONL 解析，断言归一化结果一致
- 真实验证：lab 双跑（fathom vs rustscan+nmap）对比报告入 docs/research/

## 实施分期

| 子阶段 | 内容 | 验收 |
|---|---|---|
| M4.1 | fathom runner + JSONL 解析 + 归一化别名表 + Finding 映射 | 单测全绿，fixture 平价 |
| M4.2 | scan_target 前段切换（profile 门控，默认仍 legacy） | fathom profile 下 lab 全绿 |
| M4.3 | fathom-dual 双跑 + 对比报告 | lab + 真实 /24 平价达标 |
| M4.4 | 默认切换 + 文档（CONTEXT.md/CHANGELOG/doctor/web 配置） | 一个版本周期观察 |

## Out of Scope

- fathom discover 段接入、IPv6 支持、fathom TLS 探测（后续里程碑）
- fathom checks 替代 nuclei（边界不变：fathom 管权限级，nuclei 管覆盖广度）
- rustscan/nmap -sV 代码删除（保留 legacy 路径至少一个版本周期）
