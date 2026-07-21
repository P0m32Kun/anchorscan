# DOCX 渲染调研：占位符原位替换 vs 重新生成（一手来源版）

> 调研日期：2026-07-20。文中所有链接均于 2026-07-20 实际访问验证（HTTP 200）。
> 一手来源 = 官方文档 / 规范条款 / 库源码与 GitHub issue / 官方定价页；未引用任何二手博客。
> 配套文件：同目录 `template-analysis.md`（模板结构核对）、`handoff.md`、`technical-design.md`。

## TL;DR

**结论：要以模板为蓝本保持可见版式，采用模板填充路线——一次性制备干净模板，运行时只填充目标文本、复制指定表格数据行、插入/替换图片。** Pandoc + reference.docx 做不到：Pandoc 官方手册明确 reference.docx 只提供样式表与文档属性（页边距、页面大小、页眉、页脚），其内容被忽略，正文 XML 由 Pandoc 从自己的文档模型重新生成——当前模板的节结构、页脚浮动图形、域代码和直接格式无法可靠保留。

**推荐方案：python-docx + docxtpl 作为 sidecar 进程，Go 主程序经 `uv run` 编排，doctor 工具检查同一 uv 环境中的 `docxtpl`**（与现有“编排外部二进制”架构一致）。在“自由/开源 + 表格行循环、图片插入和模板控制标签”约束下，这是最成熟的现成选择。Go 生态无等价物：能力最全的 Go 库 unioffice 是商业许可（按文档计费/询价），其余 Go 库缺口见 §5.3。

降级路径：`lukasjarosch/go-docx`（MIT，纯 Go，跨 run 占位符 + 图片原位替换）+ 模板预制上限行数/区块数；功能缺口清单见 §5.3。

---

## 1. 路线论证：原位替换 vs 重新生成

### 1.1 Pandoc 官方原文（一手）

Pandoc MANUAL `--reference-doc` 选项（https://pandoc.org/MANUAL.html#option--reference-doc ，访问 2026-07-20）：

> "The contents of the reference docx **are ignored**, but its **stylesheets and document properties** (including margins, page size, header, and footer) are used in the new docx."
> "For best results, **do not make changes to this file other than modifying the styles** used by pandoc: Paragraph styles: Normal, Body Text, … Heading 1–9 … Character styles: … Table style: Table."

即 reference.docx 的贡献**仅限**：styles.xml 中 pandoc 认识的那一组具名样式 + 文档级属性（页边距/页面大小/页眉/页脚）。其余一切——正文 document.xml 的 run/段落直接格式、多节（sectPr）结构、表格行结构、浮动图形、域代码、文本框——都由 Pandoc 从自己的 AST 重新序列化。本模板排版的核心恰恰在这些“其余一切”里：

- 当前模板的直接格式与域代码——Pandoc AST 无法可靠保留逐 run 直接格式；
- 3 个 A4 节、7 个页脚——Pandoc 输出单节文档，页眉页脚只取 reference 的默认部分；
- 页脚 1 个浮动图形（`footer6.xml` 的 `wp:anchor`，见 §2.6 与附录实测）——Pandoc AST 无浮动图形/文本框概念；
- 10 PAGEREF + 5 PAGE + 1 STYLEREF + TOC 域——Pandoc 的 `--toc` 生成的是自己的目录结构，不会复刻模板域代码；
- 表 3-1 的一个样本行与 2 张正文示例图——本来就要动态化，重新生成路线等于把“复刻模板排版”这个最难的问题原样留给自己用 AST 重写一遍。

### 1.2 本质差异

| | 占位符原位替换 | 重新生成（pandoc / from scratch） |
|---|---|---|
| document.xml | 仅对指定模板槽进行填充，保留未参与填充的可见结构和样式（须渲染回归验证） | 由库/工具的文档模型**从头序列化** |
| 直接格式（122 run 级 / 251 段落级） | 不动即保留 | 模型无法表达即丢失 |
| 节/页脚浮动图形/域代码 | 不动即保留 | 需逐特性重建，多数工具不支持 |
| 保真上限 | 与模板一致（除了你主动改的部分） | 上限是工具的 OOXML 表达能力 |
| 工作量 | 一次性制备模板 + 填数据 | 逐特性复刻排版 |

