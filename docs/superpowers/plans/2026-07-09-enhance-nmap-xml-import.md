---
change: enhance-nmap-xml-import
design-doc: docs/superpowers/specs/2026-07-09-enhance-nmap-xml-import-design.md
base-ref: fa9b0590aadaca242a342db9912d6388cc37be38
---

# Enhance Nmap XML Import 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `anchorscan import-nmap` 命令，把已有 Nmap XML 导入为完成态 AnchorScan run，并以 `ip + port + protocol` 为服务身份保留 protocol、CPE、NSE script 输出与 scope，复用现有 SQLite、JSON/HTML 报告和 Web Console 链路。

**Architecture:** 轻量增强（design 决策 B），不引入 Python/独立前端。改动落在现有模块边界内：解析器（fingerprint）、持久化（store migration v3 + 事务化导入）、报告模型（report + web 的 portKey 改造）、CLI/app 编排（import-nmap 命令）。失败路径采用事务，保证不留半截 run。

**Tech Stack:** Go 1.26，`encoding/xml`（标准库），`modernc.org/sqlite`（纯 Go，无 CGO），标准库 `flag` CLI，内联 Go 字符串 fixture。

**Design Doc:** `docs/superpowers/specs/2026-07-09-enhance-nmap-xml-import-design.md`
**OpenSpec tasks:** `openspec/changes/enhance-nmap-xml-import/tasks.md`
**Canonical spec:** `openspec/changes/enhance-nmap-xml-import/specs/nmap-xml-import/spec.md`

## 代码现实校准（与 design 的差异）

design 写作时假设 protocol/CPE/ExtraInfo/Tunnel 都需新增。探索代码后的真实情况：

| 字段 | 内存模型 `ServiceFingerprint` | 解析器 `nmap_xml.go` | 数据库 | 报告 |
|------|------------------------------|---------------------|--------|------|
| `Protocol` | ✅ 已有 (`model.go:6`) | ✅ 已解析 (`nmap_xml.go:22,59`) | ❌ 未落库 | ❌ 未输出 |
| `ExtraInfo` | ✅ 已有 (`model.go:10`) | ✅ 已解析 (`nmap_xml.go:36,63`) | ❌ 未落库 | ❌ 未输出（仅 `Classify` 用） |
| `Tunnel` | ✅ 已有 (`model.go:11`) | ✅ 已解析 (`nmap_xml.go:37,64`) | ❌ 未落库 | ❌ 未输出（仅 `Classify` 用） |
| `CPE` | ❌ 不存在 | ❌ 不解析 | ❌ 无列 | ❌ 不输出 |
| NSE script | ❌ 不存在 | ❌ 不解析 | ❌（findings 表可承载 source/output） | — |

**结论：** protocol/ExtraInfo/Tunnel 不是"新增字段"，而是"打通已有内存字段到落库+报告"。CPE 和 NSE script 才是真正新增。

**最关键的改动点（风险最高）：** 服务身份当前 = `ip + port`。`report.Build` 的 `portKey(ip, port)` (`report/model.go:90`)、web 的 `findingMatchesService` (`web/reports.go:147`)、`exportFindingsCSV` (`web/reports.go:315`) 都用 `ip:port` 做 key。TCP/UDP 同端口会被合并/互相覆盖。**必须把 portKey 改成 `ip:port:protocol`**，牵涉 report 和 web 两个包，需配套回归测试。

## Global Constraints

- 保持单二进制交付，不引入 Python/FastAPI 或独立前端。
- 不复制或读取 `nmap_viewer.db`；不迁移完整 `risk_rules.json`。
- migration v3 必须向后兼容：`ALTER TABLE ... ADD COLUMN ... DEFAULT`，容错 `duplicate column name`（仿 `migrations.go:82,93`）。旧 run 必须继续可读。
- 导入写库必须事务化：校验和解析在事务前完成，落库任一步失败回滚，不留下半截 run（spec 硬性要求）。
- 沿用仓库内联 XML 字符串 fixture 风格（无 testdata 目录）。
- 不改 user-local 工作流文件（`.codex/`、`.comet/`、`openspec/`、`skills-lock.json`）。
- 每个任务完成后跑相关测试；全部完成后跑 `go test ./...`。
- 用中文撰写面向用户的文案（错误信息、帮助文本），与现有 zh-CN 配置一致。

