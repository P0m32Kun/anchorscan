# DOCX 模板结构分析与报告映射

> **状态纠正（2026-07-21）：** 本文的旧“占位槽映射”和原型描述只作为源文档分析记录，不是已批准的正式模板契约。用户已否决现有实验模板。正式逐位置槽位、占位符类型、数据来源和空值规则以 `template-slot-contract.md` 为唯一待审草案；批准前不继续制作 DOCX 模板。

## 分析边界

### 原始模板（历史参考）

参考文件：`/Users/kun/NARI/杂项/XX电力有限公司渗透测试报告模板-20220727-（等级保护渗透用）.docx`

原 sha256：`fc5f42440b1b6721be1efd1ba722e770fe15e1572d3856cb9e63f5f86d94fac1`。历史只读检查（2026-07-20 第一轮）：20 页、4 个 Section、A4 纵向、左右页边距约 30mm、上下约 20mm；正文 15 张内嵌图片、页脚 2 个浮动图形；2 条编制批注、10 个 PAGEREF、5 个 PAGE、1 个 STYLEREF 字段；无可用作业务槽位的内容控件；汇总表为表头+7 空行。该版本不适合直接当运行时填充模板（122 处 run 级、251 处 paragraph 级直接格式覆盖）。

### 当前源模板（用户简化版，运行时蓝本）

用户已简化模板，现 sha256：`e5f6a89663065737db00c1aa94ab97b86da308c8a75571db5a4e19c238a2c6e5`（2026-07-20 只读复核，attr-tolerant 重数）：

| 项 | 原始 | 简化版 |
|---|---|---|
| 正文图片 | 15（`wp:inline`）+ 页脚 2 浮动 | **2**（均 `wp:inline`，body 无 `wp:anchor`）|
| 批注 | 2 | **0**（`word/comments.xml` 不存在）|
| `w:sectPr` | 4 | **3**（document.xml 计数；这是版式节，不等同于文档内容章节）|
| 汇总表 `w:tbl`/`w:tr` | 表头+7 空行 | **1 表，表头+1 样本行**（`tr`=2，`tc`=8）|
| 页眉 part | 3 | 3（header1–3）|
| 页脚 part | 7 | 7（footer1–7）|
| PAGEREF | 10 | 10（目录 `_Toc21060`/`_Toc24649`/`_Toc29966` 对应 3.1 I区/3.2 II区/3.3 III区）|
| STYLEREF | 1 | 1 |
| `w:fldChar` / `w:instrText` | — | 66 / 22（TOC 与交叉引用域）|
| `XX`（大写） | — | 11 |
| `xx`（小写） | — | 91（公司名、IP 段、漏洞数、问题类别等同名异义混用）|
| settings.xml `updateFields` | 无 | 无（制备时需补）|
| zip 条目 | — | 40 |

该简化版即 docxtpl 占位符模板的制备蓝本（ADR-0005）。报告内容仍有 4 个一级章节：概述、测试过程及方法、测试结果与分析、渗透测试结论；它们不按 `w:sectPr` 计数。

> 决策更新（2026-07-20 第二轮）：用户确认最终交付为 HTML + DOCX 双格式。以下“不适合作为运行时模板”的结论仍然成立——它约束的是不要直接 patch 这份原始文件；新方案是以它为蓝本一次性制作干净的 DOCX 模板/样式模板，再由程序填充。

- SHA-256：`fc5f42440b1b6721be1efd1ba722e770fe15e1572d3856cb9e63f5f86d94fac1`
- 只读检查日期：2026-07-20
- 共 20 页、4 个 Section，均为 A4 纵向；左右页边距约 30 mm，上下约 20 mm。
- 正文有 15 张内嵌图片，页脚另有 2 个浮动图形。遵照用户要求，只确认数量、位置类型和版式占用，不读取或判断截图内容。
- 文档包含 2 条编制批注、10 个 PAGEREF、5 个 PAGE 和 1 个 STYLEREF 字段；没有可作为业务槽位的带 tag 内容控件。
- 汇总表只有 1 个固定表格，表头加 7 个占位数据行，不支持动态行数。
- 文档大量依赖直接格式：122 个 run 级、251 个 paragraph 级格式覆盖。它适合作为视觉和内容结构参考，不适合作为稳定的运行时填充模板。

## 模板语义结构

1. 封面：报告标题、测试对象、测试时间、测试人员、编制单位和编制月份。
2. 目录：概述、测试过程及方法、测试结果与分析、I/II/III 区、测试结论。
3. 概述：被测单位、测试范围、测试时间和方法摘要。
4. 测试过程及方法：渗透测试概念、工具介绍、测试过程。
5. 测试结果与分析：表 3-1 动态汇总，然后按 I、II、III 区展开。
6. Zone 详情：现场接入信息、测试范围、已确认漏洞详情、其他漏洞验证未发现截图。
7. 测试结论：漏洞总数和分级统计、主要问题归纳、整改建议。

其他 Zone 应复用相同结构，不受模板只列三个区的限制。目录在 HTML 中使用锚点导航和打印样式，不复刻 Word 的静态 PAGEREF 字段。

## 占位槽映射

