# ADR-0006：采用 Rapid7 BlueKeep Scan 作为安全行为基线

- 状态：Accepted
- 日期：2026-07-20
- 关联计划：`docs/plans/add-builtin-vulnerability-probes/`
- 替代：ADR-0005

首个默认 Builtin Probe 采用 Rapid7 官方 `auxiliary/scanner/rdp/cve_2019_0708_bluekeep` 的 `Scan` 分支作为工程安全基线。依据不只是模块位于 `auxiliary`：官方将默认动作设为 `Scan`、标记 `CRASH_SAFE`，并明确说明该分支发送合法长度的 non-DoS 检查报文。模块同时存在的 `Crash` 动作和 `UNKNOWN_SIDE_EFFECTS` 说明目录分类本身不足以证明安全，因此 AnchorScan 必须施加更窄的实现约束。

实现只复刻固定的 `Scan` 协议状态机和判定语义，不提供动作选择器，也不包含、生成或隐藏 Crash、DoS、Exploit、Payload、内存 grooming、shellcode 或可达性重试路径。所有出站协议报文由 golden fixture 做字节级允许列表审计；只有结构正确的 MCS Disconnect Provider Ultimatum 才能产生 `vulnerable`。发布前仍必须在隔离实验室验证未修补、已修补和 NLA 目标，并确认重复扫描不会导致服务异常。