---

## Phase 1 — Parser and Model（对应 tasks 1.1–1.3）

### Task 1.1 — 扩展解析器数据结构与 CPE/NSE 解析

**文件：** `internal/fingerprint/nmap_xml.go`、`internal/fingerprint/model.go`

**为什么先做：** 解析器是数据源，下游 store/report/CLI 都依赖它产出的结构。

**改动：**

1. `model.go`：`ServiceFingerprint` 增加 `CPE string` 字段（按 design，先用字符串承载，多个 CPE 用换行或分号分隔）。
2. `nmap_xml.go` 结构体扩展（基于现有 71 行增量改）：
   - `nmapRun` 增加 `Prescripts []nmapScript \`xml:"prescripts>script"\`` 和 `Postscripts []nmapScript \`xml:"postscripts>script"\``（nmaprun 直接子节点）。
   - `nmapHost` 增加 `Hostscripts []nmapScript \`xml:"hostscript>script"\``。
   - `nmapPort` 增加 `Scripts []nmapScript \`xml:"script"\``（port-level）。
   - 新增 `nmapScript` 结构：`ID string \`xml:"id,attr"\``、`Output string \`xml:"output,attr"\``。
   - `nmapPort` 增加 `CPEs []string \`xml:"cpe"\``（nmap XML 里 `<cpe>` 是 `<port>` 的子节点，与 `<service>` 兄弟；可多个）。

**验收：** 结构体编译通过（此任务不改 `ParseNmapXML` 函数体，留给 1.2）。

### Task 1.2 — 增强 ParseNmapXML：CPE、script 输出与 scope、错误路径

**文件：** `internal/fingerprint/nmap_xml.go`（`ParseNmapXML` 函数）、`internal/fingerprint/model.go`

**改动：**

1. `model.go`：新增 `ImportedScript` 结构承载 NSE script（`Scope string`、`IP string`、`Port int`、`Protocol string`、`ID string`、`Output string`）。Scope 取值：`port`/`host`/`pre`/`post`。
2. `ParseNmapXML` 函数签名改为返回 `([]ServiceFingerprint, []ImportedScript, error)`（向后看：导入编排需要 script 列表转 findings）。现有调用方 `internal/tools/nmap.go:26` 同步更新（只取第一个返回值，忽略 script，用 `_`）。
3. 函数体：
   - 填充 `ServiceFingerprint.CPE`（把 `port.CPEs` 合并为字符串）。
   - port-level script → `ImportedScript{Scope:"port", IP:host.Addr, Port:portID, Protocol:port.Protocol, ID, Output}`。
   - hostscript → `ImportedScript{Scope:"host", IP:host.Addr, Port:0, ...}`。
   - prescript/postscript → `ImportedScript{Scope:"pre"/"post", IP:"", Port:0, ...}`。
4. 错误路径（在 `xml.Unmarshal` 前后增加校验）：
   - 空数据 → `errors.New("empty XML file")`。
   - 根节点非 `nmaprun` → `errors.New("root element is not nmaprun")`。检测方法：先 `xml.Unmarshal` 进一个只含根元素名的探针结构 `{XMLName xml.Name}`，校验 `Local == "nmaprun"`，再正式解析。
   - XML 语法错误 → 保留 Unmarshal 原始 err，包成 `fmt.Errorf("invalid Nmap XML: %w", err)`。

**验收：** 解析器单元测试（1.3）全过；现有 `tools/nmap.go` 调用方编译通过。

