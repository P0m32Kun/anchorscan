# 01 — 集成 rdpscan 作为 BlueKeep 可选检测引擎

**What to build:** 操作者在 config 配置 rdpscan 路径后，完整 Scan Run 对归一化为 `rdp` 的服务指纹自动执行 rdpscan；`VULNERABLE` 产出可审计的 critical Finding 与 DetectionCheck，其余结论只保留执行事实。未配置时记 `skipped/tool_unconfigured`，扫描照常。doctor 提供分平台安装指引。

**Blocked by:** 无

**Status:** done

**Spec:** `docs/plans/add-rdpscan-bluekeep-detection/spec.md`

**Review fixed point:** `94093aef2512894992544a69d8e2b2896771e125`

- [ ] config 增加 `tools.rdpscan` 路径（PATH 自动探测 + 显式覆盖）与 `timeouts.rdpscan`（与其他工具一致默认 `0`）；不新增 `rdpscan_args` 字段，yaml 往返测试通过
- [ ] `internal/tools/rdpscan.go`：`RunRdpscan` 执行并返回原始输出，`ParseRdpscanOutput` 解析 VULNERABLE / SAFE / UNKNOWN 三态；fake Runner fixture 测试先行（red-green）
- [ ] `scan_target.go`：`fp.Normalized == "rdp"` 时执行 rdpscan 段，形状对齐 nuclei 尾巴（running → artifact → parse → finding → completed）；非 rdp 记 `skipped/no_matching_rule`，未配置记 `skipped/tool_unconfigured`
- [ ] `VULNERABLE` 产出 `Source="rdpscan"`、severity critical、引用 CVE-2019-0708 的 Finding；`SAFE`/`UNKNOWN` 不产出任何 Finding
- [ ] rdpscan 原始输出落受管 Artifact；执行服从 Run 取消与超时
- [ ] `internal/doctor/doctor.go`：rdpscan 未安装仍返回 `OK=true`，避免可选工具误报 doctor 失败；消息带分平台编译指引
- [ ] `internal/preflight/preflight.go`：rdpscan 作为可选工具检查；未配置只产生 warning，不阻塞扫描
- [ ] `cmd/anchorscan/scan_command.go` 预检日志输出包含 `rdpscan` 超时
- [ ] `internal/web/config.go` + `templates/config.html`：新增 rdpscan 路径与超时输入，避免 Web 保存时覆盖该字段
- [ ] `internal/report/vulnerability_delivery.go`：rdpscan Finding 的 `Source` 落入 `ToolUnknown` 默认分支，CVE 由既有正则提取；不新增分支
- [ ] CONTEXT.md 与 CHANGELOG.md 同步；`go test ./...` 与 `go vet ./...` 全绿
