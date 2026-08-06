# Fathom 集成 M4.1 实施报告 — runner + JSONL 解析 + 归一化别名表 + Finding 映射

> 分支：`feat/fathom-m41-runner`
> Spec：`docs/plans/fathom-integration/spec.md`（v1.0，4 决策点已拍板）
> 任务书：`docs/fathom-m41-brief.md`
> 范围：**仅 M4.1**。不涉及 scan_target 集成（M4.2）、fathom-dual（M4.3）、默认切换（M4.4）。

## 一、结论

| 验收项 | 命令 | 结果 |
|---|---|---|
| 1. 全量构建 | `go build ./...` | ✅ PASS（exit 0） |
| 2. 三包单测 | `go test ./internal/tools/ ./internal/fingerprint/ ./internal/config/ -count=1` | ✅ 全 PASS |
| 3. 平价测试 | `TestFathomNmapNormalizationParity`（11 case） | ✅ PASS |
| 4. JSONL fixture 真实形态 | 真实 binary 实证 + 源码逐字段核对 | ✅ 见第二节 |
| 5. spec 决策点落地 | 见第五节 | ✅ |

铁律遵守：零新依赖（纯 Go std + 既有 fingerprint/report/config）；未做任何 git 操作。

## 二、fathom JSONL schema 确认来源（不得猜测 — 已双重核实）

### 2.1 schema 形态（`fathom scan --json`）

每行一个 JSON 对象，一个开放端口一行：

```jsonc
// 指纹行（fingerprint.rs:25-37 Fingerprint::json）
{"host":"...","port":N,"service":"...","product":"...","version":"...","checks":[...]}
// checks 元素（checks.rs:46-54 CheckResult::json）
{"id":"...","verdict":"vulnerable|safe|unknown","proof":"..."}
```

- `connections` 计数**仅存在于人类可读文本输出**，JSON 不含（已从源码与实测确认）。
- `checks` 数组可选；服务无适用检查时省略。
- fathom 只探测 TCP，故所有指纹映射 `protocol="tcp"`。
- `--discover` 开启时，会在指纹行尾注入 `,"discover":{...}` 字段（main.rs:174-180）。本任务保持 discover 关闭，且解析器的 `fathomRecord` 用 `encoding/json` 解码，未知键被忽略，故对 discover-on 输出向前兼容（M4.2/后续如启用 discover 不需改解析器）。

### 2.2 源码来源（行号以 ~/DEV/fathom 当前 HEAD 为准）

| 字段/枚举 | 来源文件:行 |
|---|---|
| `Fingerprint` 结构 | `src/fingerprint.rs:14-24` |
| `Fingerprint::json()` 输出顺序 host/port/service/product/version/checks | `src/fingerprint.rs:25-37` |
| `Verdict` 枚举（Vulnerable/Safe/Unknown） | `src/checks.rs:22-27` |
| `Verdict::as_str()` → `"vulnerable"\|"safe"\|"unknown"` | `src/checks.rs:29-37` |
| `CheckResult` 结构（id/verdict/proof） | `src/checks.rs:39-44` |
| `CheckResult::json()` 输出 id/verdict/proof | `src/checks.rs:46-54` |
| `run_checks` 服务门控 → check-id 分派 | `src/checks.rs`（文件末段 dispatcher） |
| scan 子命令参数解析（`--json`/`-p`/`-l`/`--no-checks` 等） | `src/main.rs:71-118`（`fn scan`） |
| write_results：JSONL 每行写一个 `result.json()` | `src/main.rs:162-182` |
| 服务 token 枚举（RULES + banner/http 探针） | `src/rules.rs`（RULES 常量）、`src/fingerprint.rs`（banner/http/found） |

### 2.3 check-id 枚举（10 项，与 anchorscan `fathomCheckSeverity` 表逐一吻合）

来源 `src/checks.rs` 的 `run_checks` 分派器 + `~/DEV/fathom/CHECKS.md`（首批 M3，准入标准=最高危「检出即等于拿到数据/权限」）：

