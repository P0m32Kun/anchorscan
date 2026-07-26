# 03 — 建模并执行授权 Scan Scope

**What to build:** 用一个 Scan Scope 模块统一解析 include/exclude、限制危险范围并保证被排除地址不会进入任何扫描阶段。

**Blocked by:** 02 — 对运行时资源给出可执行诊断。

**Status:** ready-for-agent

**Execution skills:** `implement`、`tdd`、`code-review`、`ponytail`。

## 行为契约

- 支持规范 IPv4、IPv6 和 CIDR；拒绝空值、flag-like 输入和未承诺格式。
- exclusion 对 CIDR 内单 IP和子网生效，不是字符串精确删除。
- 不完整展开大 prefix；在运行外部命令前估算规模并应用安全上限。
- Nmap 参数和返回 alive 地址都经过 Scope 约束；后续 rustscan/httpx/detection 只能收到允许地址。
- Run 保存稳定 Scope snapshot，CLI 与 Web 使用相同 PrepareScan 路径。

## 测试 seam

- `ScanScope` unit：固定 IP/CIDR/IPv6 表格。
- App scan seam：fake Runner 证明被排除地址从不进入后续调用。
- CLI/Web 只补各一个入口契约，不重复集合运算矩阵。

## 验收

- [ ] 先写 `/24` 排除单 IP 仍被扫描的失败测试。
- [ ] 使用 `net/netip` 完成最小 Scope 类型和 membership 判断。
- [ ] 覆盖排除子网、IPv6、重复和规范化。
- [ ] 覆盖以 `-` 开头输入和超大 Scope 在 Runner 前失败。
- [ ] 更新 CONTEXT 中 Target/Scope 术语及配置 snapshot。
- [ ] 聚焦测试、`make test`、`go vet ./...`、相关 Web smoke 通过。

## 非目标

- 不支持任意 Nmap target expression。
- 不展开全网段，不增加资产数据库或 DNS inventory。