**结论：模板填充路线的版式保真上限远高于重新生成路线；重新生成路线的保真上限受工具 AST 表达能力限制，对本模板不达标。注意 docxtemplater 的“XML 不变”说明不适用于 Python 的 docxtpl：docxtpl 会以 python-docx 重新保存 DOCX 包，因此验收标准必须是渲染后版式与结构，而非 ZIP/XML 字节一致。**

---

## 2. OOXML 机制（ECMA-376 / Microsoft Learn 一手）

规范总入口：ECMA-376 官方页（https://ecma-international.org/publications-and-standards/standards/ecma-376/ ，访问 2026-07-20；WordprocessingML 在 Part 1）。以下条款引文取自 Microsoft Learn 的 Open XML SDK API 参考——每页 Remarks 都逐字转录 ISO/IEC 29500-1（= ECMA-376-1）对应条款。

### 2.1 Run 拆分与直接格式保全

**run 的定义与 rPr**（MS Learn, *Working with runs*：https://learn.microsoft.com/en-us/office/open-xml/word/working-with-runs ，访问 2026-07-20）：

> "a run, which defines a region of text with a common set of properties… All of the elements inside an r element have their properties controlled by a corresponding optional **rPr run properties element, which must be the first child of the r element**."

`r (Text Run)` 条款（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.run ）：run 是段落/域/超链接内的内容单元。

**推论 1**：模板的 122 处 run 级直接格式就住在各 `w:rPr` 里。只要**在原有 run 的 `w:t` 内换文本**，`w:rPr` 原样不动，格式零损失；反之删除/重建 run 而不复制 `w:rPr`，格式即丢。python-docx 里典型的踩坑写法 `paragraph.text = value` 就是清空全部 run 重建。

**推论 2：占位符必须按“碎片”处理。** Word 会因编辑历史、拼写检查（`w:proofErr`）、rsid 会话等把一段文字切成多个 run。lukasjarosch/go-docx README（https://github.com/lukasjarosch/go-docx ，访问 2026-07-20）给出了机制级描述：

> "Due to the nature of the WordprocessingML specification, a placeholder which is defined as `{the-placeholder}` may be ripped apart inside the resulting XML. The placeholder may then be in two fragments for example `{the-` and `placeholder}`…"
> 其替换策略："The first fragment found … will be replaced with the value … **All other fragments of the placeholders are cut out**, removing the leftovers." 并建议："it's best to just **style the whole placeholder**"（因为取值继承第一个碎片所在 run 的格式）。

**本模板实测**（附录）：封面 `测试对象：XX电力有限公司` 在 document.xml 中被拆为 `XX|电力|有限|公司` 四个 run——run 拆分在本模板真实存在。

**各库处理**：
- docxtpl：官方限制——"The usual jinja2 tags, are only to be used **inside the same run of a same paragraph**, it can not be used across several paragraphs, table rows, runs."（https://docxtpl.readthedocs.io/en/latest/ ，访问 2026-07-20）。即制备模板时须保证每个 `{{ tag }}` 一次性键入、落在单个 run 内（Word 里整词重新输入即可）。
- docxtemplater（JS）：解析器先对全文词法分析再匹配标签，天然跨 run（其 internals 文档示例正是 Word 把 "World" 切成 `W`+`orld` 两个 run 夹 `w:proofErr`）。
- lukasjarosch/go-docx：跨 run 碎片替换是其唯一卖点（上引 README）。
- nguyenthenguyen/docx：朴素 `strings.Replace`，碎片化即失败——open issue #40 "Replace is very 'flaky'"、#43 "fix: Handle split XML runs and spellcheck tags in placeholder replacement"（https://github.com/nguyenthenguyen/docx/issues/40 、/issues/43 ，访问 2026-07-20）。

### 2.2 动态表格行

`tr (Table Row)` 条款（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.tablerow ，访问 2026-07-20）：

> "A tr element has one formatting child element, **trPr (§17.4.82), which defines the properties for the row**."

MS Learn *Working with tables*（https://learn.microsoft.com/en-us/office/open-xml/word/working-with-wordprocessingml-tables ）确认单元格格式在 `tcPr`。

