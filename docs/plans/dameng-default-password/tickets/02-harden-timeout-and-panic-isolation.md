# 02 — 隔离达梦检测的超时与驱动 Panic

**What to build:** 为达梦默认口令检测提供 15 秒的新配置默认值，并将第三方达梦驱动的 panic 或 deadline 转换为可审计的 DetectionCheck 失败，不能使整个扫描进程退出。

**Blocked by:** 无。

**Status:** done — 已完成确定性自动化验收；Docker 人工冒烟按已记录的 fixture 缺口延期。

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 新生成配置的 `timeouts.dameng` 为 `15s`；用户显式 `0` 仍代表不限时。
- 达梦认证成功仍产生弱口令 Finding；认证拒绝仍为安全结论；普通连接/协议错误仍为 unknown。
- 第三方驱动 panic 或达梦检查 deadline 不得退出扫描进程；对应检查是 `failed/command_failed`，Run 依据既有部分失败语义结束为 `completed_with_errors`。
- 不更换驱动、不修改凭据尝试范围、不变更历史 Run。

## 测试 seam

- Tools unit：注入 panic/deadline 的 `DamengAuthChecker`，验证恢复与分类。
- App fake checker + Store：验证 DetectionCheck、Finding 缺失和 Run 终态。
- Config unit：验证 15 秒默认值及显式 `0` 兼容性。
- Docker 人工冒烟已延期：已调查的第三方镜像无法可靠提供可配置的 `SYSDBA/SYSDBA` fixture；本 ticket 不再继续镜像探索。

## 验收

- [x] 先写 panic checker 不使测试或扫描 goroutine 崩溃的失败测试。
- [x] 先写 deadline 被记录为失败而不是 completed unknown 的失败测试。
- [x] 认证拒绝、普通网络错误和默认凭据成功的既有行为不回归。
- [x] `go test ./internal/tools ./internal/app ./internal/config`、`make test`、`go vet ./...` 通过；`make pr-check` 亦通过。
- [x] Docker fixture 缺口已记录为延期项，不阻塞 deterministic tools/app/config 自动化验证。

## 完成记录

- `RunDamengDefaultPassword` 仅在 `DamengAuthChecker.Check` 调用边界恢复第三方 panic，并将 panic 和 `context.DeadlineExceeded` 返回为非 nil 执行错误；认证和普通连接错误的既有 verdict 未变。
- App 回归测试验证两类失败均持久化 `failed/command_failed`、保留可诊断 Detail、无弱口令 Finding，且 Run 为 `completed_with_errors`。
- 已完成独立 standards/spec review；无 blocker 或 high，后续补足的 Detail 与 Finding 断言已复测。

## 非目标

- 全局收紧所有工具 timeout。
- Fork、升级或替换达梦驱动。
- 将未经验证来源的第三方达梦镜像引入 CI 或发布依赖。
- 把任意未知连接错误都升级为失败，或扫描现场目标以验证该 bug。