### Task 1.3 — 解析器单元测试

**文件：** `internal/fingerprint/nmap_xml_test.go`

**改动：** 沿用内联 XML 字符串风格，补以下 case（每个一个测试函数或表驱动）：

- TCP/UDP 同端口（`53/tcp` + `53/udp`）→ 两个 ServiceFingerprint，protocol 分别为 tcp/udp。
- CPE 值（`<service>` 后多个 `<cpe>`）→ `CPE` 字段含合并值。
- port-level `<script id="http-methods" output="...">` → ImportedScript{Scope:"port", Port 对应}。
- hostscript `<hostscript><script id="ssh-hostkey">` → ImportedScript{Scope:"host"}。
- postscript → ImportedScript{Scope:"post"}。
- 空 `[]byte{}` → error 含 "empty XML file"。
- 非 nmaprun XML（如 `<foo/>`）→ error 含 "root element is not nmaprun"。
- 非法 XML（`<nmaprun><unclosed>`）→ error 含 "invalid Nmap XML"。
- 保留现有 `TestParseNmapXMLExtractsServiceFields` 并适配新返回签名。

**验收：** `go test ./internal/fingerprint/...` 全过。

### Task 1.4 — 报告模型加 protocol/CPE 并改 portKey

**文件：** `internal/report/model.go`、`internal/report/html.go`（内联模板）、`internal/web/templates/report.html`、`internal/web/reports.go`

**这是风险最高的任务（portKey 改造），单独成任务。**

**改动：**

1. `report/model.go`：
   - `PortReport` 增加 `Protocol string \`json:"protocol"\`` 和 `CPE string \`json:"cpe,omitempty"\``。
   - `Finding` 增加 `Protocol string \`json:"protocol,omitempty"\``（让导入的 NSE finding 携带 protocol 维度）。
   - `portKey` 签名改为 `portKey(ip string, port int, protocol string) string`，返回 `fmt.Sprintf("%s:%d:%s", ip, port, protocol)`。
   - `Build` 内 `findingsByPort` 用新 key（`portKey(finding.IP, finding.Port, finding.Protocol)`），并在构造 `PortReport` 时填 `Protocol: fp.Protocol`、`CPE: fp.CPE`。
2. `report/html.go` 内联模板：端口列渲染改为 `{{.Protocol}}/{{.Port}}`（或新增 protocol 列）；fingerprint/详情处展示 CPE。
3. `web/templates/report.html`：两套表格（端口列表视图 `report.html:192-216`、主机聚合视图 `report.html:148-167`）加 protocol 列。
4. `web/reports.go`：
   - `findingMatchesService`（`:147`）：增加 `fp.Protocol == item.Protocol` 判断。
   - `exportFindingsCSV`（`:312-324`）：services map key 改 `item.IP+":"+port+":"+Protocol`；CSV 表头加 `protocol`/`cpe`。
   - `exportAssetsCSV`（`:287`）：表头加 `protocol`/`cpe`，行加对应字段。

**验收：**
- `go test ./internal/report/... ./internal/web/...` 全过。
- 新增/更新测试：TCP/UDP 同端口导入后，`Build` 产出两个独立 PortReport（不合并）；findings 按 protocol 正确挂载。
- 现有报告/web 测试（无 protocol 的旧数据，protocol 默认 `""`）仍通过——需确认 key 不破坏（空 protocol 的 `ip:port:` 仍唯一）。

---

## Phase 2 — Persistence（对应 tasks 2.1–2.3）

### Task 2.1 — migration v3：扩展 fingerprints/findings 列

**文件：** `internal/store/migrations.go`、`internal/store/sqlite_test.go`

**改动：**

