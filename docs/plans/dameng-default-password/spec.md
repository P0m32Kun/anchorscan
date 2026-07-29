# Spec — 达梦数据库默认口令检测器

**Status:** completed

达梦默认口令检测 MVP 已于 2026-07-28 交付。权威交付 ticket 是
[`tickets/01-delivered-mvp.md`](tickets/01-delivered-mvp.md)；归档 Trellis task
`07-28-01-dameng-mvp` 只保存执行记录和该 ticket 的引用。

## 1. 背景与覆盖缺口

AnchorScan 的双引擎漏洞检测（nuclei + NSE）在国产数据库达梦（DM）上存在明确空白。

依据 `docs/research/vulnerability-coverage-official-sources.md` 第 48 项（核查日期 2026-07-19，已核对 Nmap 7.99 / Nuclei 3.11.0 / nuclei-templates v10.4.5 官方快照）：

> **达梦数据库默认口令：NSE「无直接」、Nuclei「无直接」** —— 「两个官方库未找到 Dameng/DM 默认口令检测；需凭证审计或自定义模板。」

结论：**上游无现成方法，必须自定义检测器（POC）。**

## 2. 达梦事实（已核实）

> 达梦官网 `eco.dameng.com` 在本机被 fake-IP 代理拦截（`198.18.0.0/15`），以下事实通过 web 搜索交叉核实，官方 Go 编程指南链接：<https://eco.dameng.com/document/dm/zh-cn/pm/go-rogramming-guide.html>

| 项 | 值 |
|---|---|
| 默认端口 | **5236**（DM7/DM8 官方默认，`dm.ini` 中配置） |
| 默认管理员账号 | `SYSDBA` |
| 默认口令 | `SYSDBA`（明文；新版本安装强制改密，但历史/默认部署仍常见默认值） |
| 协议 | DM 私有二进制协议，口令用服务端下发公钥做**非对称加密协商** |
| Go 驱动（推荐） | `gitee.com/chunanyong/dm`（社区封装的达梦官方 Go 驱动，`database/sql` 标准，go mod 友好） |
| 驱动备选 | `github.com/ganl/go-dm`（GitHub 镜像，CI/GOPROXY 友好） |
| DSN 格式 | `dm://SYSDBA:SYSDBA@host:5236`（密码含特殊字符需 `url.PathEscape` + `&escapeProcess=true`） |
| 用法 | `import _ "gitee.com/chunanyong/dm"` → `sql.Open("dm", dsn)` → `db.Ping()` 成功即默认口令存在 |

## 3. 已纠正的端口记录

`docs/research/vulnerability-coverage-official-sources.md` 第 48 项已修正为达梦默认端口
**`5236`**。`config/ports-highrisk.txt` 同时保留 `5236`（达梦）和 `12345`
（其他高风险服务）；后者不是达梦端口，不应移除。早期实施 PRD 中“移除 `12345`”的
表述是错误的，产品配置保持正确。

## 4. 技术路线决策（已定）

为什么**不能**只走 nuclei / NSE 做默认口令检测，必须用 anchorscan 内置原生 Go 检测器：

| 路线 | 结论 | 原因 |
|---|---|---|
| nuclei network/tcp 模板做**口令检测** | ❌ 不可行 | 私有协议 + 口令非对称加密协商，模板无法构造正确握手字节；跨大版本协议不兼容。 |
| nuclei code 模板做口令检测 | ❌ 务实不可行 | 需 code 模板签名 + 本地装驱动 + `-code` 开关；改造成本 > 自写。 |
| NSE 脚本做口令检测 | ❌ 不可行 | Lua 无达梦客户端，手写协议比 Go 难得多。 |
| **内置原生 Go 检测器** | ✅ 采用 | 本项目即 Go；用达梦 Go 驱动发起真实登录，`Ping()` 成功判默认口令存在。 |

**指纹识别**：nmap 对达梦没有 service probe，常把 5236 报成 `padl2sim` 或 `unknown`。因此**不能依赖固定端口或服务名**。MVP 采用 Nuclei 社区 `javascript/detection/dameng-detect.yaml` 中的主动协议握手包做轻量级指纹识别，命中后再跑 Go 检测器。该探测包只握手、不登录，对目标影响小。

