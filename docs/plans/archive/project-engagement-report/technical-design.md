---
change: project-engagement-report
role: technical-design
spec: docs/plans/project-engagement-report/spec.md
---

# 项目级渗透测试与交付报告（HTML + DOCX）技术设计

## Current Seams

当前代码已有可复用基础：

- `Project` 与 `ScanRun.ProjectID` 已形成一对多存储关系；
- `ListProjectScanRuns` 已支持项目页列出 Runs；
- `VulnerabilityDelivery` 已按知识库 Entry ID 聚合非 `info` Findings，并按 host/port/protocol 去重；
- 报告筛选、批量 IP:port、Nuclei/Nmap/MSF 命令生成和 `raw_args` 工具页预填已经可用；
- `DetectionCheck` 已持久化 NSE/nuclei 的运行事实；
- `report.WriteHTML` 已生成自包含 HTML。

需要深化的 seam 是“构建一个 Project Report”，而不是在每个 handler、模板和导出器中分别遍历所有 Runs。外部 Interface 应保持为一个纯构建入口，内部隐藏跨 Run 查询、Zone 分组、知识库匹配、Verification 投影、去重、排序和汇总：

```go
BuildProjectReport(ProjectReportInput) (ProjectReport, error)
```

HTML handler 和测试都只消费 `ProjectReport`。不要为 JSON、CSV、DOCX 各建一套聚合模型。

## Persistence Model

### projects

保留现有表和 ID。新增报告元数据字段或一张一对一 `project_reports` 表均可；首选 `project_reports(project_id PRIMARY KEY, client_name, report_title, test_subject, test_start, test_end, testers, conclusion, updated_at)`，避免继续膨胀 Project 的任务身份字段。

现有 `default_targets/default_ports/exclude_targets/exclude_ports/default_profile` 暂不删除，迁移后 Web 不再读取或写入它们。等所有调用和历史兼容完成后另行删除，避免同一 ticket 同时承担领域迁移与破坏性 schema 清理。

### project_zones

```text
id TEXT PRIMARY KEY
project_id TEXT NOT NULL
code TEXT NOT NULL
name TEXT NOT NULL
position INTEGER NOT NULL
UNIQUE(project_id, code)
```

新 Project 默认写入 I、II、III；其他 Zone 由用户显式添加。删除 Zone 前必须确认没有 Runs 或 Verifications。

### scan_runs

新增：

- `zone_id`：Project-owned Web Run 必填；旧数据允许空值并显示“未分区”；
- `kind`：`scan` / `tool`，旧数据按迁移时可确认的来源填充，无法确认时默认 `scan`；
- `label`：可读运行名称；
- `access_point`、`tester_ip`、`notes`；
- `include_in_report`：完整 completed/completed_with_errors Scan 默认 true，tool/failed/running 默认 false；用户可修改。

扫描参数继续保存在 Run 现有列与 `config_snapshot`，但 `scanForm` 需要加入 Zone、排除项和接入信息。`PrepareScanRequest` 无需因本功能抽共享摄入类型；ADR-0001 仍成立。

### report_verifications

```text
id TEXT PRIMARY KEY
project_id TEXT NOT NULL
zone_id TEXT NOT NULL
vulnerability_key TEXT NOT NULL
outcome TEXT NOT NULL
title TEXT NOT NULL
severity TEXT NOT NULL
description TEXT NOT NULL
remediation TEXT NOT NULL
notes TEXT NOT NULL
included INTEGER NOT NULL
position INTEGER NOT NULL
created_at TEXT NOT NULL
updated_at TEXT NOT NULL
```

`outcome` 只允许 `confirmed/not_observed/inconclusive`。`vulnerability_key` 对匹配项使用知识库 Entry ID；人工项使用 Verification 自有稳定键。报告字段保存确认时快照，避免知识库变更重写已确认交付内容。

关联资产使用 `verification_assets(verification_id, ip, port, protocol, asset_name, position)`；来源事实使用 `verification_sources(verification_id, run_id, source, finding_id, ip, port, protocol)`。不要引用 SQLite `rowid` 或可编辑标题作为身份。

### verification_evidence

```text
id TEXT PRIMARY KEY
verification_id TEXT NOT NULL
relative_path TEXT NOT NULL
media_type TEXT NOT NULL
sha256 TEXT NOT NULL
width INTEGER NOT NULL
height INTEGER NOT NULL
caption TEXT NOT NULL
position INTEGER NOT NULL
created_at TEXT NOT NULL
```

文件路径位于 `data/projects/<project-id>/evidence/<verification-id>/<evidence-id>.<ext>`。上传先写同目录临时文件，验证成功后原子重命名，再提交数据库记录；数据库失败时清理文件。删除 Evidence 不得触碰 `ArtifactDir`。

Evidence 数量按漏洞类型计：每条 included Verification 要求至少一张 Evidence，与其 `verification_assets` 行数无关。同一类漏洞影响多个 IP 时不追加截图要求；同一类验证项批量提交的多个端点共享同一组 Evidence。

## Project Aggregation

新增 store 查询一次返回被纳入 Runs 的带上下文事实：