1. `migrations.go`：`migrations` slice 追加 version 3，name `add_import_fields`，`up func` 内用 `ALTER TABLE ... ADD COLUMN ... DEFAULT` 加列，容错 `duplicate column name`：
   - fingerprints：`protocol TEXT NOT NULL DEFAULT ''`、`cpe TEXT NOT NULL DEFAULT ''`、`extrainfo TEXT NOT NULL DEFAULT ''`、`tunnel TEXT NOT NULL DEFAULT ''`。
   - findings：`protocol TEXT NOT NULL DEFAULT ''`、`scope TEXT NOT NULL DEFAULT ''`。
2. `sqlite_test.go`：`TestOpenMigrationsAreIdempotent` 的 count 断言 `2 → 3`；`TestOpenMigratesLegacyDatabase` 确认 v3 列存在且默认值正确。

**验收：** `go test ./internal/store/... -run Migrat` 全过；旧库（只有 v1/v2）打开后新列有默认值，旧数据可读。

### Task 2.2 — store 存取方法扩展新列

**文件：** `internal/store/sqlite.go`

**改动：**

1. `SaveFingerprint`（`:41`）：INSERT 列增加 `protocol, cpe, extrainfo, tunnel`，VALUES 与参数同步加 `fp.Protocol, fp.CPE, fp.ExtraInfo, fp.Tunnel`。
2. `ListFingerprints`（`:50`）：SELECT 列增加 `protocol, cpe, extrainfo, tunnel`（顺带把当前漏选的 `version` 一起选上，Scan 对齐）；`ORDER BY ip, port` 改 `ORDER BY ip, port, protocol`（保证 TCP/UDP 同端口顺序稳定）；Scan 填入对应字段。
3. `SaveFinding`（`:76`）：INSERT 增加 `protocol, scope`，参数加 `finding.Protocol`、新传 scope（finding 结构在 1.4 已加 Protocol；scope 通过 finding 的 Source/ID 编码或单独参数——见下）。
4. `ListFindings`（`:85`）：SELECT 增加 `protocol, scope`，Scan 对齐。

**关于 scope：** design 决策是 hostscript/prescript/postscript 的 scope 编码进 finding 的 `Source` 或 `ID`（如 `nmap-import:hostscript:ssh-hostkey`）。`findings.scope` 列作为可选增强，若 finding 模型不便加 Scope 字段，则 scope 一律编码进 `Source`，列留空默认。**实现时优先用 Source 编码方案（与 brainstorm-summary 一致），`scope` 列预留为空。**

**验收：** `go test ./internal/store/...` 全过；新增测试：存 `53/tcp` 和 `53/udp` 两条 fingerprint，ListFingerprints 返回两条且 protocol 正确区分。

### Task 2.3 — 事务化导入方法

**文件：** `internal/store/import.go`（新文件）或 `internal/store/runs.go`

**改动：** 新增事务化方法，把"建 completed run + 批量 fingerprints + 批量 findings"包进单 `tx`，仿 `projects.go:124` 的 `tx.Begin()`/`Rollback`/`Commit` 模式。建议签名：

```go
func (s *Store) SaveImportRun(run ScanRun, fps []fingerprint.ServiceFingerprint, findings []report.Finding) error
```

- 内部 `tx, err := s.db.Begin()`。
- 在 tx 内：INSERT run（复用 SaveScanRun 的 SQL，但用 tx.Exec）、循环 INSERT fingerprints、循环 INSERT findings。
- 任一失败 `return err`（defer Rollback 兜底）；全部成功 `return tx.Commit()`。
- run 的 Status 由调用方设为 `"completed"`，StartedAt/FinishedAt 同一时刻。

**验收：** `go test ./internal/store/... -run Import` 全过；新增测试：中途构造失败（如 nil fp 或超长字段）确认事务回滚，DB 无新增 run/fingerprint。

---

## Phase 3 — Import Command（对应 tasks 3.1–3.3）

### Task 3.1 — CLI 命令注册与 flag

**文件：** `cmd/anchorscan/main.go`

**改动：**（以 `runReport` `main.go:396` 为样板）

