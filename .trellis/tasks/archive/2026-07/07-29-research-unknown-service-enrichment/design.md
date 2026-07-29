# 未识别服务增强研究设计

研究将 `unknown`、空服务与 `tcpwrapped` 分开处理。比较 nmap 版本探测调整、httpx、有限协议专用探针、Nuclei network/http tags；拒绝对全部未知端口广泛跑 tags，也不泛化 Dameng 握手。

推荐方案必须定义候选条件、每端口请求数、连接/读取超时、并发与全 Run 预算、重试/停止条件、授权要求、最小成功证据、置信度/冲突/provenance。用 fixture 或受控 loopback 验证 unknown、tcpwrapped、空及已知服务；无足够证据则推荐不实施。