```text
ProjectFinding { RunID, ZoneID, Finding }
ProjectFingerprint { RunID, ZoneID, ServiceFingerprint }
ProjectDetectionCheck { RunID, ZoneID, DetectionCheck }
```

项目构建器不得逐 Run 调 handler 或读取每个 `report.json`；SQLite 是聚合事实来源。只纳入用户选择且状态允许的 Runs。重复 Run 的结果按稳定身份去重，但保留来源 Run 列表供审计。

正向候选沿用 `ObservationFromFinding` 与 `Catalog.Match`。匹配项以 Entry ID 分组，待补充项沿用 pending key。资产键为规范化 host、数值 port、协议；表 3-1 的展示再投影为唯一 `IP:port`。

正式报告不直接消费原始候选，只消费 `included=true` 的 Verifications：

- `confirmed` → 汇总表与 Zone 漏洞详情；
- `not_observed` → Zone 未发现验证项；
- `inconclusive` → 工作台可见，但不进入正式统计。

## Negative Candidate Projection

对每个被纳入 Run 的 Fingerprint，使用 `(run_id, ip, port, protocol)` 查找 DetectionChecks 和 Findings：

- NSE 与 nuclei 都是 `completed`；
- 同端点没有 severity 非 `info` 的 Finding；
- 则进入负向验证候选。

候选不是持久化结论。用户必须选择具体知识库条目或人工验证项、勾选一个或多个同类候选端点、执行命令、保存至少一张共享 Evidence 并提交单条 `not_observed` Verification；该 Verification 的 `verification_assets` 关联全部选中端点。任一 DetectionCheck 缺失或非 completed 时相应端点进入检查未完成队列，不能被勾选。不要根据 `info` 文本或开放端口自动猜测具体漏洞。

## Web Flow

新增 Project 子路由，保持 Go `html/template` 与原生 JS：

- `/projects/{id}/runs/new`：Zone-aware 扫描表单；
- `/projects/{id}/verification`：项目级验证工作台；
- `/projects/{id}/verifications/{verification-id}`：保存结论/内容；
- `/projects/{id}/verifications/{verification-id}/evidence`：multipart 上传；
- `/projects/{id}/report`：正式报告预览；
- `/projects/{id}/report/export?format=html`：单文件 HTML；
- `/projects/{id}/report/export?format=docx`：模板填充 DOCX；

工具页接受并回传 `project_id/zone_id/verification_id/return_to`。禁止直接信任 `return_to` 的外部 URL；只允许本站相对路径。Tool Run 保存 `kind=tool` 与 Zone；Verification 仍由用户显式提交，不能因工具退出码为 0 自动确认结论。

Evidence 上传使用 `http.MaxBytesReader` 限制请求，按文件签名与 `image.DecodeConfig` 验证 PNG/JPEG，服务端生成 UUID 文件名，并通过 Verification→Project 关系校验路径归属。展示 Evidence 使用按 ID 查询的受控 handler，不暴露文件系统目录。

## HTML Report

新增 Project Report 模板；复用现有独立 HTML 的自包含样式原则，但不复用 Run `ScanReport` 作为数据模型。模板直接消费已验证的 Project Report：

- 元数据槽替换所有 `XX`；
- summary rows 动态输出表 3-1；
- Zones 按 position；
- confirmed Verifications 按 severity、position、title；
- Evidence 按 position，使用 data URI 内嵌，等比例限制到内容宽度；
- not_observed 单独输出；
- 结论数字从 included confirmed Verifications 计算。

工具与方法段从纳入 Runs 的 `config_snapshot`、DetectionChecks 和显式人工工具记录投影；不要把参考 DOCX 中的 Hscan、AWVS 7、CANVAS 等示例文本硬编码进新模板。危害统计支持 critical/high/medium/low，`info` 仅作为检测事实，不进入正式漏洞数量。

构建前执行完整性验证：缺失必填元数据、confirmed/not_observed 无 Evidence、Evidence 文件缺失或仍有未绑定占位符时返回结构化错误。HTML 转义统一交给 `html/template`；Evidence data URI 必须使用已验证媒体类型。该完整性验证对 HTML 与 DOCX 两个出口只执行一次，由 `BuildProjectReport` 前置完成。

## DOCX Export

DOCX 与 HTML 消费同一份 `ProjectReport`，不再单独建聚合模型。硬性要求是以用户标准模板为蓝本，保持未填充部分的版式和结构；已调研定案（ADR-0005；详见 `docx-rendering-research.md`，一手来源版）：使用模板填充，不走 Pandoc 等从文档模型重建正文的路线。

