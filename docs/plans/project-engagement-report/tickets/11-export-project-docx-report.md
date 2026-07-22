# 11 — 导出项目 DOCX 报告（docxtpl 模板填充）

**What to build:** 以当前简化 DOCX 为蓝本一次性制备干净模板（入库受版本管理），用 python-docx + docxtpl sidecar 把与 HTML 相同的 `ProjectReport` 模型填充进模板，生成保持模板未填充部分版式和结构的 DOCX 下载。调研定案与逐槽位替换地图见 `docs/plans/project-engagement-report/docx-rendering-research.md`（§4 替换地图、§5 方案与降级）。

**Blocked by:** 08 — 生成项目正式单文件 HTML。

**Status:** done

## 当前状态：正式模板、运行时导出与自动结构门已完成

用户已否决旧实验模板的业务结构，并对第一版可视占位符模板提出六项具体纠错。纠错后的契约已固化为受版本管理的正式运行时模板 `tools/docx-render/templates/project-report.docx`，Web 导出通过同目录 sidecar 填充该模板。

2026-07-21 当前模板决定：封面日期和月份取 Project 创建时间；编制单位固定；整个第二节（含 2.2）原文不动；每个 Project Scan Run 归属 I区、II区、III区或自定义分区，报告只生成实际有纳入内容的分区。第三轮进一步确认：封面三个值使用等长、同粗细下划线且值居中；月份复制用户格式刷后保存版本的段落格式；每个有 not_observed 的分区在全部 confirmed 漏洞之后追加一个“X区其它漏洞验证不存在的截图：”Heading 3，下面先写漏洞名与端口说明再插图。该标题保持 Heading 3 大纲级别，但不显示或占用 3.x.y 编号，结束后才进入下一个分区。

## 已否决实验记录（2026-07-21，仅保留技术证据）

`tools/docx-render/` 的 fixture 只证明 Python 3.12 + docxtpl 管线可以执行，以及若干 OOXML 结构检查可自动化。它不证明实验模板的业务槽位或内容结构正确。`out/prototype-template.docx` 与 `out/project-report.docx` 均已被用户否决，不得作为正式模板蓝本。

被否决的版本曾把 Zone 内容放到第四节之后并制造源模板不存在的 Zone 表格。正式契约现明确：网络分区只能位于第三节且不使用表格；第四节之后不得有网络分区内容。WPS 不响应 `updateFields` 是已知兼容性限制，导出界面会提示用户手动刷新目录；自动结构门负责校验目录域、书签、占位符和版式结构，因此该限制不再阻塞发布。

## 正式模板制备证据（2026-07-21）

- `out/project-report-placeholder-template.docx` 的全部 Jinja 槽位可见，正文示例图片为 0；整个第二节与源副本一致。
- 封面 `report.test_subject/project_created_date/testers_text` 复用“测试时间”的字符格式；三条均移除制表位引导线，改为单一默认粗细（`w:u=single`）下划线 run 包含左右不换行空格和数据，并按模板字体度量补齐到近似等宽，因此整条下划线连续、同高同粗且值居中。`report.project_created_month` 原样继承用户保存后的源段落格式。
- 表 3-1 自动生成 0/1/3 条数据行，0 条时显示跨四列空状态。
- 模板中的分区循环、Scan Run、Verification 和 Evidence 全部结束在第四节之前；confirmed 漏洞一条对应一个带编号的 Heading 3；有 not_observed 时，在该分区全部 confirmed 之后追加一个不编号、不占号但保持 Heading 3 大纲级别的汇总标题，条目说明与端口位于图片之前，随后才进入下一个分区。
- 漏洞描述与修改建议槽分别对应现有 `VulnerabilityDelivery.Description/Remediation`；模板不含 `verification.detail`、独立“证据截图”标题或 caption。“漏洞详情”下只循环 Evidence 图片，按 fixture 顺序插入，并在最大宽高框内保持 2:1、1:2、2:1 纵横比。
- 第四节使用 `report.client_name`、实际分区、总数、高/中/低计数与从知识库漏洞名称派生的 `focus_text`，第二句为固定整改文案。
- 模板与渲染结果保留 3 个 `sectPr`、3 个页眉、7 个页脚、footer6 浮动图形和原域签名；设置了 `w:updateFields`。
- 第三轮占位符模板 7 页已用标准 LibreOffice 管线重新渲染并逐页检查：三条下划线起止和粗细一致、月份恢复用户保存格式、新增负向验证标题与说明位置正确、正文无示例图。随后生成 `out/project-report-fake.docx`：11 页，覆盖 I区、III区和互联网接入区，每区依次生成 confirmed 漏洞、不编号的其它漏洞不存在小节及其 Evidence，再进入下一区或第四节；7 张 fake Evidence 的插入顺序和纵横比通过自动检查。当前渲染环境缺少源模板使用的中文字体，中文字形不据此验收；文本和结构由 OOXML 检查覆盖。WPS 的目录手动刷新流程作为已知兼容性限制接受，并由导出界面提示。
- 最终封面修复彻底移除前后制表位 run：每条线只保留一个下划线 run，并在同一 run 内用不换行空格包住数据；第四轮用户复核后把下划线从 thick 改回默认 single。自动门检查无 `w:tabs`、`w:u=single`、左右空格和统一宽度单位；模板与 fake 报告已重新生成，结构回归和 11 页渲染检查通过。
- 目录重建修复（WPS 验收发现）：源模板目录是自动 TOC 域但缓存了 3.1/3.2/3.2 三个分区条目，其 PAGEREF 指向的书签在动态分区循环中被删除，导致「未定义书签」。`prototype.rebuild_toc_field` 现清空 TOC 域缓存 result、保留 begin/instr/separate/end 骨架；Word 可依据正文 Heading 样式和 `updateFields` 更新目录，WPS 则需按导出提示手动刷新。`check_structure` 的域校验放宽为「稳定域（PAGE/STYLEREF/REF）签名不变 + 仍含 1 个 TOC 域」。重新生成的模板与 fake 报告 PAGEREF=0、无缺失书签、无残留 XX/Jinja。

- [x] 用户复核并批准 `template-slot-contract.md`，模板基线已锁定为 `tools/docx-render/templates/project-report.docx`。
- [x] 一次性正式模板制备：模板按逐位置地图放置 Jinja 槽、动态表 3-1、第三节网络分区区块，删除正文示例截图并补 `updateFields`。
- [x] Go 侧把共享项目报告序列化为 JSON context，经 `uv run --project tools/docx-render python render_docx.py ...` 渲染；doctor 检查同一 uv 环境，缺失时 DOCX 明确提示且不影响 HTML。
- [x] DOCX 与 HTML 消费同一份项目交付读模型与完整性校验（缺元数据、无 Evidence、残留占位即失败）。
- [x] 表 3-1 动态行、confirmed/not_observed 条目、Evidence 图片、Scan Run 与网络分区循环全部填充，导出后无残留 `XX`/Jinja 占位。
- [x] 多 Run、多 Zone、多 Evidence fixture 覆盖 HTML/DOCX 同源数据，自动结构门覆盖封面、目录域、表 3-1、验证详情与页脚结构。
- [x] Python sidecar 路线已接受并随 release 归档发布；不启用降级路径。