1. `switch args[0]`（`main.go:65-86`）加 `case "import-nmap": return runImportNmap(args[1:], stdout, stderr, deps)`。
2. `printRootHelp`（`main.go:605`）命令列表加 `import-nmap  Import an existing Nmap XML into an AnchorScan run`。
3. 新增 `runImportNmap(args, stdout, stderr, deps)`：
   - `flag.NewFlagSet("import-nmap", flag.ContinueOnError)` + `fs.SetOutput(io.Discard)`。
   - flag：`--config`（默认 `config/default.yaml`，保持一致）、`--xml`（必填）、`--db`（默认 `data/scans.sqlite`）、`--run-id`（可选，默认空，由 app 生成 `import-<ts>`）、`--project`（可选，默认空）、`--json`（可选）、`--html`（可选）。
   - 处理 `-h` → `printImportNmapHelp`。
   - 校验 `--xml` 非空，否则 `errors.New("--xml is required")`。
   - `deps.openStore(*dbPath)` 开库。
   - 调 `app.ImportNmap(ctx, scanStore, opts)`（3.2）。
   - 成功输出 `fmt.Fprintf(stdout, "run_id=%s\n", runID)`。
4. 新增 `printImportNmapHelp`（仿 `printReportHelp` `main.go:671`）。

**验收：** `anchorscan import-nmap -h` 显示帮助；未传 `--xml` 报清晰错误。

### Task 3.2 — 导入编排 app.ImportNmap

**文件：** `internal/app/import.go`（新文件）、`internal/app/import_test.go`（新文件）

**改动：** `ImportNmap(ctx, scanStore, opts ImportNmapOptions) (runID string, err error)`

1. 读 XML 文件（`os.ReadFile`），文件不存在/不可读返回错误。
2. `fingerprint.ParseNmapXML(data)` → fps + scripts（含 CPE/NSE）。校验失败（空/非 nmaprun/非法）直接返回错误，**不碰数据库**。
3. 对每个 fp 调 `fingerprint.Classify`（复用现有 web/URL 推断）。
4. script → finding 映射：
   - port script → `report.Finding{IP, Port, Protocol, Source:"nmap-import:port:<id>", ID:<id>, Output:<output>, Severity:"info"}`。
   - host script → `Source:"nmap-import:hostscript:<id>"`, Port:0。
   - pre/post script → `Source:"nmap-import:postscript:<id>"` / `:prescript:`。
5. 生成 runID（`opts.RunID` 非空用之，否则 `import-` + `time.Now().Format("20060102-150405")`，仿 `main.go:209`）。
6. 构造 `ScanRun{RunID, ProjectID:opts.Project, Target:"nmap-import", Status:"completed", StartedAt:now, FinishedAt:now, ConfigSnapshot: <JSON of import meta>}`。
7. `scanStore.SaveImportRun(run, fps, findings)`（事务，2.3）。
8. 若 `opts.JSONPath` 非空：`ListFingerprints` + `ListFindings` → `report.Build` → `report.WriteJSON`（复用 `main.go:433-449` 模式，`ensureParentDir`）。
9. 若 `opts.HTMLPath` 非空：同上 → `report.WriteHTML`。

**验收：** `go test ./internal/app/... -run Import` 全过。

### Task 3.3 — 失败路径保证不留半截 run

**文件：** `internal/app/import.go`（由 3.2 承载）、测试覆盖

**改动：** 3.2 已保证校验在事务前；此任务专门加测试验证 spec 的失败场景：

- 空 XML → ImportNmap 返回 "empty XML file" 错误，DB 无新增 run。
- 非 nmaprun XML → 返回 "root element is not nmaprun"，DB 无新增 run。
- 非法 XML → 返回 "invalid Nmap XML"，DB 无新增 run。
- （事务回滚）构造落库失败场景（可选：mock store 或超大字段），确认 `SaveImportRun` 回滚。

