# ADR-0007：在出现合格候选前暂缓默认 Builtin Probe

- 状态：Accepted
- 日期：2026-07-20
- 关联计划：`docs/plans/add-builtin-vulnerability-probes/`
- 替代：ADR-0006

首版默认开启的 Builtin Probe 必须同时满足：按服务指纹触发、无认证、没有持久化状态变化、不读取敏感数据，并补足 Nmap NSE 与 ProjectDiscovery nuclei 的实际检测缺口。

复核 BlueKeep 的 Rapid7 `Scan` 实现后确认，它需要完整 RDP 会话、Client Info 与许可相关交互；加之官方模块将副作用标为未知，不能满足上述默认启用的证明门槛。新的候选调研也未找到同时通过安全与覆盖门槛的候选。

因此首版只交付已经完成的逐探针 DetectionCheck 数据模型，不默认执行、也不宣称覆盖具体漏洞。未来候选必须先在调研记录中提供协议或产品维护方的安全语义、固定且非敏感的判定响应、NSE/nuclei 缺口与隔离环境验证，再新建替代票据。