**标准做法 = 保留一个“模板行”，按数据深拷贝 `w:tr`（连同 `trPr`/`tcPr`），再替换单元格文本。** docxtpl 的 `{%tr for %}` 就是这个机制的模板化：官方 Extensions 文档——"`{%tr jinja2_tag %}` for table rows … these tags also tell python-docx-template to **remove the … table row … where the tags are located**"，Jinja 循环把该行 XML（含 trPr/tcPr）按数据复制 N 份。还支持 `{% colspan %}` 横向合并、`{% vm %}`/`{% hm %}` 循环内纵横合并、单元格底色 tag（同页 Tables 章）。

**表 3-1 具体做法**（样本行 → N 行）：当前模板保留 1 个数据样本行；制备模板时额外增加两条独立控制行，分别只含 `{%tr for v in summary %}` 和 `{%tr endfor %}`，二者会被 docxtpl 删除。数据样本行仅写 `{{ v.no }}`、`{{ v.title }}`、`{{ v.assets }}`、`{{ v.severity }}`。docxtpl 禁止在同一行使用两个 `{%tr ... %}` 标签，不能把控制标签塞进数据样本行的首尾单元格。行格式（边框/底纹/行高）由被复制的数据 `w:tr` 保留。

### 2.3 图片两种策略

MS Learn *How to: Insert a picture into a word processing document*（https://learn.microsoft.com/en-us/office/open-xml/word/how-to-insert-a-picture-into-a-word-processing-document ，访问 2026-07-20）展示了新建图片的完整要素：`wp:inline` + `wp:extent`（**EMU 单位**，示例 `Cx = 990000L, Cy = 792000L`）+ `a:blip` 的 `r:embed` 关系 ID + 在包内添加图片部件。

**策略 (a) 原位替换 `/word/media` 二进制**：relationship、`wp:extent`（尺寸/纵横比）、锚定方式、docPr 全部不动 → 100% 保真，但**显示尺寸与纵横比被占位图锁死**，且格式需一致。
- docxtpl `replace_pic(embedded_file, dst_file)`：官方文档 Note——"the aspect ratio will be the same as the replaced image"、可在页眉/页脚/正文生效；源码（https://github.com/elapouya/python-docx-template/blob/master/docxtpl/template.py ，`_replace_docx_part_pics`）证实匹配键是图片的 `pic:cNvPr` `@name`/`@title`/`@descr`（非 zip 路径），替代文字（descr）即可作槽位标记。
- lukasjarosch/go-docx `SetFile("word/media/image1.jpg", bs)`：README——"the image attributes keep unchanged"、"The image format (encoding) should keep the same"。
- nguyenthenguyen/docx `ReplaceImage`：README——"Currently only swaps apples for apples i.e. png to png"；issue #30 "Fattened replaced images"、#41 "ReplaceImage doesn't work"。

**策略 (b) 新建 `w:drawing` 插入**：尺寸自由，但要自己算 EMU、分配关系 ID、生成 `wp:inline` 结构。
- docxtpl `InlineImage(tpl, image_descriptor=…, width=Mm(20), height=Mm(10))`：官方文档——"You can dynamically add one or many images… use millimeters (Mm), inches (Inches) or points(Pt)"，在 `{{ }}` 处生成新 drawing（https://docxtpl.readthedocs.io/en/latest/ ）。
- python-docx `Document.add_picture()`：官方 *Understanding pictures and other shapes*——"At the time of writing, **python-docx only supports inline pictures**. Floating pictures can be added [but not supported]"（https://python-docx.readthedocs.io/en/latest/user/shapes.html ，访问 2026-07-20）。

**对“证据截图数量、宽高比不固定”**：以策略 (b)（InlineImage 循环插入）为主——数量由循环决定，只指定不超过版心的宽度，让 docxtpl 保持原始纵横比；策略 (a) 仅用于数量固定的特殊槽（且占位图需预先按目标纵横比制作）。docxtpl 文档另明确："It is **not possible to dynamically add images in header/footer**, but you can change them."——页眉页脚图只能 (a)，与本项目无关（页脚图形不动）。

### 2.4 节（sectPr）与按 Zone 区块循环

`sectPr` 条款（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.sectionproperties ，访问 2026-07-20）：

> "This element defines the section properties for the **final section** of the document. Note: For any other section the properties are stored as **a child element of the paragraph element corresponding to the last paragraph** in the given section."

