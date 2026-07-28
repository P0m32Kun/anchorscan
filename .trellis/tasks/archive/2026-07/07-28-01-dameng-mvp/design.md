# Design — 达梦数据库默认口令检测 MVP

## 1. 总体流程

在现有扫描流程中插入一个**达梦主动指纹识别**阶段，并修改 POC 触发条件为指纹驱动：

```
rustscan → nmap 指纹识别 → 达梦主动识别（新） → httpx 增强 → NSE/nuclei → dameng POC（新）
```

## 2. 组件设计

### 2.1 主动指纹识别：`internal/fingerprint/probes/dameng.go`

**职责**：向目标端口发送达梦协议握手探测包，根据响应判断是否为达梦服务。

**接口**：

```go
package probes

import "github.com/P0m32Kun/anchorscan/internal/fingerprint"

// DetectDameng attempts to identify a Dameng DB listener on the given fingerprint.
// It returns an updated fingerprint with Normalized="dameng" if the probe succeeds,
// or the original fingerprint if not.
func DetectDameng(ctx context.Context, fp fingerprint.ServiceFingerprint, timeout time.Duration) (fingerprint.ServiceFingerprint, bool, error)
```

**探测包**（来自 Nuclei `dameng-detect.yaml`）：

```
00000000c8005100000000000000000000000099000000000000000001020000000000000000000000000000000000000000000000000000000000000000000008000000382e312e312e34390040000000068149bbe004a62fb45552831704c802d4d802b4579cb045b3c6100880725ececf148a7c9205047caccadfef5ff264460d11092a3b483bf9d24382dea1dc43e7
```

**判定逻辑**：
1. TCP 连接目标端口。
2. 发送 hex 探测包。
3. 读取响应。
4. 若响应非空且包含达梦协议特征（长度/魔数/版本字段位置符合 DM 协议），判定命中。

由于协议私有，MVP 阶段采用**保守匹配**：响应长度大于 0 且首字节符合达梦响应包常见模式即可。后续随真实抓包可细化特征。

**失败处理**：网络错误、超时、无响应均视为未命中，不报错；上层记录 `skipped`。

### 2.2 默认口令检测器：`internal/tools/dameng.go`

**职责**：对识别为达梦的端口，使用 Go 驱动尝试默认口令登录。

**接口**：

```go
package tools

type DamengVerdict int

const (
    DamengUnknown DamengVerdict = iota
    DamengVulnerable
    DamengSafe
    DamengError
)

type DamengResult struct {
    Verdict DamengVerdict
    Output  string
}

// DamengAuthChecker abstracts the real driver for testing.
type DamengAuthChecker interface {
    Check(ctx context.Context, host string, port int, username, password string) (bool, string, error)
}

// RunDamengDefaultPassword runs the default-password check against a Dameng service.
func RunDamengDefaultPassword(ctx context.Context, checker DamengAuthChecker, ip string, port int) (DamengResult, error)
```

**判定逻辑**：
- `Check()` 返回 `true`：认证成功 → `DamengVulnerable`。
- `Check()` 返回 `false`：认证失败 → `DamengSafe`（口令已改）。
- `Check()` 返回错误：区分认证错误（safe）与网络错误（error）。

**生产实现**（使用 `gitee.com/chunanyong/dm`，跨平台官方 Go 驱动封装）：
```go
type damengDriverChecker struct{}

func (c *damengDriverChecker) Check(ctx context.Context, host string, port int, username, password string) (bool, string, error) {
    dsn := fmt.Sprintf("dm://%s:%s@%s:%d", url.PathEscape(username), url.PathEscape(password), host, port)
    db, err := sql.Open("dm", dsn)
    if err != nil {
        return false, "", err
    }
    defer db.Close()
    if err := db.PingContext(ctx); err != nil {
        return false, "", err
    }
    return true, "", nil
}
```

### 2.3 扫描调度：`internal/app/scan_target.go`

新增阶段 A —— 达梦主动识别：

在 nmap fingerprint 循环中，对每个 `fp`：
1. 如果 `fp.Normalized` 已经是已知数据库（mysql/postgres/mssql/redis/oracle 等），跳过。
2. 调用 `probes.DetectDameng(ctx, fp, timeout)`。
3. 命中则更新 `fp.Normalized = "dameng"` 并重新持久化。

新增阶段 B —— dameng POC：

参考 `rdpscan` 段，在扫描循环末尾增加 dameng 段：
- `fp.Normalized != "dameng"` → `recordDetectionCheck(..., "dameng", "skipped", "no_matching_rule", ...)`
- `opts.Tools.Dameng == ""` → skipped/tool_unconfigured
- 否则调用 `tools.RunDamengDefaultPassword(...)`，根据 verdict 生成 finding 或记录 completed。

### 2.4 配置与指纹归一化

- `internal/fingerprint/normalize.go`：增加别名，如 `"dameng"`、`"dm"`、`"dameng db"` → `"dameng"`。
- `config/default.yaml` / `.example`：增加 `tools.dameng: "dameng"` 或布尔开关。
- `config/ports-highrisk.txt`：加入 `5236`。

## 3. 数据流

```
TargetScan
  ├─ rustscan 端口列表
  ├─ nmap 生成 fingerprint[]
  │   └─ 对每个 fp：
  │       ├─ probes.DetectDameng() → 命中则 fp.Normalized = "dameng"
  │       ├─ httpx 增强
  │       ├─ NSE / nuclei 漏洞检测
  │       └─ dameng 默认口令检测（条件：Normalized == "dameng"）
  └─ 返回 fingerprints + findings
```

## 4. 测试策略

按 `docs/testing-strategy.md` 选择最低充分缝：

| 行为 | 测试缝 |
|---|---|
| `DetectDameng` 协议探测命中/未命中 | Go 单元测试，本地监听 fake TCP server 返回模拟响应 |
| `RunDamengDefaultPassword` verdict 映射 | Go 单元测试，注入 fake `DamengAuthChecker` |
| `scan_target.go` 触发条件 | Go 单元测试，使用 `tools.Runner` mock 验证 dameng 段调用 |
| 端到端 | 可选：真实达梦 Docker 手动冒烟 |

## 5. 风险与回退

| 风险 | 缓解 |
|---|---|
| 达梦 Go 驱动拉取失败 | 优先 GitHub 镜像；CI 预验证 |
| 协议探测包在新版达梦失效 | 探测包来自社区模板，较稳定；失效时仅导致漏报，不误报 |
| 扫描时间增加 | 仅对未识别端口发送单包探测，超时 3s |
| 默认口令登录被目标记录 | 这是检测行为本身，需在报告中说明；MVP 只尝试默认一对凭证 |
