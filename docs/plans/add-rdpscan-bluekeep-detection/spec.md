# 集成 rdpscan 检测 BlueKeep（CVE-2019-0708）

## Problem Statement

Nmap NSE 没有官方 BlueKeep 脚本（`rdp-vuln-ms12-020` 是另一个漏洞）；ProjectDiscovery 因政策要求模板必须完整利用漏洞，关闭了社区提交的 BlueKeep 检测 PR，nuclei 官方不会覆盖。Metasploit 的 `auxiliary/scanner/rdp/cve_2019_0708_bluekeep` 是该漏洞事实标准的检测实现，但 MSF 是框架而非二进制：Ruby 冷启动几十秒、控制台输出非机器可读、失败形态与现有工具链不一致，不符合 anchorscan「编排单用途二进制」的架构。

此前内置 Go 探针方案（`docs/plans/archive/add-builtin-vulnerability-probes/`）经 ADR-0007 否决：Rapid7 Scan 基线需要完整 RDP 会话且官方将副作用标为未知，不满足默认开启内置探针的安全准入。

## Solution

将 **rdpscan**（robertdavidgraham/rdpscan，C 单用途二进制，检测逻辑与 MSF 模块同源于 zerosum0x0 的安全检测研究）作为**可选外部检测引擎**接入，完全复用 nuclei 的集成形状：

- 用户在 config 中配置 `tools.rdpscan` 路径（或 PATH 自动探测）；未配置时 DetectionCheck 记 `skipped/tool_unconfigured`，扫描照常。
- 完整扫描中对归一化为 `rdp` 的服务指纹（`fp.Normalized == "rdp"`，含非标准端口）执行 rdpscan。
- 输出三态：`VULNERABLE` → 产出 `source=rdpscan` 的 critical Finding；`SAFE` / `UNKNOWN` → 不产出 Finding，判定依据落入 Artifact。
- doctor 提供分平台下载与编译指引（Linux/macOS 源码 make；Windows 说明 MSVC+OpenSSL 门槛并给出 WSL/Docker 替代）。

## 与 ADR-0007 的关系

本方案不是 ADR-0007 约束的对象（默认开启的内置探针），而是以不同方式满足其准入条件：

- 协议复杂度不住在 anchorscan 的 Go 代码里——不存在「半实现的 RDP 会话」，检测实现是经实战验证的第三方二进制，与 nuclei/nmap 同等地位。
- 非默认开启：操作者必须显式安装并配置 rdpscan 才启用，安装动作本身即知情同意。
- 缺口证明成立：NSE 无脚本、nuclei 官方拒绝收录（见 Problem Statement）。
- 安全语义：rdpscan 仅做无认证握手探测，无 Crash/Exploit/Payload/认证尝试/目标写入路径；作者声明无法 100% 排除蓝屏风险，本 spec 要求在各层如实披露该残余风险（见 Safety Contract）。
- 隔离环境验证：lab 无 Windows RDP 靶机，验证以 fake Runner fixture 替代（见 Testing Decisions）。

## Safety Contract

- rdpscan 为可选工具；未配置 = `skipped/tool_unconfigured`，绝不阻塞或失败扫描。
- 触发条件只看服务指纹归一化结果，不因开放 3389 端口单独触发。
- 只映射明确结论：`VULNERABLE` 才产出 Finding；`SAFE`/`UNKNOWN` 不产出阴性结论或漏洞 Finding，只在 DetectionCheck/Artifact 中保留事实。
- 执行服从 Run 取消与可配置超时（`timeouts.rdpscan`，与其他工具一致默认 `0`；rdpscan 自身有内部超时）。
- 进度事件与报告中标注该检测为主动探测，存在极低概率目标蓝屏的残余风险。

## Implementation Decisions

- config：`ToolPaths.Rdpscan` + `detectToolPath("rdpscan")` + `ToolTimeouts.Rdpscan`（与其他工具一致默认 `0`）；不新增 `rdpscan_args` 字段，因为该引擎不需要额外参数。
- `internal/tools/rdpscan.go`：`RunRdpscan(ctx, runner, binaryPath, ip, port)` 与 `ParseRdpscanOutput`，形状对齐 `nuclei.go`。
- `scan_target.go`：在 nuclei 段之后追加 rdpscan 段，门控 `fp.Normalized == "rdp"`；跳过原因复用 `no_matching_rule` / `tool_unconfigured` 语义。NSE、nuclei、rdpscan 在同一指纹上可全部执行，互不抑制。
- DetectionCheck：`engine="rdpscan"`；Finding：`Source="rdpscan"`，severity critical，标题引用 CVE-2019-0708。
- Artifact：rdpscan 原始输出落盘（`safeArtifactName("rdpscan", ip, port, ...)`）。
- `internal/preflight/preflight.go`：rdpscan 作为可选工具检查；未配置只产生 warning，不阻塞扫描。
- doctor：rdpscan 未安装仍返回 `OK=true`，避免可选工具误报 doctor 失败；缺失时按 `runtime.GOOS` 给出编译/替代指引，特判实现，不做通用提示框架。
- `cmd/anchorscan/scan_command.go`：预检日志输出包含 `rdpscan` 超时。
- `internal/web/config.go` + `templates/config.html`：新增 rdpscan 路径与超时输入，避免 Web 保存时覆盖该字段。
- 报告 Detection Coverage 汇总自然覆盖新引擎；`vulnerability_delivery.go` 对 `rdpscan` source 使用 `ToolUnknown` 默认分支，CVE 由既有正则提取，不新增分支（不生成复测命令）。
- 同步更新 CONTEXT.md（Finding Source 新值、rdpscan 引擎）与 CHANGELOG。

## Testing Decisions

- `internal/tools` seam 使用 fake Runner：三种输出 fixture（VULNERABLE / SAFE / UNKNOWN）+ 错误退出，TDD red-green。
- `scan_target` 集成测试：fake Runner + 临时 SQLite + 受管 Artifact 目录，覆盖触发门控（rdp 命中 / 非 rdp 跳过 / 未配置跳过）、VULNERABLE 产出 Finding、UNKNOWN 不产出 Finding。
- doctor 测试覆盖 rdpscan 缺失时的分平台提示文案。
- 不需要真实 RDP 目标；真实验证由操作者按 doctor 指引装好后在内网实测。

## Out of Scope

- 复活内置 Go 探针、任何形式的 MSF 集成（CLI 或 msfrpcd）。
- rdpscan 预编译二进制分发、CI 打包；用户自行编译。
- BKScan / NLA 变体、其他 POC 检测器、通用「POC 引擎」抽象。
- 用 SAFE/UNKNOWN 结果宣称主机安全或补丁状态。

## Review Fixed Point

`main @ 94093aef2512894992544a69d8e2b2896771e125`（worktree `new-Anchor-rdpscan`，分支 `feat/rdpscan-bluekeep`）。