当前模板的 `document.xml` 有 3 个 `sectPr`，保存 3 个版式节的页面设置与页眉页脚引用；它与报告的 4 个内容章节（概述、测试过程及方法、测试结果与分析、渗透测试结论）无关。**Zone 标题位于“测试结果与分析”内容章节内，重复 Zone 不应复制 `sectPr`。**

**区块循环支持**：
- docxtpl：`{%p for %}`/`{%p if %}` 是段落级扩展标签（官方 Extensions——标签所在段落被移除）。它能否包住当前 Zone 的“段落 + 表格 + 图片”混合结构必须由原型确认，不能仅凭段落循环文档假定可行。另有 **Subdoc**：`tpl.new_subdoc('path.docx')` 可把另一个 docx 的内容并入 `{{p mysubdoc }}` 位置（官方 Sub-documents 章），原型不通过时可将 Zone 制作成独立子模板。
- docxtemplater：core（MIT）的 `{#loop}…{/loop}` 支持区块循环（https://docxtemplater.com/docs/tag-types/ ）。
- Go 三库（nguyen / lukasjarosch / fumiama）：均无。

### 2.5 域字段（PAGEREF/PAGE/STYLEREF/TOC）

`updateFields` 条款（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.updatefieldsonopen ，访问 2026-07-20）：

> "updateFields (Automatically Recalculate Fields on Open) — This element specifies whether the fields contained in this document should **automatically have their field result recalculated from the field codes when this document is opened** by an application which supports field calculations."

**通行做法 = 保留全部域代码（fldChar/instrText 结构不动），在 settings.xml 加 `<w:updateFields w:val="true"/>`，让 Word 打开时重算页码/目录。** 本模板实测 settings.xml 当前无此设置（附录），制备模板时补上即可。

必要性佐证：模板引擎本身不是排版引擎，无法预计算页码——docxtemplater 官方 FAQ 直说（https://docxtemplater.com/docs/faq/ ，访问 2026-07-20）：

> "docxtemplater is only a templating engine… it has no idea on how the docx is rendered at the end… this is why it is **impossible to regenerate the page numbers** within docxtemplater."

注意：Zone 数量变化后目录条目本身也会变，`updateFields` 让 Word 重算 TOC 是唯一务实解（部分 Word 版本打开时会提示“是否更新域”，属预期行为）。PAGEREF 指向的 `_Toc*` 书签住在各级标题里，制备模板时**不要删标题文字**（改标题文本要整词改，保住 bookmarkStart/End）。

### 2.6 页脚浮动图形（wp:anchor）

`anchor` 条款（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.wordprocessing.anchor ，访问 2026-07-20）：

> "anchor (Anchor for Floating DrawingML Object)… **Floating** - The drawing object is anchored within the text, but can be absolutely positioned in the document relative to the page."（对比 Inline：随文本流排，如同大字 Glyph。）

实测：当前模板的 `footer6.xml` 有 1 个 `wp:anchor`，是 `wps:wsp` 文本框形状（`wp:anchor` + `txbxContent`，非图片，见附录）。**模板填充路线不应向该页脚放置标签；渲染后须通过结构审计与视觉比对确认其仍正常。重新生成路线下，Pandoc 只取 reference 的默认页眉页脚，且其 AST 无浮动文本框概念 → 必丢。** python-docx 官方也只支持 inline 图片（§2.3 引文），浮动图形本就超出多数库的表达能力。

### 2.7 编制批注剔除

`comment` 条款（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.comment ，访问 2026-07-20）：批注内容存于独立 part（`word/comments.xml`），正文通过 `commentRangeStart/End` 范围 + `commentReference (§17.13.4.5)` 按 id 关联。python-docx 官方 *Working with Comments* 同述（https://python-docx.readthedocs.io/en/latest/user/comments.html ）。python-docx 1.2.0（2025-06-16）起支持 comments（https://github.com/python-openxml/python-docx/blob/master/HISTORY.rst ）。

当前简化模板没有批注，制备时无需处理批注。以上删除方式仅适用于 `template-analysis.md` 标注的历史参考模板，不纳入本次实现任务。

---

## 3. 候选库对比（全部一手来源）

维护状态取自 GitHub API `pushed_at` / 最新 release，2026-07-20 查询。