**验收：** 测试断言 `ListScanRuns` 在失败前后数量不变。

---

## Phase 4 — Verification and Docs（对应 tasks 4.1–4.3）

### Task 4.1 — CLI 集成测试

**文件：** `cmd/anchorscan/main_test.go`（仿 `TestExecuteReportWritesHTMLFromStoredRun` `main_test.go:433`）

**改动：** 用内联 XML fixture（含 `53/tcp` + `53/udp` + CPE + port script + hostscript + postscript），写临时文件，调 `run([]string{"import-nmap", "--xml", fixture, "--db", tempDB, "--json", tempJSON, "--html", tempHTML})`：

- 成功：stdout 含 `run_id=`；tempJSON 和 tempHTML 文件存在；重新 `store.Open` 读 DB，`ListFingerprints` 返回两条 53 服务（tcp/udp 各一）；`ListFindings` 含 NSE finding。
- 空 XML：`run` 返回错误，DB 无新增 run。
- 非 nmaprun XML：同上。
- 帮助测试：`run([]string{"import-nmap", "-h"})` 输出含 `--xml`/`--db`/`--json`/`--html`。

**验收：** `go test ./cmd/anchorscan/... -run Import` 全过。

### Task 4.2 — 文档更新

**文件：** `README.md`（或 `docs/` 下命令文档）

**改动：** 说明 `anchorscan import-nmap` 用法、flag、典型示例，并明确非目标（不引入 Python、不读 nmap_viewer.db、不迁移 risk_rules、本期不做 Web 上传）。

**验收：** 文档清晰，命令示例与实现一致。

### Task 4.3 — 全量验证与实跑

**改动：**

1. `go test ./...` 全绿。
2. `go build ./cmd/anchorscan` 编译通过。
3. 用 fixture XML 实跑一次 `./anchorscan import-nmap --xml <fixture> --db /tmp/import.sqlite --json /tmp/import.json --html /tmp/import.html`，确认 run 创建、报告生成、53/tcp 与 53/udp 分别可见。
4. （可选）`make test`。

**验收：** 所有命令成功，输出符合预期；tasks.md 对应条目可勾选。

---

## 任务依赖与执行顺序

```
Phase 1 (parser/model/report) 是下游基础，先做。
  1.1 → 1.2 → 1.3（解析器，自洽）
  1.4（report portKey 改造，依赖 1.2 的 CPE/Protocol 字段，可并行于 1.3）
Phase 2 (persistence) 依赖 Phase 1 的模型字段。
  2.1（migration）→ 2.2（store 存取）→ 2.3（事务导入）
Phase 3 (import command) 依赖 Phase 1 + 2。
  3.1（CLI）+ 3.2（编排）→ 3.3（失败路径）
Phase 4 (verification) 依赖 Phase 3。
  4.1（集成测试）→ 4.2（文档）→ 4.3（全量验证）
```

**推荐 TDD 顺序：** 每个任务先写/改测试（红），再实现（绿），再重构。尤其 1.3（解析测试）、2.1（migration 测试）、4.1（CLI 集成测试）适合先行。

## 关键风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| portKey 改造波及 report + web，旧数据 protocol 为空 | 高 | 空 protocol 的 key `ip:port:` 仍唯一；现有报告/web 测试需全部回归通过；先确认旧测试在改 portKey 后仍绿 |
| `ListFingerprints` 历史漏选 version | 中 | 2.2 顺带理顺，但要确保 Scan 列顺序与 SELECT 严格对齐 |
| CPE 多值编码格式 | 低 | design 定为字符串；用换行或分号分隔，测试固定一种 |
| finding scope 编码方案 | 低 | 优先用 Source 编码（`nmap-import:hostscript:<id>`），scope 列预留 |
| 现有 `tools/nmap.go` 调用 ParseNmapXML | 中 | 1.2 改签名时同步更新该调用方，忽略 script 返回值 |
