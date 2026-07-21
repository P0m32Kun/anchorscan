# Handoff — 项目级渗透测试与 HTML 与 DOCX 报告

## 当前状态：最终封面下划线修复已进入模板和 fake 报告，待用户在 WPS 复核

用户在第三轮复核中要求统一封面三条下划线、保留其格式刷后的月份位置，并为每个有负向验证的分区增加三级标题和图片前说明。随后进一步确认：该标题位于所属分区全部 confirmed 漏洞之后、下一个分区之前，沿用 Heading 3 大纲级别但不显示或占用 3.x.y 编号。`template-slot-contract.md` 与 `tools/docx-render/out/project-report-placeholder-template.docx` 已同步修订；`tools/docx-render/out/project-report-fake.docx` 已按新契约生成并完成自动检查和 11 页逐页核对。

当前硬边界：业务模板只保留表 3-1 一张表；Network Zone、Verification 和 Evidence 都是第三节中的段落区块；第四节之后不得再出现任何网络分区或测试结果内容；模板中不得放示例图片。每个 Project Scan Run 必须归属 I区、II区、III区或自定义分区，报告只生成实际有纳入内容的分区。

## 从这里继续

1. 读 `CONTEXT.md` 中 Project、Network Zone、Verification、Evidence、Run Report、Project Report。
2. 读 `docs/adr/0004-model-project-as-pentest-engagement.md` 与 `docs/adr/0005-docx-rendering-via-docxtpl.md`。
3. 读本目录 `template-analysis.md`、`spec.md`、`product-design.md`、`technical-design.md`、`docx-rendering-research.md`。
4. 按 `tickets/` 阻塞边推进；当前全部为 draft，先让用户批准 spec，再把 01 标记为 `ready-for-agent`。

## 本轮已确认

- Project 是一次渗透测试任务，不是扫描参数预设。
- I区、II区、III区或甲方自定义分区统一称为 Network Zone；创建 Project Scan Run 时必须且只能选一个。报告只输出实际有纳入 Run 或 included Verification 的分区，不补空分区。
- Project Report 聚合多个 Runs。
- 正向漏洞必须人工确认并有截图；负向只从双引擎 completed、无非 info Finding 的指纹端点发起，最终仍需人工验证和截图。
- 表 3-1 动态聚合 confirmed Verifications：标题、所有唯一 IP:port、危险等级。
- 所有 `XX` 必须绑定 Project/Network Zone/Verification/结论数据，不能原样导出。
- 最终导出两类格式：单文件 HTML 和 DOCX，二者由同一 `BuildProjectReport` 模型驱动；DOCX 以用户标准模板为蓝本，通过模板填充保持未填充部分的版式和结构。删除用户 JSON/CSV，但保留内部 report.json。
- DOCX 渲染技术路线已定案（调研见 `docx-rendering-research.md`）：python-docx + docxtpl sidecar，经 `uv run` 编排并由 doctor 检查；一次性在 Word 里手工制备干净模板（逐槽换占位符、表 3-1 保留 1 个数据模板行并加独立控制行、删示例图、补 updateFields）。网络分区在第三节按实际数据动态生成；降级路径为 lukasjarosch/go-docx + 预制上限行数。Pandoc 已被一手来源否决。
- 报告元数据（报告标题、测试对象、测试人员、起止日期）只在项目管理界面维护，报告页只读引用并做缺失检查。
- Evidence 按漏洞类型计，不按 IP：同一类漏洞的 confirmed 至少一张截图覆盖全部关联资产；同一类验证项的 not_observed 一张共享截图覆盖本次勾选的全部端点。负向验证流程改为“选验证项 → 勾选多个同类端点 → 一次验证 → 一张截图 → 提交单条 not_observed”。
- DOCX 结构核对见 `template-analysis.md`“当前源模板（用户简化版）”表：用户已简化模板（sha256 `e5f6a896…`）现含 2 张正文 `wp:inline` 图片、0 批注、3 个 `w:sectPr` 版式节、汇总表表头+1 样本行、10 PAGEREF、3 个 Zone 标题块（3.1 I区 / 3.2 II区 / 3.3 III区）。报告内容另有 4 个一级章节（概述、测试过程及方法、测试结果与分析、渗透测试结论），两种计数不冲突。原始文件（`fc5f4244…`，20 页/4 版式节/7 空行表/15 图/2 批注）仅作历史参考，不作为运行时蓝本。
- 整个第二节（2.1、2.2、2.3）是固定模板正文，原封不动保留，不设工具占位符，也不从 Runs 生成。
- 封面日期和页面底部月份均取 Project 创建时间；“南京南瑞信息通信科技有限公司”为固定模板文案。
- 封面“测试对象 / 测试时间 / 测试人员”不再使用制表位引导线；三条均由一个粗下划线 run 承载“左侧不换行空格 + 数据 + 右侧不换行空格”，并按模板字体度量补齐到近似等宽，使整条线粗细、高度连续且值居中。页面底部月份复制用户格式刷后保存版本的缩进和段落标记字符格式，不另加居中样式。
- 漏洞描述和修改建议复用现有 `knowledgebase.Catalog` → `BuildMatchedVulnerabilityDeliveries` → `VulnerabilityDelivery.Description/Remediation` 链路；DOCX 不再设计第二套解析，也不再增加 `verification.detail`。
- “漏洞详情”是 Evidence 截图区，只放按顺序插入的图片；不再输出“证据截图”标题、说明文字或 caption。
- 每条 confirmed 漏洞使用一个带 3.x.y 编号的 Heading 3；若分区存在 not_observed，则在该区全部 confirmed 漏洞之后追加一个不编号的“X区其它漏洞验证不存在的截图：”Heading 3。它不显示或占用 3.x.y 编号，其下每条负向验证先显示“xx漏洞不存在证明，端口（XX）”，再按顺序插入图片；完成后才进入下一个分区。
- 第四节使用用户指定的单位、实际分区、总数、高/中/低计数、知识库漏洞名称摘要和固定整改文案，不再使用人工 `conclusion.text`。

