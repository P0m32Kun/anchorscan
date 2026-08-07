# Fathom 集成：替代存活/端口/指纹三段 + 权限级检测

> 状态：v2.0 已拍板（2026-08-07，设计反转：fathom 成为唯一扫描路径，不保留 legacy 回退）
> 前置证据：fathom spikes/001（nmap 发包量基线）、spikes/002（fathom 指纹引擎验证）、
> fathom M1-M7 验收报告（~/DEV/fathom/docs/）

## Problem Statement

当前流水线 rustscan→nmap(-sn/-sV)→httpx→NSE/nuclei 中，rustscan+nmap -sV 是时间与发包瓶颈：
lab 9 端口实测 146s / 117 连接，占整扫 90%+ 时间（spike 001）。fathom（自研 Rust 工具）
已完成五段能力：存活+端口+指纹+高危检测，同场景 4.2s / 22 连接、9/9 识别（spike 002）。

**集成目的不是"加一个选项"，而是降低软件设计复杂度**：fathom 包揽存活+端口+指纹+高危检测，
减少工具数量、减少数据在多工具间的流转。保留 rustscan+nmap 做回退 = 维护两套路径，
复杂度翻倍，违背初衷。

## Solution

fathom 成为**唯一扫描路径**（非可选 profile，非灰度切换）：

- `scan_target` 前段直接调 `RunFathomScan`（替代 rustscan 端口发现 + nmap -sV 指纹两次调用）
- fathom JSONL 落 artifactDir
- checks → `report.Finding{Source: "fathom", ...}` + DetectionCheck engine="fathom"
- fathom 未配置 → preflight 直接报错，不启动扫描（不回退、不降级）
- rustscan 从扫描流水线移除（config 字段保留但扫描不再调用）
- nmap 角色变为**仅 NSE 引擎**（不再跑 -sn 存活 / -sV 指纹）
- httpx/nuclei/rdpscan 保留（fathom 指纹喂给它们）

### 关键映射决策（2026-08-06 用户已全部拍板）

1. **服务名归一化**：normalize.go fathom→nmap 别名表（mssql→ms-sql 等）——✅ M4.1 已完成
2. **IsWeb/Tunnel/URL**：fathom http 产品识别 → IsWeb；TLS 缺口缓解（未知服务 + TLS web 候选端口
   触发 httpx 增强）——✅ 已拍板，M4.2 实现
3. **CPE**：fathom 不产 CPE；报告降级为空——✅ 已拍板（接受降级）
4. **达梦**：fathom dameng 探针是协议级握手；fathom 检出 dameng 时跳过 nuclei dameng-identify，
   直接进入默认口令检查——✅ 已拍板（接受）
5. **IPv6**：fathom 当前仅 IPv4；IPv6 不支持（fathom 后续里程碑，非 anchorscan 回退）——✅ 已拍板
6. **discover 段**：集成中保持关闭——✅

### v1.0 → v2.0 设计反转

v1.0 的灰度/双跑/旧路径保留方案全部推翻：
- ~~profile `fathom-dual` 双跑模式~~ — 删除
- ~~旧路径保留一个版本周期~~ — 删除
- ~~scan_target 门控回退 legacy~~ — 删除
- ~~fathom 未配置时回退 rustscan+nmap~~ — 改为直接报错退出

## Testing Decisions

- internal/tools seam：fake Runner + fathom JSONL fixture TDD——✅ M4.1 已完成
- scan_target 集成测试：fake Runner，fathom JSONL → 指纹 → httpx/NSE/nuclei 衔接
- fathom 未配置时 preflight 报错（不回退）的测试
- 真实验证：lab 扫描对比（fathom 替代后功能等价）

## 实施分期

| 子阶段 | 内容 | 验收 | 状态 |
|---|---|---|---|
| M4.1 | fathom runner + JSONL 解析 + 归一化别名表 + Finding 映射 | 单测全绿，fixture 平价 | ✅ 已合并 PR #42 |
| M4.2 | scan_target 前段切换为 fathom（唯一路径，无回退）；fathom 未配置 preflight 报错；TLS web 增强；达梦衔接 | lab 全绿 | 进行中 |
| M4.3 | 收尾：doctor/web/preflight 适配；rustscan 从扫描路径清理；文档更新 | 全量 pr-check 绿 | 待定 |

## Out of Scope

- fathom discover 段接入、IPv6 支持、fathom TLS 探测（fathom 后续里程碑）
- fathom checks 替代 nuclei（边界不变：fathom 管权限级，nuclei 管覆盖广度）