| check-id | 服务门控 | anchorscan severity |
|---|---|---|
| redis-unauth | redis | high |
| redis-weak | redis | high |
| mysql-weak | mysql | high |
| zk-unauth | zookeeper | high |
| mongo-unauth | mongodb | high |
| es-unauth | http && product=elasticsearch | high |
| docker-unauth | http && product=docker | high |
| ssh-weak | ssh | high |
| mssql-weak | mssql | high |
| postgres-weak | postgres | high |

全部映射 `high`：符合 CHECKS.md 准入标准（最高危）。fallback 亦为 `high`（表已穷尽当前所有 check-id）。

### 2.4 服务 token 枚举（normalize 别名表的实证依据）

fathom 实际输出的 service token（来自 `src/rules.rs` RULES + `src/fingerprint.rs` 探针 + 兜底 `found()`）：
`smb, rdp, mssql, dameng, mongodb, rabbitmq, rpc, rsync, redis, zookeeper, postgres`（RULES）；
`ssh, mysql, vnc, smtp, ftp`（banner 探针）；`http, mongodb`（http 探针）；`unknown`（兜底）。

别名表仅覆盖**与 nmap/nse.yaml 键名分歧**的三项（均有 RULES 实证，非猜测）：

| fathom token | nmap/nse.yaml 键 | 别名 |
|---|---|---|
| `mssql` | `ms-sql` | mssql→ms-sql |
| `postgres` | `postgresql` | postgres→postgresql |
| `rabbitmq` | `amqp` | rabbitmq→amqp |

其余 token（redis/mysql/ssh/smb/rdp/http/mongodb/dameng/zookeeper/…）fathom 与 nmap 同名，直接穿透。

### 2.5 真实 binary 实证（实测，非静态推断）

用 `~/DEV/fathom/target/release/fathom` 真跑 `scan --json`，捕获到的真实输出：

**① ssh（本机开放 22）**：
```json
{"host":"127.0.0.1","port":22,"service":"ssh","product":"OpenSSH","version":"10.3","checks":[{"id":"ssh-weak","verdict":"unknown","proof":"no auth response"}]}
```

**② redis 未授权（127.0.0.1:6379 最小 mock，返回 redis_version、拒绝 AUTH）**：
```json
{"host":"127.0.0.1","port":6379,"service":"redis","product":"Redis","version":"","checks":[{"id":"redis-unauth","verdict":"vulnerable","proof":"redis_version:7.0.0"},{"id":"redis-weak","verdict":"safe","proof":"no weak password matched"}]}
```
→ 与 `fathom_test.go` 的 `fathomRedisFixture` **逐字段一致**（仅测试用合成端口 16379 隔离）。

**③ 关闭端口**：`scan --json -p 1 127.0.0.1` 输出为空（fathom scan 仅对开放端口产出指纹行，关闭端口不输出）。

> 说明：redis rule 的端口识别依赖 `rules.rs` RULES 的 `ports` 字段（redis=[6379,6380]）；早期 fixture 注释误标 16379，已修正为准确表述（fixture 为合成测试数据，形态来自 6379 实测）。

## 三、改动文件清单

| 文件 | 状态 | 内容 |
|---|---|---|
| `internal/tools/fathom.go` | **新增** | `RunFathomScan` / `parseFathomJSONL` / `fathomRecord`/`fathomCheck` / `fathomCheckSeverity` / `TLSWebCandidatePorts` / `NeedsTLSWebEnhancement` |
| `internal/tools/fathom_test.go` | **新增** | 9 个测试：args 构造、redis vulnerable 解析、多服务、非 JSON 行跳过、畸形行报错、runner 错误透传、web URL、TLS 谓词、**平价测试** |
| `internal/fingerprint/normalize.go` | 改 | 别名表加 mssql→ms-sql / postgres→postgresql / rabbitmq→amqp（附来源注释） |
| `internal/fingerprint/normalize_test.go` | **新增** | `TestNormalizeAliases`（含 fathom token + 既有别名回归）、product 回退 |
| `internal/config/config.go` | 改 | `ToolPaths.Fathom` / `ToolTimeouts.Fathom` / `ToolDurations.Fathom` + `Durations()`/`Normalized()` 覆盖 |
| `internal/config/init.go` | 改 | `defaultConfig` 加 `Fathom: "0"`（timeout）+ `detectToolPath("fathom")`（PATH 自动探测，与 rustscan/nmap 一致） |
| `internal/config/init_test.go` | 改 | 新增 `TestInitFathomConfig`（断言 generated≈example 的 fathom timeout 一致 + duration 解析） |
| `config/default.yaml.example` | 改 | tools.fathom + timeouts.fathom（与 defaultConfig 单一真相对齐） |

