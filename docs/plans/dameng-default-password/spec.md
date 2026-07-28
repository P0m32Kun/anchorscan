# Spec — 达梦数据库默认口令检测器

**Status:** backlog（未排期，待有时间实现）

本计划为 backlog 占位，**调研与路线决策已完成**（2026-07-28），只待排期实施。下次接手可直接按 §9 启动，无需重新调研。

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

## 3. 待纠正的事实错误（实现时一并修复）

调研中发现既有数据存在**端口错误**，会让 POC 打不到真实目标：

1. `docs/research/vulnerability-coverage-official-sources.md` 第 48 项将达梦端口列为 `12345`，应为 **`5236`**。`12345` 与达梦无关。
2. `config/ports-highrisk.txt` 当前**含 `12345`、缺 `5236`**，方向反了。实现时需：加入 `5236`，并处置 `12345`（若无其他用途则移除）。

## 4. 技术路线决策（已定）

为什么**不能**走 nuclei / NSE，必须用 anchorscan 内置原生 Go 检测器：

| 路线 | 结论 | 原因 |
|---|---|---|
| nuclei network/tcp 模板（手写握手） | ❌ 不可行 | 私有协议 + 口令非对称加密协商，模板无法构造正确握手字节；跨大版本协议不兼容。这是 nuclei 官方至今无达梦模板的根因。 |
| nuclei code 模板（Go 调驱动） | ❌ 务实不可行 | 需 code 模板签名 + 本地装驱动 + `-code` 开关；anchorscan 当前调度 nuclei 用 `-tags` 不传 `-code`，改造成本 > 自写。 |
| NSE 脚本（Lua） | ❌ 不可行 | Lua 无达梦客户端，手写协议比 Go 难得多，官方库无即为证据。 |
| **内置原生 Go 检测器** | ✅ 采用 | 本项目即 Go；用达梦 Go 驱动发起真实登录，`Ping()` 成功判默认口令存在。 |

**挂载范式**：仿照 `internal/app/scan_target.go` 中 `rdpscan`（BlueKeep）作为 `scanTarget()` 内可选引擎段，复用其三态 verdict + `recordDetectionCheck` 记录样板。

## 5. 涉及文件与改动点

| 层 | 文件 | 改动 |
|---|---|---|
| 工具层 | `internal/tools/dameng.go`（新增） | 检测器实现：`Ping()` 判定，返回 verdict + output |
| 指纹层 | `internal/fingerprint/normalize.go` | `aliases` 增加达梦归一化（nmap 若返回 `dm`/自定义串 → 统一名） |
| 调度层 | `internal/app/scan_target.go` | 在 `scanTarget()` 末尾增加 `dameng` 引擎段，仿 `rdpscan` |
| 配置层 | `config/default.yaml` + `.example` | `tools` 增加 `dameng` 开关（可选，默认启用） |
| 配置层 | `config/ports-highrisk.txt` | 加 `5236`，处置 `12345`（见 §3） |
| 报告/前端 | `internal/report/*`、`internal/web/*` | 新 finding source `dameng` 的展示与图标 |
| 研究文档 | `docs/research/vulnerability-coverage-official-sources.md` | 第 48 项端口 `12345` → `5236`（见 §3） |

## 6. 关键设计点与风险

1. **指纹识别是真正难点（最高风险）**：nmap `-sV` 对 5236 多半识别为 `unknown`/`tcpwrapped`（达梦无 nmap service probe）。触发条件**不能只靠 `Normalized`**，应采用「端口 ∈ 达梦常用端口 {5236,5237,5238,…} **且** nmap 未明确归为 mysql/postgres/mssql/redis 等已知 DB」复合条件，可能还需加一个轻量的达梦握手特征探测（读服务端首包 DM 协议头）。
2. **依赖供应链**：`gitee.com/chunanyong/dm` 在 gitee，CI/GOPROXY 能否拉取需验证；若受限改用 `github.com/ganl/go-dm`。新增依赖会增大产物体积与供应链面，需评估。
3. **跨层回归风险**：新增依赖影响构建；改 `scanTarget()` 调度影响所有扫描路径，需全量回归。
4. **误报控制**：`sql.Open` 本身不发网络包，判定必须以 `Ping()`（真实握手）成功为准；连接超时与认证失败要区分（认证失败 = 服务在但非默认口令，不算漏洞）。

## 7. 验收标准

- 对运行默认口令 `SYSDBA/SYSDBA` 的达梦实例，检测器报告「达梦数据库默认口令」漏洞（high/critical）。
- 对改过口令的达梦实例，检测器**不**误报（认证失败视为无漏洞，仅记录检测已运行）。
- 对非达梦服务（如 5236 上跑别的服务），检测器不误报、不崩溃。
- 达梦端口 5236 进入高危端口预设，默认扫描可覆盖。
- 检测结果在 HTML/JSON/DOCX 报告中正确展示，`detection_checks` 有完整审计记录。
- 全量测试与静态检查通过。

## 8. 测试策略（无真实达梦环境如何 TDD）

- **检测器接口抽离**：把"发起登录并判定"抽象为可注入的 `dialer`/`connector` 接口，生产实现调真驱动，测试用假连接器返回（握手成功 / 认证失败 / 网络错误）三种结果，验证 verdict 映射。
- **触发条件单元测试**：用表驱动测试验证端口/归一化服务名复合判定（5236+unknown 触发、5236+mysql 不触发、3306+mysql 不触发等）。
- **真实环境验证（可选，非门禁）**：用达梦官方 Docker 镜像（若可获取）或本地安装做端到端冒烟，作为 manual 验收证据。
- 遵循 `docs/testing-strategy.md`，选最低充分测试缝，避免跨层重复覆盖。

## 9. 下次接手指引

1. 本 spec 即完整起点，先读本文件全文 + `docs/research/vulnerability-coverage-official-sources.md` 第 48 项。
2. 创建 `tickets/01-dameng-default-password-detector.md`，按 §5/§6 拆步骤，状态置 `ready-for-agent` 前先与用户确认。
3. 第一个不确定项优先验证：**`gitee.com/chunanyong/dm` 能否在项目 CI 的 GOPROXY 下拉取**（决定驱动选型）。
4. 按 `docs/agents/issue-tracker.md` 流程实施（fixed point → implement → tdd → code-review → done）。

---

## 变更记录

- 2026-07-28 — 初版。完成调研与路线决策（nuclei/NSE 均无 → 原生 Go 检测器），固化为 backlog spec。同时记录了 research 文档/高危端口表的端口事实错误（§3）待实现时纠正。