- **方案**：python-docx + docxtpl 作为 sidecar 进程，Go 主程序把 ProjectReport 序列化为 JSON context，经 `uv run --project tools/docx-render python render_docx.py ...` 启动。Python 运行时与依赖由 **uv** 管理，隔离在仓库内 `tools/docx-render/` 子项目（`pyproject.toml`、`.python-version=3.12`、`uv.lock`），不进入 Go 构建。doctor 必须运行同一条 `uv run --project tools/docx-render python -c "import docxtpl"` 检查；缺失时禁用 DOCX 导出并提示，不影响 HTML 导出。LGPL-2.1 的运行环境再分发义务由发布前合规流程确认。
- **模板制备**（一次性工序，产物入库受版本管理）：以当前简化 DOCX 为蓝本在 Word 里手工制备干净模板——同名异义的 `XX` 逐槽人工换成 Jinja 占位符（整词一次键入保证单 run）、表 3-1 保留 1 个数据模板行并增加独立的 `{%tr for %}` / `{%tr endfor %}` 控制行、删除 2 张正文示例截图（证据图走 InlineImage 插入）、补 `<w:updateFields/>` 让 Word 打开时刷新域。Zone 跨表格重复策略须由原型验证后确定，不预设 `{%p for %}` 可包住任意表格。完整地图见调研报告 §4。
- **保留清单**：模板中未放置占位符的样式、编号、主题、3 个页眉 part、7 个页脚 part（含 footer6 的浮动图形）、3 个 `sectPr` 与 TOC/PAGEREF/PAGE/STYLEREF 域结构须经渲染回归测试确认版式保持。`docxtpl` 会重新保存 DOCX 包，不能承诺未修改 ZIP/XML 字节不变。
- **降级路径**（仅当 sidecar 部署被否）：lukasjarosch/go-docx + 模板预制上限行数/区块数，缺口清单见调研报告 §5.3；不得临时自研 OOXML patcher。
- 渲染前完整性检查与 HTML 共用同一检查点：context 缺字段、模板残留 `XX`、confirmed/not_observed 无 Evidence 时导出失败并列出缺失项。

## Export Cleanup

删除的是用户可见格式和 handler 分支：`ExportJSON`、`ExportCSV`、finding CSV、asset CSV 与相应按钮。保留：

- 扫描/单工具流程内部 `WriteJSON` 和 `report.json`；
- CLI 兼容行为，除非另有明确 ticket；
- HTML 导出；
- 复制 IP/IP:PORT/URL 与需要供命令运行的 TXT 目标清单。

先完成 Project HTML 并验证，再移除旧下载入口，避免在新交付链路可用前切断现有工作流。

## Legacy Data and Gansu Migration

Schema migration 只给旧 Runs 设置空 Zone/默认 kind，不自动按 Project 名称合并。甘肃迁移使用一次性、显式参数的本地命令：默认只预览；`--apply` 前强制创建带时间戳的数据库和受管理目录备份，然后更新 ProjectID/ZoneID/include_in_report，并迁移受管理目录或保留兼容路径。旧 Projects 只有在无 Runs 且再次显式指定时才删除。本功能不建设通用 Web 合并 UI。

现有甘肃 Runs 没有 DetectionCheck 行，因此迁移后：

- 可跨 Runs 聚合已有非 `info` Findings；
- 可人工建立 confirmed Verification 与 Evidence；
- 不生成自动负向候选；需要重新运行相关检测或手工建立且明确标为历史人工验证。

## Testing Seams

按以下 seam 使用 TDD：

1. Store migration 与 Project/Zone/Run 持久化：旧数据可读，新 Web Run 必须有 Zone；
2. `BuildProjectReport`：跨 Run/Zone 聚合、稳定去重、Verification gating、表 3-1 投影和 XX 完整性；
3. Negative candidate：双 completed、info-only、非 info、缺失/失败/跳过/取消/中断矩阵；
4. Evidence HTTP seam：大小、格式、路径归属、排序、删除和失败清理；
5. Web workflow：候选筛选 → 命令 → 预填工具页 → 返回 Verification → Evidence → HTML/DOCX；
6. Export regression：HTML 与 DOCX 保留，用户 JSON/CSV/asset CSV 消失，内部 `report.json` 继续生成；
7. DOCX 渲染：docxtpl 填充后无残留占位、表 3-1 行动态、Evidence 图片嵌入、排版与原模板逐页一致、与 HTML 同源数据一致。

纯视觉样式不做脆弱 DOM 快照；用项目 fixture 生成正式 HTML，并通过浏览器检查三区、多 Runs、长资产列表、多 Evidence 与打印布局。

## Mature Components, Not Custom Infrastructure

- 报告渲染继续使用 Go 标准库 `html/template`、`html/template` 自动转义和 `go:embed`；DOCX 经由 python-docx + docxtpl sidecar 对受版本管理的干净模板做占位符原位替换（调研定案见 `docx-rendering-research.md`），不引入自研 OOXML patcher 或 Pandoc。
- Evidence 上传使用浏览器原生 Clipboard/File/Drag-and-Drop API 与标准 multipart；服务端使用 Go 标准库文件签名检查、`image.DecodeConfig`、SHA-256 和原子重命名。当前需求不需要图像处理流水线、OCR 或 BLOB 存储。
- 截图排序若原生交互不能满足可用性，再引入单一成熟排序组件；第一版不为上传区引入整套前端构建系统。
- 扫描与验证继续编排现有 RustScan/Nmap/httpx/Nuclei 二进制，不在报告功能中重写探测器或漏洞判断逻辑。
