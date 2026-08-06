# Ticket 03 验收报告：canonical command 与 Nuclei `-code`

- 日期：2026-08-05
- 范围：`docs/ticket-03-brief.md`、`docs/ticket-03-approval.md`、catalog v2 spec/ticket 03
- 风险等级：中等（跨 knowledgebase parser、report 命令绑定、项目候选命令与固化 producer 产物）

## 改动摘要

- JSON consumer 的正式顶层来源改为 `source: "handbook-v3"`；spec 4.1 已勘误，ticket 01 与 ticket 03 均标记为 `done`。
- loader 只校验 canonical command：Nuclei 的 `-code` 必须且只能在 `nuclei` 后一次出现，且与 `verify.code` 双向一致；Nmap/MSF 的 `verify.code` 继续拒绝。
- `BuildNucleiCommand` 保留受限的 `-code` 并在绑定后拒绝残留占位符。
- 多资产项目候选不再把首个目标伪装成 target file：每个选中资产生成一条已绑定的 Nuclei 命令，均保留 `-code`。
- 新增冻结的真实 producer artifact `internal/knowledgebase/testdata/catalog-v2-real.json`，并在 README 记录来源、commit、协议、SHA-256 与获取日期。

## TDD 证据

1. **Red（source 与真实产物）**：把 reduced fixture 改为 `handbook-v3`、添加真实 artifact 与 `-code` 正反例后，原 loader 仍只接受 `handbook-v2`；`go test ./internal/knowledgebase ./internal/report` 以 `EXIT=1` 失败，真实 fixture 和 report 层均报告顶层协议无效。
2. **Green**：将 consumer 常量改为 `handbook-v3`，收紧 `validNucleiJSON`，并使 `BuildNucleiCommand` 接受仅位于第二个 argv 的可选 `-code`；聚焦测试通过。
3. **Red（项目候选）**：两资产 code 候选测试发现原实现只返回首个目标并把其地址放入 `target_file`：`go test ./internal/report -run '^TestBuildCandidateNucleiCommandPreservesCode$'` 以 `EXIT=1` 失败。
4. **Green**：项目候选改为对每个已选资产调用同一严格 binder；聚焦测试以 `EXIT=0` 通过。

## 实测：真实产物与必需验收

### 真实 producer artifact

只读复制并复验以下上游文件，未修改上游工作区：

```text
来源：~/DEV/Pentest-Playbook/handbook-v3/dist/catalog.json
commit：57d739e (57d739efcc77427b323d6233b016bd3035843834)
fixture SHA-256：7d8ce203a503f63b8d733e6c07fa10c2f1bbb1daf4d5c0619b61e553f374224e
cmp：一致
version=2 source=handbook-v3 entries=188 commands=145
code=cve-2026-24061,cve-2017-7529
```

`TestLoadJSONRealCatalogV2AcceptsAllEntriesAndCodeCommands` 断言上述 188 条均被 loader 接受、145 条命令投影完整，并逐条断言两个真实 `nuclei -code` 条目的 `verify.code` 与 canonical command 一致。

### 必需命令

```bash
go test ./internal/knowledgebase/... ./internal/report/...
```

结果：`EXIT=0`

```text
ok  github.com/P0m32Kun/anchorscan/internal/knowledgebase  0.170s
ok  github.com/P0m32Kun/anchorscan/internal/report          0.596s
```

```bash
go build ./...
```

结果：`EXIT=0`（无输出）。

另外，修改过的 5 个 Go 文件均通过 LSP diagnostics（0 diagnostics），并执行 `gofmt` 与 `git diff --check`（均无输出）。

## 模拟 / 未执行真实扫描器

- 所有 Go 测试均是 unit seam；绑定目标使用 TEST-NET 地址，没有启动 Nuclei、Nmap 或 MSF，也没有执行上游命令。
- `TestLoadJSONRejectsInvalidNucleiCode` 覆盖重复、错位、`verify.code` 缺失和意外 `-code`；`TestLoadJSONRejectsNmapAndMSFCode` 覆盖 Nmap/MSF 的 code 拒绝。
- `TestBuildNucleiCommandBindsRealCodeEntries` 使用固化的两条真实 producer 条目验证 host:port 与 URL 绑定；批量测试验证 `-code`、两个目标及无残留占位符；项目候选测试验证两个资产均保留 `-code` 且不丢目标。
- 既有 knowledgebase/report 测试随验收命令执行，覆盖 legacy Markdown 仍为 fail-closed 以及 Nmap/MSF 的现有正反例。

## 审查与剩余项

两轴只读审查先发现：(1) 本报告缺失；(2) 多资产项目候选会丢失第二个目标。两项均已在本次改动中修正；未发现 UI 确认逻辑、未来占位实现或 Nmap/MSF 放宽。

未运行 `make test`、`make pr-check`、Playwright 或真实工具 Docker E2E：本 ticket 未改 UI/打包/扫描器执行，且任务书指定的 Go package 测试和全量 build 已实测。未执行 git commit 或 push。