| DOCX 内容 | HTML 数据来源 | 完整性规则 |
|---|---|---|
| `XX电力有限公司安全渗透测试分析报告` | `ProjectReport.report_title` | 必填，默认由被测单位生成，可人工覆盖 |
| `测试对象：XX电力有限公司` | `ProjectReport.test_subject` | 必填，不假设等同被测单位；保留源模板下划线 |
| `2022年X月X日`、`二零二二年X月` | Project `created_at` 的日期/月格式 | 必填；日期保留下划线，月份居中 |
| `测试人员：XX` | `testers` | 必填，支持多人；保留源模板下划线 |
| 概述中的 `XX电力有限公司` | `client_name/test_subject` | 模板文本参数化，不做全局字符串替换 |
| Zone 接入点、测试机 IP、范围 | 每个纳入 Run 的 `access_point/tester_ip/targets/exclusions/notes` | 同一 Zone 多 Runs 分行，不覆盖或拼成不可审计长文本 |
| 漏洞标题、描述、详情、资产、建议 | included `confirmed` Verification + 现有 `VulnerabilityDelivery` | 描述和建议复用知识库字段；“漏洞详情”只放至少一张 Evidence 图片，不输出过程文本或图片说明 |
| “其他漏洞验证不存在” | included `not_observed` Verification | 使用未编号普通段落，只放截图，不占用 3.x.y 漏洞编号 |
| 结论中的漏洞总数和等级数 | included `confirmed` Verifications | 按用户批准句式动态计算总数和高/中/低计数 |
| 主要问题集中在 `xx\xx\xx` | confirmed 对应的知识库漏洞名称 | 去等级、去重、稳定排序后以 `\` 连接，不另设人工自由文本 |

不能采用正则把所有 `XX` 一次替换为公司名，因为同一符号同时表示公司、日期、人员、设备、IP、漏洞数量和问题类别。应以明确字段渲染，并在导出前检查缺失字段与残留占位模式。

## 表 3-1 动态规则

标题固定显示“表 3-1 渗透测试结果汇总表”，不要沿用源文件渲染成“表 31”的破损分隔符。表体完全由 Project Report 构建器生成，不能复制源文档的 7 个空行。

| 列 | 规则 |
|---|---|
| 序号 | 按报告稳定排序从 1 开始 |
| 安全问题 | Verification 的正式标题，不把危害等级拼进标题 |
| 关联资产/域名 | 聚合所有纳入 Runs/Zones 中确认存在该漏洞的唯一 `IP:port`；规范化、去重、稳定排序 |
| 严重程度 | Verification 最终危害等级 |

聚合键优先使用知识库 Entry ID，人工条目使用稳定 Verification 身份。标题是可编辑展示字段，不能作为去重键。相同漏洞在多次扫描中重复命中同一端点只显示一次；不同端点全部保留。详情区仍记录 Zone、来源 Run 和协议，保证可追溯。

## 动态重复块

源 DOCX 为每个 Zone 固定放置 4 个漏洞示例，并在 I 区列出一组固定的负向验证示例。正式 HTML 不保留这些固定数量或固定漏洞名，使用以下重复结构：

```text
Project Report
  for Zone in selected Zones
    Zone scope/access sessions (one row per included Run)
    for Verification in confirmed
      vulnerability detail
      ordered Evidence
    for Verification in not_observed
      validation item + endpoints + note
      ordered Evidence
```

模板批注明确要求所有图片替换为真实验证截图，并尽可能保留已验证对象的证据；另一条批注要求测试范围与资产范围一致，并标注范围外资产。新模型因此需要保存来源 Run、关联资产和范围说明，不能只把截图挂在最终 HTML 上。

## Evidence 保存与报告位置

- Evidence 归属于一条 Verification，不归属于 Finding、DetectionCheck 或整个 Project。
- 用户从候选项打开工具页时携带 Project、Zone、Verification 和返回地址；执行后直接在当前验证项粘贴、拖拽或选择 PNG/JPEG。
- confirmed 详情中的图片位于“关联资产”之后、“修改建议”之前，按用户排序并显示可选说明。
- not_observed 图片位于该验证项的资产和验证说明之后；页面标题不得写成绝对的“不存在漏洞”。
- 服务端只验证文件类型、尺寸、哈希和归属路径；不做 OCR，不分析截图内容，也不自动决定结论。
- HTML 使用经过验证的 data URI 内嵌原图，CSS 只做等比例缩放和分页控制，保证单文件离线交付。

## 不应原样继承的内容

- 工具介绍包含 Hscan、AWVS 7、CANVAS 等历史示例。正式报告必须从实际纳入 Runs 的配置和执行事实生成真实工具清单，允许人工补充但不能凭模板宣称使用过某工具。
- “不存在漏洞”措辞过强，统一改为“本次验证未发现”。
- 固定 I/II/III、固定 4 个漏洞块、固定 7 行汇总表都必须改为动态循环。
- 静态目录页码、Word 书签和 PAGEREF 不适用于单文件 HTML，替换为语义 heading、锚点目录和打印 CSS。
- 源文件中的编制批注和示例图片不进入交付报告。

## 工具选择结论

HTML 继续采用现有 Go `html/template` 构建 Project Report，使用 `go:embed` 内嵌 CSS，并用浏览器标准 Clipboard/File/Drag-and-Drop API 和 multipart 保存 Evidence。

DOCX 已调研定案（见 `docx-rendering-research.md`，一手来源）：采用模板填充路线——以本文件为蓝本一次性制备干净模板，运行时只填充指定文本、动态复制表格数据行并插入/替换图片；以渲染回归测试验证未填充部分的版式和结构。Pandoc + reference.docx 被官方手册原文否决（reference-doc 只提供样式与页面属性，正文 XML 全部重建，直接格式/节结构/浮动图形/域必丢）。渲染实现采用 python-docx + docxtpl sidecar，经 `uv run` 编排并由 doctor 检查；逐槽位地图见调研报告 §4。不引入 OCR、图像数据库或新的前端构建框架。