**挂载范式**：
- 指纹识别：新增 `internal/fingerprint/probes/dameng.go`。
- 漏洞检测：仿照 `internal/app/scan_target.go` 中 `rdpscan` 作为可选引擎段，复用其三态 verdict + `recordDetectionCheck` 记录样板。

## 5. 涉及文件与改动点

| 层 | 文件 | 改动 |
|---|---|---|
| 主动指纹 | `internal/fingerprint/probes/dameng.go`（新增） | 使用 nuclei 社区探测包做协议指纹识别，返回命中状态并更新 `Normalized` |
| 工具层 | `internal/tools/dameng.go`（新增） | 检测器实现：`Ping()` 判定，返回 verdict + output |
| 指纹层 | `internal/fingerprint/normalize.go` | `aliases` 增加达梦归一化 |
| 调度层 | `internal/app/scan_target.go` | nmap 后增加达梦主动识别；末尾增加 `dameng` 引擎段，仿 `rdpscan` |
| 配置层 | `config/default.yaml` + `.example` | `tools` 增加 `dameng` 开关（默认启用），`timeouts` 增加 `dameng` |
| 研究文档 | `docs/research/vulnerability-coverage-official-sources.md` | 第 48 项端口 `12345` → `5236` |

## 6. 关键设计点与风险

1. **指纹识别不固定端口**：主动协议探测不依赖 nmap 服务名，达梦跑在任意端口都可能被识别。触发 POC 的唯一条件是 `fp.Normalized == "dameng"`。
2. **误报控制**：`sql.Open` 本身不发网络包，判定必须以 `Ping()`（真实握手）成功为准；连接超时与认证失败要区分（认证失败 = 服务在但非默认口令，不算漏洞）。
3. **依赖供应链**：`github.com/ganl/go-dm` 是 GitHub 镜像，CI/GOPROXY 友好。新增依赖会增大产物体积与供应链面，需评估。
4. **跨层回归风险**：新增依赖影响构建；改 `scanTarget()` 调度影响所有扫描路径，需全量回归。
5. **保守的协议响应匹配**：MVP 阶段对主动探测响应采用宽松匹配（非空 + 4 字节长度字段合理），后续可随真实抓包收紧。

## 7. 交付状态与验收证据

- [x] `e69b8cd` 实现主动指纹识别、默认口令检测器、指纹驱动调度和检测审计记录。
- [x] `5236` 已在高危端口预设中；`12345` 仍作为非达梦高危端口保留。
- [x] 归档 task 的实现与设计文档存在：
  `.trellis/tasks/archive/2026-07/07-28-01-dameng-mvp/`。
- [x] 单元测试覆盖主动探测命中/未命中、verdict 映射和调度条件。
- [ ] 对真实默认口令达梦实例的端到端观察：未记录为本次已知证据。
- [ ] 原始 Red/Green、独立评审、完整 PR URL：未记录为本次已知证据，不作追溯性补写。

## 8. 测试策略（无真实达梦环境如何 TDD）

- **检测器接口抽离**：把"发起登录并判定"抽象为可注入的 `DamengAuthChecker` 接口，生产实现调真驱动，测试用假连接器返回（握手成功 / 认证失败 / 网络错误）三种结果，验证 verdict 映射。
- **主动指纹识别单元测试**：用本地 fake TCP server 返回模拟 DM 响应，验证 `DetectDameng` 命中与未命中。
- **触发条件单元测试**：用表驱动测试验证 `fp.Normalized == "dameng"` 时 dameng 引擎段才会被调用；nmap 已明确识别为 mysql 等已知服务时不触发主动探测。
- **真实环境验证（可选，非门禁）**：用达梦官方 Docker 镜像（若可获取）或本地安装做端到端冒烟，作为 manual 验收证据。
- 遵循 `docs/testing-strategy.md`，选最低充分测试缝，避免跨层重复覆盖。

## 9. 后续工作

本 MVP 已完成。后续若扩展达梦协议版本识别、凭证策略或真实环境 E2E，应新建 ticket，
而非复用本交付 ticket。

---

## 变更记录

- 2026-07-28 — 初版。完成调研与路线决策（nuclei/NSE 均无 → 原生 Go 检测器）。
- 2026-07-29 — 对齐已交付实现、归档 Trellis task 与权威 spec；明确历史未观测证据，并纠正 `12345` 保留规则的冲突表述。