未改动：`internal/config/config_test.go`（现有 `TestLoadValidatesToolTimeouts` 字段断言兼容，无需改）；`spikes/`（既往产物，任务书明示不动）。

## 四、测试与平价证据

### 4.1 三包单测（验收命令，全过）

```
ok  github.com/P0m32Kun/anchorscan/internal/tools         PASS
ok  github.com/P0m32Kun/anchorscan/internal/fingerprint   PASS
ok  github.com/P0m32Kun/anchorscan/internal/config        PASS
```

新增 fathom 相关测试覆盖矩阵：
- `TestRunFathomScanBuildsArgs` — 命令构造 `scan --json <ip> -p <ports>`
- `TestRunFathomScanParsesRedisVulnerable` — vulnerable→Finding（Source=fathom/ID/Severity=high/Output=proof），safe/unknown→仅 DetectionCheck
- `TestRunFathomScanParsesMultipleServices` — mssql 归一化 ms-sql、unknown 穿透
- `TestRunFathomScanSkipsNonJSONLines` — banner 行容错
- `TestRunFathomScanRejectsMalformedJSONLine` — 畸形 JSON 报错
- `TestRunFathomScanPropagatesRunnerError` — runner 错误透传且保留 Output
- `TestRunFathomScanWebFingerprintSetsURL` — http 指纹 IsWeb + URL 构造（复用 `Classify`）
- `TestNeedsTLSWebEnhancement` — TLS 谓词（spec 决策 2）
- `TestFathomNmapNormalizationParity` — **平价测试**
- `TestNormalizeAliases` / `TestNormalizeFallsBackToProductWhenServiceEmpty`
- `TestInitFathomConfig` — config 字段 + duration 解析 + example 一致性

### 4.2 平价测试（验收项 3）

`TestFathomNmapNormalizationParity`（internal/tools/fathom_test.go）：同一逻辑服务，分别经 nmap XML 解析（`ParseNmapXML`+`Classify`）与 fathom JSONL 解析（`RunFathomScan`→`Classify`），断言 `Normalized` 完全一致。覆盖 11 个 case，含三个 fathom-only 分歧 token（mssql/postgres/rabbitmq）与共享 token（redis/mysql/ssh/smb/rdp/http/mongodb/dameng）。

```go
// 断言核心
if fathomNorm != nmapNorm { t.Errorf("parity FAIL …") }
if fathomNorm != c.name   { t.Errorf("normalized …") }
```

## 五、Spec 4 决策点落地说明

| # | 决策 | M4.1 落地 | 留给 M4.2+ |
|---|---|---|---|
| 1 | 服务名归一化 | ✅ **做**：normalize.go 别名表（mssql/postgres/rabbitmq）+ 平价测试 | — |
| 2 | TLS 缓解（未知服务+TLS web 候选端口→httpx 增强） | ✅ **数据结构预留**：`TLSWebCandidatePorts`（443/8443/9443/4443/8843）+ `NeedsTLSWebEnhancement` 谓词 + 单测；**不触发 httpx** | httpx 触发接入 scan_target 前段 |
| 3 | CPE 降级 | ✅ **做**：`fathomRecord` 无 CPE 字段；`ServiceFingerprint.CPE` 允许为空；`TestRunFathomScanParsesRedisVulnerable` 显式断言 CPE="" | — |
| 4 | 达梦（fathom 检出→跳过 nuclei dameng-identify） | ✅ **Finding 结构就绪**：dameng 指纹经别名穿透 normalize（dameng→dameng），run_checks 无 dameng 专用 check（dameng 仅协议握手提版本，无高危 check），故 M4.1 无 dameng Finding 产生；DetectionCheck engine=fathom 结构已就位 | nuclei dameng-identify 衔接门控（profile 前段切换时） |
| 5 | IPv6 legacy 回退 | ⏸ **不涉及本阶段**：M4.1 仅 IP-in/JSONL-out 解析层，不接 scan_target | IPv6 target→rustscan/nmap 回退 |
| 6 | discover 段关闭 | ⏸ **保持关闭**：解析器对 discover 字段向前兼容（未知键忽略），但 scan 不传 `--discover` | discover 接入属后续里程碑 |

