# 03 — 建模并执行授权 Scan Scope

**What to build:** 用一个 Scan Scope 模块统一解析 include/exclude、限制危险范围并保证被排除地址不会进入任何扫描阶段。

**Blocked by:** 02 — 对运行时资源给出可执行诊断。

**Status:** done

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

- [x] 先写 `/24` 排除单 IP 仍被扫描的失败测试。
- [x] 使用 `net/netip` 完成最小 Scope 类型和 membership 判断。
- [x] 覆盖排除子网、IPv6、重复和规范化。
- [x] 覆盖以 `-` 开头输入和超大 Scope 在 Runner 前失败。
- [x] 更新 CONTEXT 中 Target/Scope 术语及配置 snapshot。
- [x] 聚焦测试、`make test`、`go vet ./...`、相关 Web smoke 通过。

## 验收记录

- Scope 用紧凑 `net/netip.Prefix` include/exclude 表示，执行前估算并限制至 4096 地址；CIDR 不展开。
- Nmap discovery 按 IPv4/IPv6 地址族拆分，回传地址、服务 fingerprint、HTTPX URL、NSE/Nuclei/RDP 调用与所有 Nuclei 证据端点均在持久化前受 Scope 约束。
- CLI 与 Web 都经 `PrepareScan` 构造稳定 Scope snapshot；CLI 支持 `--exclude-targets`。
- raw tool args 为严格 allowlist；Manager/RunScan 在获取 lease 前校验；Nmap 缺失时 CIDR/exclusion Scope fail-closed。
- `make test`、`go vet ./...`、`make pr-check`、修改 Go 文件 LSP diagnostics 均通过；最终 Standards/Spec 双轴 review 无 blocker/high。

## 非目标

- 不支持任意 Nmap target expression。
- 不展开全网段，不增加资产数据库或 DNS inventory。