| 库 | 文本占位（跨 run） | 表格行循环 | 图片原位替换 | 图片插入（自由尺寸） | 区块循环（Zone） | 许可 | 维护状态 |
|---|---|---|---|---|---|---|---|
| **python-docx 1.2.0** | 无占位符引擎，需自遍历 run | 无（需自 clone `w:tr`） | 无 API | `add_picture` 仅 inline | 无 | MIT | 活跃（push 2025-06-17） |
| **docxtpl 0.20.2**（python-docx-template） | ✅ `{{ }}`（限单 run 内） | ✅ `{%tr for %}` | ✅ `replace_pic`（锁纵横比，按 cNvPr name/descr 匹配） | ✅ `InlineImage`（Mm/Inches/Pt） | ✅ `{%p for %}` + Subdoc | LGPL-2.1 | 活跃（push 2026-07-07） |
| nguyenthenguyen/docx | ⚠️ 朴素替换，碎片即失效（issue #40/#43） | ❌（issue #2） | ✅ 同格式原位换（README；issue #30/#41） | ❌ | ❌ | MIT | 停滞（push 2024-07-29） |
| lukasjarosch/go-docx | ✅ 跨 run 碎片（核心卖点） | ❌ | ✅ `SetFile`（锁尺寸/格式） | ❌ | ❌ | MIT | 停滞（push 2024-07-31） |
| fumiama/go-docx | ❌（读写/构建 API，无模板概念） | 可编程建表但无模板循环 | ❌ | 可编程插入 | ❌ | **AGPL-3.0** | 半活跃（push 2025-05-06） |
| unidoc/unioffice v2.12.0 | ✅ MailMerge（走 MERGEFIELD 域）+ find-and-replace 示例 | 有表格 API，行循环需自写 clone | edit-document 示例"replace/remove text without modifying formatting" | ✅（inline/floating 示例） | merge-documents 示例可拼 | **商业 EULA**：metered $0/$20/$100/$300/$500 每月 + 每文档 $0.15/$0.10/$0.08/$0.05；离线 Business/Gold/Platinum 一律 Request a Quote | 活跃（v2.12.0，2026-06-29） |
| docxtemplater core | ✅ `{tag}`（解析器跨 run） | ✅ `{#loop}` 可用于行 | ❌（付费 Image 模块） | ❌（付费 Image 模块） | ✅ `{#loop}` | core **MIT**；模块 €500/个/年、PRO 4 个 €1250/年、Enterprise 全 18 个 €3000/年、Premium €9000/年 | 活跃（push 2026-06-18） |
| .NET Open XML SDK | 全可行但要自己写（低层 SDK，无模板引擎） | 同左 | 同左 | 同左 | 同左 | MIT | 活跃（push 2026-07-14） |

来源逐项：

**Python**
- python-docx docs 首页/Shapes/Comments（https://python-docx.readthedocs.io/en/latest/ 、/user/shapes.html、/user/comments.html）；HISTORY.rst v1.2.0；repo https://github.com/python-openxml/python-docx （均访问 2026-07-20）。
- docxtpl docs（https://docxtpl.readthedocs.io/en/latest/ ）：`{{ }}`/`{%tr %}`/`{%tc %}`/`{%p %}`/`{%r %}` 扩展标签、InlineImage、Subdoc（含 `new_subdoc(path)` 合并既有 docx）、RichText/RichTextParagraph、Listing、colspan/vm/hm/cellbg、`replace_pic`/`replace_media`（CRC 匹配）/`replace_embedded`/`replace_zipname`、`get_undeclared_template_variables()`（渲染后列出未赋值变量——可做导出前完整性检查）、Escaping 章（`<>&` 需 `{{r }}` 或转义——漏洞描述文本必经此处理）、Word 2016 special cases（jinja 标签内 tab/前导空格被忽略，用 RichText 解决）。源码 template.py（同 repo）核实 replace_pic 匹配机制与 `{%tr %}` 的行删除语义。LGPL-2.1 对“独立 sidecar 进程被 exec 调用”无传染性顾虑（不链入 Go 二进制；库本身随 pip 安装、源码可得即满足）。