### TLS 预留结构说明（决策 2）

```go
// internal/tools/fathom.go
var TLSWebCandidatePorts = map[int]bool{443:true, 8443:true, 9443:true, 4443:true, 8843:true}
func NeedsTLSWebEnhancement(service string, port int) bool {
    return strings.EqualFold(strings.TrimSpace(service), "unknown") && TLSWebCandidatePorts[port]
}
```

语义：fathom 的 http 探针发明文 `GET /`（src/fingerprint.rs `http()`），无法完成 TLS 握手，故 TLS-only 端口上指纹引擎放弃并报 `unknown`。当 `unknown` 命中候选端口集，标记待 httpx 增强。M4.1 仅交付数据结构与谓词（附 8 case 单测），httpx 触发由 M4.2 scan_target 前段接入。根治（fathom TLS ClientHello 探测）为 fathom 后续里程碑。

## 六、遗留风险与限制

1. **DetectionCoverage 引擎计数未含 fathom**：`report.summarizeDetectionCoverage`（report/model.go）的 triple/dual/single 计数仅认 `nse/nuclei/rdpscan` 三引擎。M4.1 的 fathom `DetectionCheck`（engine=fathom）会落入 `ScanReport.DetectionChecks` 数组（审计可见），但**不进入 coverage 引擎档位计数**。这是既有三引擎设计；是否把 fathom 纳入 coverage 统计属 M4.2 scan_target 集成决策（spec 原文「Detection Coverage 汇总自然覆盖新引擎」的精确语义需 M4.2 确认）。
2. **fathom 未安装时 path 为空**：`detectToolPath("fathom")` 在本环境返回 `""`（fathom 不在 PATH）。这是设计行为（PATH 自动探测，与 rustscan/nmap 一致，doctor 会提示未配置）；scan_target 门控回退 legacy 路径属 M4.2。
3. **fathom 仅 IPv4**：M4.1 解析层无 IPv6 处理；IPv6 target 的 legacy 回退在 M4.2。
4. **TLS web 增强未触发**：仅预留结构与谓词，真实 TLS 端口在 fathom profile 下当前会显示 `unknown` 直到 M4.2 接入 httpx。
5. **run_checks 对 dameng/mongodb 等无弱口令 check**：dameng 仅有协议握手（提版本），无高危 check，故 M4.1 下 dameng 不产生 Finding（符合 fathom CHECKS.md 边界：dameng 未授权/弱口令非 fathom 职责，归 nuclei/达梦默认口令检查）。

## 七、诚实声明：实测 vs 静态推断

- **实测（真实 binary）**：fathom JSONL 行级 schema（host/port/service/product/version/checks）、verdict 三态、redis-unauth/redis-weak/ssh-weak 的 check 形态与 proof 字符串、关闭端口无输出行为、service token（redis/ssh/unknown）。
- **静态推断（源码确认，未逐项实跑）**：其余 check-id（mysql/mssql/postgres/mongo/zk/es/docker 弱口令或未授权）的 proof 字符串与 verdict 组合——已由 `src/checks.rs` 源码逐函数确认，解析层对其通用（按 verdict 分流，不依赖 proof 文本），故不影响映射正确性。fixture 中未实测的 check 形态（如 mssql-weak safe）严格遵循 `CheckResult::json()` 同一序列化路径。