## 本地数据证据（只读检查）

- `data/scans.sqlite` 中有 4 个“甘肃-新一代”Projects，每个恰好 1 个 completed Run。
- 其中 3 个项目名属于 I 区不同网段，1 个属于 III 区；应显式归并为一个任务 Project。
- 4 个 Runs 共包含大量 `info` 与少量 `low` Findings，但没有 DetectionCheck 行；因此不能从旧数据自动生成负向验证结论。
- 不要把这些 Project 自动按名称合并，也不要在未备份数据库时执行迁移。

## 已选择的最小实现边界

- confirmed 与 not_observed 都严格要求至少一张截图；不提供“无截图原因”绕过。
- 报告编号、版本、审核/批准人员不在当前参考模板中，留到有明确格式后再加。
- 甘肃历史归并使用默认 dry-run、`--apply` 前强制备份的一次性本地命令，不建设通用合并 UI。
- Tool Run 默认不进入自动候选聚合，但 Verification 可以引用其执行来源和 Evidence。

## 落地原则（跨 agent 协作）

设计与研究**必须落在仓库文档**，不能只存在某个 agent 的 session 记忆里：ADR 记不可逆架构决策（`docs/adr/`），计划与调研记在 `docs/plans/project-engagement-report/`，领域词在 `CONTEXT.md`。其他 coding agent 接手时只读仓库文档即可对齐，不依赖会话记忆。

## DOCX 渲染落地（原型先行，再 ticket 11）

在动手制备最终模板前，先做 throwaway 原型验证 docxtpl 管线（不依赖 ticket 08，现在可跑），验证 4 件事：

1. **uv 锁环境**：`tools/docx-render/` 建独立 uv 子项目，`.python-version` 锁 Python 3.12 以取得可复现环境，`pyproject.toml`/`uv.lock` 锁 `docxtpl` 与依赖，`uv run` 一次跑通。
2. **文本占位保版式**：拷贝简化模板，封面 `XX电力有限公司` 换 `{{ client_name }}`，确认不被 run 拆碎、直接格式不丢。
3. **表格行循环 + 图片插入**：汇总表保留一个数据行，并增加独立 `{%tr for %}` / `{%tr endfor %}` 控制行；将一张示例图改为 `InlineImage` 插入，确认 Word 打开后表/图/域正常。
4. **目录域刷新**：补 `<w:updateFields w:val="true"/>`，Word 打开能更新页码。

原型只保留可复用的 `tools/docx-render/` 运行骨架和自动检查；一次性试验脚本与 fixture 删除，结论留在本文件，随后进入 ticket 11。

本机工具链实测（2026-07-20）：`uv 0.11.29` ✓、`python3 3.14.6`、`docxtpl` 未装、仓库无任何 Python 基础设施。选择 3.12 是为锁定可复现环境，不是因 docxtpl 不支持 3.14。

### DOCX 原型记录（2026-07-21，第三轮占位符模板与 fake 报告已生成）

原型位于 `tools/docx-render/`。它把用户简化模板复制为 `fixtures/source-template.docx`（SHA-256 仍为 `e5f6a896...a2c6e5`），只改写 `out/` 下的实验模板和产物；固定输入为 `fixtures/project_report.json`，不接数据库或 Web。Python 3.12.12、`docxtpl==0.20.2` 及传递依赖由 `uv.lock` 锁定。