**Go**
- nguyenthenguyen/docx README + issues #2/#30/#40/#41/#43（https://github.com/nguyenthenguyen/docx ）。
- lukasjarosch/go-docx README（占位符碎片算法、SetFile 语义；仅处理 document.xml 与 header/footer part）。
- fumiama/go-docx README + LICENSE（AGPL-3.0 全文——**AGPL 对 Go 单体/内网分发同样触发开源义务，直接排除**）；其 README 自述"API is likely to change"。
- unioffice README（"This software package is a commercial product and requires a license code to operate"）+ LICENSE.md + 定价页（https://www.unidoc.io/pricing/ ，访问 2026-07-20；离线版明确"zero outbound network calls at runtime… key embedded at build time"，适合内网但要询价）+ 示例库 unioffice-examples（document/mail-merge 用 `d.MailMerge()` 填 MERGEFIELD 域；document/comment-remove；document/image；document/use-template；document/merge-documents）。

**JS**
- docxtemplater 定价页（https://docxtemplater.com/pricing/ ）："Open source — Free — {tag} replacement, Conditions, Loops, Change delimiters — MIT License"；Image 模块页（https://docxtemplater.com/modules/image/ ）："This module is available as part of the **Docxtemplater PRO plan**"，且该模块"You can also replace an existing image"。作为 sidecar 需 Node.js 运行时（官方 Get Started with Node.js）。

**.NET**
- Open XML SDK 总览（https://learn.microsoft.com/en-us/office/open-xml/open-xml-sdk ）+ repo https://github.com/dotnet/Open-XML-SDK （MIT、活跃）。能力完备但仅是低层 SDK（上面 API 参考页即为证据——颗粒度到单个元素），写成可用填充器等于自研方案 C 换语言；还要引入 .NET 运行时。作 sidecar 不如 docxtpl 现成。

---

## 4. 落地替换地图（基于模板实测）

实测数据见附录。原则：模板仅在已制备的槽位填充数据；样式、编号、主题、3 个 header part、7 个 footer part（含 footer6 的浮动图形）、3 个 `sectPr` 和域代码结构不得被业务渲染逻辑主动修改。docxtpl 会重新保存 DOCX 包，必须用渲染结果的结构审计与视觉比对证明版式保持，不能以 ZIP/XML 字节不变作为承诺。

### 4.1 文本槽（`XX` → Jinja 占位符 → ProjectReport 字段）

制备时**整词一次性键入**（保证单 run，§2.1），样式即所见即所得。`XX` 同名异义，禁止全局正则替换（template-analysis 已否决），逐槽人工替换：

| 位置 | 模板现状 | 占位符 | ProjectReport 字段 |
|---|---|---|---|
| 封面标题 | `XX电力有限公司安全渗透测试分析报告` | `{{ report_title }}` | `report_title`（默认按 client 生成，可人工覆盖） |
| 封面测试对象 | `测试对象：XX电力有限公司` | `{{ report.test_subject }}` | `test_subject`；复用源模板下划线 run |
| 封面日期/月 | `2022年X月X日`、`二零二二年X月` | `{{ report.project_created_date }}` / `{{ report.project_created_month }}` | Project `created_at` 不同格式；日期保留下划线、月份居中 |
| 封面测试人员 | `测试人员：XX` | `{{ report.testers_text }}` | `testers`（多人 join）；复用源模板下划线 run |
| 封面编制单位 | 编制单位行 | 固定文案 | 固定为“南京南瑞信息通信科技有限公司”，无占位符 |
| 概述/结论公司名 | `XX电力有限公司` 多处 | `{{ client_name }}` | `client_name` |
| 结论统计 | 漏洞总数/分级数 | `{{ conclusion.total/high/medium/low }}` | 由 confirmed Verifications 按用户批准句式动态计算 |
| 问题归纳 | `xx\xx\xx` | `{{ conclusion.focus_text }}` | confirmed 对应知识库漏洞名称去等级、去重、稳定排序后连接 |
| 概述中 IP 示例 | `xx.xx.xx.xx` 系列（实测 73 处 `xx`） | 删除示例句或参数化 | Zone 的 `access_point`/`tester_ip`/`targets` |
| 工具清单 | Hscan/AWVS 7/CANVAS 示例 | 固定正文 | 整个第二节原封不动保留，不从 Runs 生成 |

