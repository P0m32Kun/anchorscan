# ADR-0005：首版默认内置探针排除 BlueKeep

- 状态：Superseded by ADR-0006
- 日期：2026-07-20
- 关联计划：`docs/plans/add-builtin-vulnerability-probes/`

首版默认 Builtin Probe 只接收能够证明无认证、非破坏且不修改目标状态的协议交互。Rapid7 的 BlueKeep Scan 路径虽然不包含 Crash 分支，但仍建立 RDP 会话并进入 Client Info 与许可相关步骤；其官方模块也将副作用标为未知。因此它不能满足本项目默认开启探针的准入标准。

BlueKeep 从首版范围移除，原有 BlueKeep 实施票据不再推进。已完成的 DetectionCheck `check_id`/`verdict` 扩展保留，作为后续通过安全准入评审的探针基础。首个具体探针必须先通过第一方资料与本地测试证明其交互边界；如果没有合格候选，首版只交付数据模型，不伪造具体漏洞覆盖。
