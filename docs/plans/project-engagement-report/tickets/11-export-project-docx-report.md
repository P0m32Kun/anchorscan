# 11 — 导出项目 DOCX 报告（docxtpl 模板填充）

**What to build:** 以当前简化 DOCX 为蓝本一次性制备干净模板（入库受版本管理），用 python-docx + docxtpl sidecar 把与 HTML 相同的 `ProjectReport` 模型填充进模板，生成保持模板未填充部分版式和结构的 DOCX 下载。调研定案与逐槽位替换地图见 `docs/plans/project-engagement-report/docx-rendering-research.md`（§4 替换地图、§5 方案与降级）。

**Blocked by:** 08 — 生成项目正式单文件 HTML。

**Status:** draft

## 当前状态：最终封面下划线修复已进入模板和 fake 报告，待用户在 WPS 复核

用户已否决旧实验模板的业务结构，并对第一版可视占位符模板提出六项具体纠错。`docs/plans/project-engagement-report/template-slot-contract.md` 已同步修订；新的 `out/project-report-placeholder-template.docx` 按第二轮契约生成，当前用于逐页确认槽位和第三节结构，不能直接当作正式运行时模板。

2026-07-21 当前模板决定：封面日期和月份取 Project 创建时间；编制单位固定；整个第二节（含 2.2）原文不动；每个 Project Scan Run 归属 I区、II区、III区或自定义分区，报告只生成实际有纳入内容的分区。第三轮进一步确认：封面三个值使用等长、同粗细下划线且值居中；月份复制用户格式刷后保存版本的段落格式；每个有 not_observed 的分区在全部 confirmed 漏洞之后追加一个“X区其它漏洞验证不存在的截图：”Heading 3，下面先写漏洞名与端口说明再插图。该标题保持 Heading 3 大纲级别，但不显示或占用 3.x.y 编号，结束后才进入下一个分区。

## 已否决实验记录（2026-07-21，仅保留技术证据）

`tools/docx-render/` 的 fixture 只证明 Python 3.12 + docxtpl 管线可以执行，以及若干 OOXML 结构检查可自动化。它不证明实验模板的业务槽位或内容结构正确。`out/prototype-template.docx` 与 `out/project-report.docx` 均已被用户否决，不得作为正式模板蓝本。

被否决的版本曾把 Zone 内容放到第四节之后并制造源模板不存在的 Zone 表格。正式契约现明确：如果保留网络分区，它只能在第三节且不使用表格；第四节之后不得有网络分区内容。已启动本机 `wpsoffice.app`，但当前会话无法读取其 GUI 窗口，尚未验证“无修复提示打开”及实际目录/页码刷新；这仍是正式模板的人工验收门。

## 新实验模板证据（2026-07-21）

- `out/project-report-placeholder-template.docx` 的全部 Jinja 槽位可见，正文示例图片为 0；整个第二节与源副本一致。
- 封面 `report.test_subject/project_created_date/testers_text` 复用“测试时间”的字符格式；三条均移除制表位引导线，改为单一默认粗细（`w:u=single`）下划线 run 包含左右不换行空格和数据，并按模板字体度量补齐到近似等宽，因此整条下划线连续、同高同粗且值居中。`report.project_created_month` 原样继承用户保存后的源段落格式。
- 表 3-1 自动生成 0/1/3 条数据行，0 条时显示跨四列空状态。
- 模板中的分区循环、Scan Run、Verification 和 Evidence 全部结束在第四节之前；confirmed 漏洞一条对应一个带编号的 Heading 3；有 not_observed 时，在该分区全部 confirmed 之后追加一个不编号、不占号但保持 Heading 3 大纲级别的汇总标题，条目说明与端口位于图片之前，随后才进入下一个分区。
- 漏洞描述与修改建议槽分别对应现有 `VulnerabilityDelivery.Description/Remediation`；模板不含 `verification.detail`、独立“证据截图”标题或 caption。“漏洞详情”下只循环 Evidence 图片，按 fixture 顺序插入，并在最大宽高框内保持 2:1、1:2、2:1 纵横比。
- 第四节使用 `report.client_name`、实际分区、总数、高/中/低计数与从知识库漏洞名称派生的 `focus_text`，第二句为固定整改文案。
- 模板与渲染结果保留 3 个 `sectPr`、3 个页眉、7 个页脚、footer6 浮动图形和原域签名；设置了 `w:updateFields`。
- 第三轮占位符模板 7 页已用标准 LibreOffice 管线重新渲染并逐页检查：三条下划线起止和粗细一致、月份恢复用户保存格式、新增负向验证标题与说明位置正确、正文无示例图。随后生成 `out/project-report-fake.docx`：11 页，覆盖 I区、III区和互联网接入区，每区依次生成 confirmed 漏洞、不编号的其它漏洞不存在小节及其 Evidence，再进入下一区或第四节；7 张 fake Evidence 的插入顺序和纵横比通过自动检查。当前渲染环境缺少源模板使用的中文字体，中文字形不据此验收；文本和结构由 OOXML 检查覆盖。WPS 无修复提示及目录/页码刷新仍待可交互人工验收。
- 最终封面修复彻底移除前后制表位 run：每条线只保留一个下划线 run，并在同一 run 内用不换行空格包住数据；第四轮用户复核后把下划线从 thick 改回默认 single。自动门检查无 `w:tabs`、`w:u=single`、左右空格和统一宽度单位；模板与 fake 报告已重新生成，结构回归和 11 页渲染检查通过。

- [ ] 用户复核并批准第二轮 `template-slot-contract.md` 与可视占位符模板；历史决定已记录，但模板基线尚未最终锁定。
- [ ] 一次性正式模板制备：实验模板已按逐位置地图放置 Jinja 槽、动态表 3-1、第三节网络分区区块，已删除 2 张正文示例截图并补 `updateFields`；待用户确认业务版式和 WPS 人工验收后再作为受版本管理的正式模板入库。
- [ ] Go 侧把 `ProjectReport` 序列化为 JSON context，经 `uv run --project tools/docx-render python render_docx.py ...` 渲染；doctor 运行同一 uv 环境的 `import docxtpl` 检查，缺失时 DOCX 按钮明确提示且不影响 HTML。
- [ ] DOCX 与 HTML 消费同一份 `BuildProjectReport` 输出与完整性校验（缺元数据、无 Evidence、残留占位即失败），不另建聚合模型。
- [ ] 表 3-1 动态行、confirmed/not_observed 条目、Evidence 图片和网络分区循环全部填充；confirmed 每条生成带编号的 Heading 3；not_observed 非空时，在每区最后追加一个不编号、不占号但保持 Heading 3 大纲级别的汇总标题，随后才进入下一个区；“漏洞详情”只插图；描述和修改建议复用现有知识库交付字段；导出后扫描无残留 `XX`/Jinja 占位。
- [ ] 用多 Run、多 Zone、多 Evidence fixture 校验 DOCX 与 HTML 同源一致，并与原模板逐页比对排版（封面、目录域、表 3-1、验证详情、页脚浮动图形）。
- [ ] 若部署方否决 Python sidecar，停下来走降级路径（lukasjarosch/go-docx + 模板预制上限行数，缺口清单见调研 §5.3），不得临时自研 OOXML patcher 或重启 Pandoc。