漏洞区块（Zone 循环内嵌套）：`{{ verification.heading }}`、`{{ verification.description }}`、`{{ verification.assets_text }}`、`{{ verification.remediation }}`；描述/建议直接复用现有 `VulnerabilityDelivery.Description/Remediation`（文本含 XML 特殊字符时按 docxtpl Escaping 章转义）。“漏洞详情”标题下只循环 `{{ evidence.image }}`，没有 `verification.detail`、独立“证据截图”标题或 caption。“其他漏洞验证不存在”使用未编号普通段落，不能消耗 Heading 3 编号。

### 4.2 表 3-1 动态行

§2.2 的做法：保留当前的 1 个数据模板行，额外增加只含 `{%tr for v in summary %}` 与 `{%tr endfor %}` 的两条控制行；四个 `{{ }}` 只放在数据模板行。标题行固定文字“表 3-1 渗透测试结果汇总表”（template-analysis 已明确不要沿用源文件破损分隔符）。

### 4.3 证据截图槽（2 张正文示例图 → 动态）

- 2 张正文示例截图**在制备时全部删除**（template-analysis：示例图片不进入交付报告）。
- 主策略 = **InlineImage 循环插入**（§2.3 策略 b）：漏洞详情内为每张 Evidence 预留图片槽。Python 侧只传入最大宽度 `Mm()`，由 `InlineImage` 保持原图纵横比；不依赖不可靠的截图 DPI 元数据。图片循环结构与 Zone 区块一起在原型中验证。
- 辅助策略 = 原位替换（§2.3 策略 a）：仅用于数量固定的槽。槽位标记用**图片替代文字（descr）**——Word 右键图片 → 编辑替代文字，填入如 `EVIDENCE_FIXED_TOPO`；`replace_pic` 源码证实按 `pic:cNvPr @descr/@name` 匹配（§2.3）。当前 2 张示例图若复用，必须先制备时重命名/重设。
- `wp:docPr`（Drawing Object Non-Visual Properties，https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.drawing.wordprocessing.docproperties ）的 name 亦可在 Word“选择窗格”中改名，作程序化定位的备选锚点。

### 4.4 Zone 章节（数量不固定）

先用原型验证一份 Zone 的“段落 + 表格 + 图片”混合区块能否由 docxtpl 重复；验证通过后才在模板内使用对应控制标签。若不通过，采用 Zone 独立子文档并用 docxtpl Subdoc 合并（`new_subdoc(path)`，§2.4）。Zone 在“测试结果与分析”内容章节内，重复时不触碰 3 个版式节的 `sectPr`。不预制 N 套 Zone：Zone 数和漏洞数都不固定，预制套数不解决核心问题。

### 4.5 不动清单 + 域 + 批注

- 页脚 7 part（含 footer6 的 `wps:wsp` 浮动文本框）、页眉 3 part：不放业务占位符，渲染后以结构审计和视觉比对确认保留。
- 域：TOC/HYPERLINK/PAGEREF/PAGE/STYLEREF 的 fldChar+instrText 结构全保留；settings.xml 补 `<w:updateFields w:val="true"/>`（§2.5）。
- 批注：制备时 Word 内一次删除（§2.7 方式 1）。
- 导出前完整性检查（与 technical-design.md 的单一检查点合并）：`get_undeclared_template_variables()` 对照 context、残留 `xx|XX` 扫描、每条 confirmed/not_observed 至少一张 Evidence 的既有规则。

### 4.6 一次性制备工序建议

**在 Word 里手工制备（推荐主路径）**：槽位语义需人工判断（同名 `XX` 异义），手工整词键入天然满足“单 run”约束，同时完成删 2 张示例图、表格控制行和域刷新设置。原型先验证表格、图片和 Zone 重复，再固化最终模板。
**程序化预处理（辅助）**：只在手工步骤无法可靠完成时补一个一次性脚本，例如向 settings.xml 加 `updateFields`。不要为当前无批注、仅 2 张图片的模板引入批量 OOXML 修改脚本。制备产物用 `get_undeclared_template_variables()`、结构审计和 Word 打开刷新验收，与源模板逐页比对。

---

## 5. 最终建议（Go 单体约束下）

### 5.1 推荐方案（唯一）