1. **文本槽：自动通过、待用户复核。** `out/project-report-placeholder-template.docx` 的封面三个值槽均复用源模板“测试时间”的字符格式；每条线只有一个 `w:u=thick` run，内容同时包含左右补齐空格和数据，不含 `w:tabs`。渲染器按模板 14 磅宋体的实际字符宽度补齐，三条线连续且近似等长；月份段落复制用户格式刷后保存版本的 `pPr`（含缩进与 East Asia 字体提示），不再添加自定义居中样式。
2. **表 3-1：通过。** `{%tr for r in summary_rows %}` 可生成 0、1、3 条数据行；0 条时显示跨四列空状态，控制行在结果中删除。模板内使用短别名 `r.no/r.title/r.assets/r.level`，避免窄列把长字段路径拆得无法辨认。
3. **网络分区与负向验证结构：模板与 fake 报告通过。** `network_zones[]` 内先重复 Scan Run 和全部 confirmed，再在 `not_observed` 非空时生成“`{{ network_zone.name }}`其它漏洞验证不存在的截图：”Heading 3；每条负向验证在 Evidence 前显示漏洞名和端口。该标题保持与 confirmed 漏洞名相同的大纲级别，但通过直接编号覆盖关闭显示编号，不占用 3.x.y；负向区块结束后才进入下一个分区。fake 数据覆盖 I区、III区和互联网接入区，每区均按“confirmed 漏洞 → 其它漏洞不存在 → 下一区/第四节”的顺序生成。
4. **漏洞内容与 Evidence：通过。** “漏洞描述 / 修改建议”只保留 `verification.description/remediation`，正式实现来源明确为现有 `VulnerabilityDelivery` 知识库字段；“漏洞详情”标题下直接循环 `{{ evidence.image }}`。模板无 `verification.detail`、无“证据截图”标题、无 caption，正文图片为 0。fixture 的 3 张测试 PNG 按红（宽）→蓝（高）→绿（宽）插入，最大宽高框内的 OOXML 尺寸比为 2:1、1:2、2:1，顺序正确且未拉伸。
5. **保留结构：通过。** 自动检查以未改源副本为基线，确认实验模板及 0/1/3 行渲染结果仍有 3 个 `sectPr`、3 个 header part、7 个 footer part，`footer6.xml` 的 1 个 `wp:anchor` 和 `wps:wsp` 仍在；document/header/footer 域签名不变，`settings.xml` 已有 `w:updateFields w:val="true"`，第二节文本顺序与源副本一致。
6. **逐页渲染：占位符模板通过（LibreOffice 环境）。** 使用 documents skill 的标准渲染脚本渲染模板并逐页检查：三条封面下划线起止一致、粗细一致；月份位置按用户保存的源格式恢复；负向验证标题位于所属分区末尾、保持 Heading 3 大纲级别但不显示编号，说明位于图片槽之前；模板正文无示例图。当前 LibreOffice 环境缺少源模板使用的中文字体，中文字形不据此验收，固定文本、编号覆盖和槽位顺序由 OOXML 检查覆盖。
7. **WPS 验收：未完成。** 当前会话无法读取 WPS GUI，因此不能声称“无修复提示打开”或“目录和页码已刷新”。模板保留原域结构并设置 `updateFields`；仍需用户在可交互 WPS 中打开，选择更新整个目录并确认无修复提示。

`check_structure.py` 是保留的最小结构门：校验源副本哈希、显式槽位、正文示例图为 0、第二节不变、网络分区/第四节顺序、缺失 II 区不输出、自定义分区输出、模板域/页脚结构、0/1/多表格行、文本 run 属性、`updateFields` 与 Evidence 顺序/纵横比；`out/` 被忽略，不进入正式交付。

第三轮修改已进入占位符模板、结构门、槽位契约和 fake fixture。`out/project-report-fake.docx` 共 11 页，包含 3 个动态分区、3 条 confirmed、3 条 not_observed 和 7 张按顺序插入且保持纵横比的 fake Evidence；自动结构检查与逐页渲染检查均通过。下一步由用户在 WPS 中审阅实际版式；WPS GUI 的无修复提示、目录和页码刷新仍是人工验收门。

最终封面修复：已删除三条数据线的制表位和下划线引导符，改为“左侧不换行空格 + 数据 + 右侧不换行空格”共用一个 `w:u=thick` run。结构门锁定“无 `w:tabs`、单一粗下划线 run、左右均有补齐空格、三条宽度单位一致”；模板和 fake 报告已重新生成并完成 11 页渲染检查，LibreOffice 渲染中三条线连续、同高同粗，长度差处于字体取整范围内。

## 不要做

- 不要重启 Pandoc/reference.docx 方案或自研 OOXML patcher；DOCX 只走 docxtpl 占位符原位替换（降级也只用 lukasjarosch/go-docx）。
- 不要直接 patch 原始示例 DOCX 充当运行时模板；先一次性制备干净模板再入库。
- 不要删除 `report.WriteJSON` 或内部 `report.json`；用户说的是下载格式。
- 不要把 Evidence 放进 Finding.Output、DetectionCheck.Detail、SQLite BLOB 或 ArtifactDir。
- 不要把缺失 DetectionCheck 当成“已完成且零发现”。
- 不要直接以可编辑漏洞标题作为聚合键。
