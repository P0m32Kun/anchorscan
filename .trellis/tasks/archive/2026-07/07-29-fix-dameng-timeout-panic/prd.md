# 修复达梦检测超时与驱动 Panic 隔离

## Goal

达梦默认口令检测在驱动连接异常、超时或第三方驱动 panic 时不得终止整个扫描进程；操作者应在本次 Run 的 DetectionCheck 与控制台中获得可诊断的失败事实。

## Confirmed Facts

- `internal/app/scan_target.go:394` 已通过 `toolContext(ctx, opts.Timeouts.Dameng)` 为达梦检测构造上下文；`internal/tools/dameng.go:50` 将其传给 `db.PingContext(ctx)`。
- `config/default.yaml.example:22` 与 `internal/config/init.go:40` 的 `timeouts.dameng` 均为 `"0"`；`internal/app/scan.go:259-264` 对非正 timeout 不设置 deadline。因此新生成配置的达梦检查默认不限时。
- 现场栈显示 `gitee.com/chunanyong/dm@v1.8.23` 在连接握手路径触发 `index out of range [64] with length 64`；panic 穿过 `damengDriverChecker.Check` 和 `scanTarget` 工作 goroutine，使进程以 `exit status 2` 退出。
- 目前 `RunDamengDefaultPassword` 会把普通非认证错误归类为 `DamengUnknown` 且返回 nil；而 `scanTarget` 仅在返回非 nil error 时写 `failed/command_failed`。panic 没有任何恢复边界。
- 用户已批准本地 Docker 冒烟并指定两个第三方候选。`ghcr.io/renfei/dm8:1.1.144` 的公开记录标明其授权于 2022-09-25 到期。`sizx/dm8:1-2-128-22.08.04-166351-20005-CTM` 可拉取，且确认是 linux/amd64；在本机通过 `--platform linux/amd64` 运行时使用默认 license（日志到期日 2026-08-12），但 `SYSDBA_PWD=SYSDBA` 因 9-48 字符限制被拒绝，随后 50 秒内 Dmserver 未就绪且本地 disql 返回 socket connection failure。因此这两个镜像目前都不能作为正确命中的可靠验收 fixture。
- 用户提供的 Compose 将 `SYSDBA_PWD` 设为 `123456789`，所以即使容器正常也只能验证“改密后不命中”，不能验证本产品固定尝试的 `SYSDBA/SYSDBA` 正确命中；并且 `LENGTH_IN_CHAR` 行缺少 YAML 注释标记，会把中文说明拼进环境变量值，需修正后才可运行。

## Requirements

1. 新生成的默认配置将 `timeouts.dameng` 设为 `15s`；用户显式配置的正值或 `0` 继续保留既有语义，其中 `0` 仍表示用户选择不限时。
2. 在调用达梦数据库驱动的最窄生产边界恢复第三方 panic，并转换为包含 `dameng driver panic` 的可诊断错误；不得吞掉或重抛该 panic。
3. 达梦检查因 driver panic 或其超时而失败时，扫描进程继续运行；对应 DetectionCheck 为 `failed/command_failed`，Run 根据既有部分失败规则成为 `completed_with_errors`。
4. 保持正常认证拒绝为 `DamengSafe`、成功登录为 `DamengVulnerable`、其他普通连接/协议错误为 `DamengUnknown` 的既有分类。
5. 不更换达梦驱动、不扩大凭据尝试、不修改扫描授权范围，也不修改历史 Run 或已有 DetectionCheck。
6. 本次不执行 Docker 人工冒烟；等待将来获得可验证、可启动且可配置 `SYSDBA/SYSDBA` 的达梦 fixture 后再另行验证。该延期不阻塞确定性的自动化回归。

## Acceptance Criteria

- [ ] 新初始化配置和示例配置中的达梦 timeout 都为 `15s`；配置解析仍接受显式 `0` 与其他非负 Go duration。
- [ ] 注入会 panic 的 `DamengAuthChecker` 不会导致测试进程或扫描 goroutine 崩溃，且错误文本能说明驱动 panic。
- [ ] 达梦 checker 因 panic 或 deadline 失败时，测试证明 Run 返回正常的部分失败结果、DetectionCheck 是 `failed/command_failed`，且不会持久化弱口令 Finding。
- [ ] 正常认证拒绝、成功及普通网络错误的既有 verdict 行为仍由最低充分单元测试覆盖。
- [ ] Docker 人工冒烟已因可用 fixture 不足延期；其缺口和已尝试的镜像证据已记录，不阻塞本任务的自动化验收。

## Out of Scope

- 升级、替换或 fork `gitee.com/chunanyong/dm`。
- 将所有扫描器工具的默认 timeout 一并改为有限值。
- 对现场目标或历史 Run 重跑、重写或修复记录。
- 将未经验证的第三方达梦镜像作为发布、CI 或自动化回归依赖。
- 在本任务中继续探索或维护 Docker 达梦 fixture。

## Key Decisions

- 默认 timeout 仅改变未来首次生成的达梦配置为 15 秒；显式 `0` 仍是可用的不限时运维选择。
- 第三方驱动 panic 是检测引擎失败，不是“未知但完成”的安全结论；必须转换为可审计的失败记录。
- Docker 冒烟已延期：两个已试候选无法可靠提供 `SYSDBA/SYSDBA` 的可连接服务。保留其事实记录，未来有有效 fixture 时另建验证任务；本次只以确定性的 fake checker 与本地 TCP 黑洞（如实现需要）验证超时与 panic 隔离。