**python-docx + docxtpl sidecar**：Go 主程序把 ProjectReport 模型序列化为 JSON context → `uv run --project tools/docx-render python render_docx.py template.docx context.json out.docx` → doctor 工具运行同一 uv 环境中的 `import docxtpl` 检查。渲染失败/依赖缺失时仍可交付 HTML。

理由汇总：模板填充路线下能力完整；模板未填充结构可通过原型和渲染回归验证；活跃维护（2026-07-07 仍有提交）。docxtpl 为 LGPL-2.1，运行环境分发前须走许可证与源码获取的合规确认，不在设计文档中作法律结论。

### 5.2 降级路径（按顺序）

1. **lukasjarosch/go-docx（纯 Go，MIT）**：覆盖跨 run 文本占位 + 同格式图片原位替换。缺口用模板设计弥补：表 3-1 预制最大行数（如 40 行，多余行占位为空——视觉可接受度需用户确认）、Zone 预制 4 套（I/II/III/其他，按类别条件启用）、证据图改为“每漏洞固定 1 槽”原位替换。**注意其已 2 年未更新（2024-07-31），需自行验证对现代 Word 文件的鲁棒性。**
2. **unioffice（纯 Go，商业）**：能力最全（MailMerge、find-and-replace、merge-documents、图片插入），离线 license 不联网（官方定价页明确），但需采购询价；MailMerge 语义要求模板用 MERGEFIELD 域而非自由占位符。
3. docxtemplater sidecar：核心 MIT 但图片必须付费 PRO 模块（€1250/年起），且引入 Node.js 运行时——同样引入外部运行时，不如 Python 方案成熟且免费。

### 5.3 明确回答：“python-docx/docxtpl sidecar 是否是占位符替换路线下唯一成熟选择”

**是。** 在“开源免费 + 以下五项全占”的约束下无替代品。若坚持纯 Go，缺口清单（按必要度排序）：

1. **表格行克隆**（trPr/tcPr 保留的深拷贝）——Go 三库皆无；
2. **图片插入**（新 `w:drawing` + EMU 计算 + rel ID 分配）——仅 unioffice（商业）有；
3. **区块级循环**（段落序列按数据复制）——Go 三库皆无，docxtpl `{%p %}`/docxtemplater `{#loop}` 才有；
4. 跨 run 占位符——仅 lukasjarosch 有（且 2 年未维护）；nguyen 无（issue #40/#43 未修）；
5. 图片原位替换——nguyen/lukasjarosch 有（同格式限制）；
6. 域/浮动图形/节保留——任何“不动原 XML”的路线天然具备，一旦走“解析为结构体再序列化”（fumiama 路线）就有丢失风险（其 README 自述 Go xml parser 限制与 fork 原因）。

补齐 1–3 即等于自研方案 C（template-analysis 已评估为开发量最大），且需求的是对 OOXML 边角（rsid、proofErr、嵌套 fldChar）的长期踩坑——docxtpl 已踩完十年（2015 年创建）。

---

## 附录：模板文件本地实测记录（2026-07-20，只读）

对象：`/Users/kun/NARI/杂项/XX电力有限公司渗透测试报告模板-20220727-（等级保护渗透用）.docx`（当前 SHA-256 见 template-analysis.md）。方法：unzip 后直接检查 XML，与 template-analysis.md 核对一致：

- `word/media/`：2 张正文内嵌图；document.xml 中 2 处 `wp:inline`、0 处 `wp:anchor`。
- 浮动图形：`footer6.xml` 有 1 处 `wp:anchor`，内容为 `wps:wsp` 文本框（`txbxContent`，无图片引用）。
- 版式节：`document.xml` 含 3 个 `w:sectPr`。它们与报告的 4 个内容章节无对应关系。
- 域：10 处 PAGEREF、3 处 PAGE、1 处 STYLEREF；`settings.xml` **无** `updateFields`（制备时补）。
- 批注：不存在 comments part。
- 表：全文仅 1 张表，2 个 `w:tr`（1 表头 + 1 样本数据行），即表 3-1。
- Zone 标题块：3.1 I区、3.2 II区、3.3 III区共 3 块；模板制备时需收敛为可重复结构，但具体方式由原型验证决定。
- 包内其余 part：3 header、7 footer、styles/numbering/theme/fontTable、customXml×2、docProps×3。
