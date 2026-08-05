# 03 — 支持 canonical 命令与 Nuclei `-code` 绑定

**What to build:** 让 AnchorScan 验证 producer 的 canonical command，而非重写 renderer；使 Nuclei 的单条和批量命令绑定正确保留受限的 `-code` 参数。

**Blocked by:** 02 — 建立 catalog v2 模型与双格式 loader。

**Status:** done

**Execution skills:** `tdd`、`implement`、`code-review`、`ponytail`。

## 行为契约

- JSON loader 不从 `verify` 拼接字符串；只校验 `command` 的受限语法及它与 `verify.tool` / `verify.code` 的一致性。
- Nuclei 只允许 `nuclei [-code] -t ... -u {{url|host:port}}`；`-code` 只能出现一次且紧跟 `nuclei`。
- `verify.code: true` 与 Nuclei `-code` 必须双向一致；Nmap/MSF 不得携带 code。
- `BuildNucleiCommand`、批量 Nuclei 命令与项目候选命令保留 `-code`，仍拒绝残留占位符与不一致批次。
- Nmap/MSF 的既有严格绑定不放宽。

## 测试 seam

- `internal/knowledgebase` command grammar unit；
- `internal/report` 单条/批量 command-binding unit；
- 使用 producer fixture 的两个真实 `-code` 条目。

## 验收

- [ ] loader 接受真实 `-code` 条目，拒绝重复、错位或与 verify 不一致的 `-code`。
- [ ] 单条及批量 Nuclei 输出都保留 `-code`，目标替换后没有未替换占位符。
- [ ] Nmap/MSF 既有正反例测试保持通过。
- [ ] `go test ./internal/knowledgebase/... ./internal/report/...` 通过。

## 非目标

- 不在这里增加 UI 确认；命令是否可以向用户返回由 Ticket 04 决定。
