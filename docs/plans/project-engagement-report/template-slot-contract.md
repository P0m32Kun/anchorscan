# Project Report DOCX 模板槽位契约

**状态：accepted（2026-07-22）/ 已固化为正式运行时模板。**

本文定义正式报告模板中哪里可变、如何变化、数据从哪里来。已确认的运行时基线为：

- `tools/docx-render/templates/project-report.docx` 是受版本管理的正式运行时模板；
- Web 的 HTML 与 DOCX 导出消费同一项目交付读模型和完整性校验；
- WPS 需手动刷新目录，导出界面明确提示该操作；
- 用户提供的源 DOCX 保持只读，只能复制后制备实验模板。

## 1. 文档结构边界

正式报告沿用四个一级章节，顺序不可改变：

1. 概述；
2. 测试过程及方法；
3. 测试结果与分析；
4. 渗透测试结论。

所有网络分区、接入记录、测试结果、漏洞详情、未发现验证项和 Evidence 必须位于第三节内。第四节只包含结论，第四节之后不得再出现网络分区、接入点、测试范围、漏洞详情或 Evidence。

正式模板中只保留一个业务表格：**表 3-1 渗透测试结果汇总表**。验证详情按段落区块输出，不使用表格；不得制造“Zone / 接入点”表格。

目录、页码、章节号、表号交叉引用是 Word 域或编号结构，不是业务占位符。模板必须保留这些域，并设置打开文档时刷新。

## 2. 占位符一共四类

| 类别 | 标识 | 用途 | docxtpl 形式 |
|---|---|---|---|
| 文本槽 | `T` | 标题、日期、人员、名称、知识库描述、资产、修复建议、统计文本 | `{{ value }}` |
| 段落区块槽 | `B` | 网络分区、Run 接入记录、Verification、Evidence 图片的重复或条件显示 | `{%p for ... %}` / `{%p if ... %}` |
| 表格数据行槽 | `R` | 仅用于表 3-1 的动态数据行和空状态行 | `{%tr for ... %}` / `{%tr if ... %}` |
| 图片槽 | `I` | Evidence 原图，按顺序动态插入 | `{{ evidence.image }}` / `InlineImage` |

循环控制标签属于相应的 `B` 或 `R`，不另算业务数据类型。模板中不放示例图片；图片位置只保留可见的 `{{ evidence.image }}` 槽位。

## 3. 数据来源一共六组

| 来源 | 责任数据 | 当前状态 |
|---|---|---|
| `M` Project Report 元数据 | 被测单位、报告标题、测试对象、测试起止日期、测试人员 | 设计中；尚未实现 |
| `P` Project / Network Zone | Project 创建时间、网络分区名称和稳定排序 | Project 创建时间已有；网络分区规划持久化 |
| `R` 纳入报告的 Scan Runs | Run 名称、所属网络分区、接入点、测试机 IP、目标、排除项、备注 | Run 部分字段已有；网络分区/接入字段尚未实现 |
| `V` included Verification + 现有漏洞交付聚合 | 结论、正式标题、等级、关联资产、排序；复用现有 `Catalog.Entry` 匹配得到的描述与修复建议 | Verification 设计中；`BuildMatchedVulnerabilityDeliveries` 已实现知识库匹配与字段提取 |
| `E` Verification Evidence | 图片文件、顺序、尺寸、媒体类型 | 设计中；尚未实现 |
| `D` `BuildProjectReport` 派生值 | Project 创建日期/月、表 3-1、编号、去重资产、漏洞统计、问题名称摘要、实际纳入的网络分区清单 | 构建器尚未实现 |

模板固定文案、编制单位、Word 目录/页码域不从 JSON 读取，不属于以上数据源。

## 4. 逐位置槽位清单

### 4.1 封面

| ID | 源报告位置 | 类型 | 正式槽位 | 来源 | 必填与规则 |
|---|---|---|---|---|---|
| `C-01` | 主标题 | `T` | `report.title` | `M` | 必填；完整报告标题，不在模板中再拼公司名 |
| `C-02` | “测试对象”下划线 | `T` | `report.test_subject` | `M` | 必填；不得默认等于被测单位；复用源模板“测试时间”值的字符格式，与 `C-03/C-04` 使用同一粗下划线和等宽空格补齐规则，值位于整条线中间 |
| `C-03` | “测试时间”下划线 | `T` | `report.project_created_date` | `D(P.created_at)` | 必填；取 Project 创建日期，不另设报告日期；与左右补齐空格位于同一个粗下划线 run |
| `C-04` | “测试人员”下划线 | `T` | `report.testers_text` | `D(M.testers)` | 必填；多人使用统一分隔格式；复用 `C-03` 字符格式和等宽空格补齐规则，值位于整条线中间 |
| `C-05` | 页面底部月份 | `T` | `report.project_created_month` | `D(P.created_at)` | 必填；与 `C-03` 使用同一个 Project 创建时间，仅格式化到月份；复制用户格式刷后保存版本的段落缩进和段落标记字符格式，不另加“居中”样式 |

封面三条下划线不得使用 tab leader。每条线必须只有一个 `w:u w:val="thick"` 的数据 run，run 内容是“左侧不换行空格 + 数据 + 右侧不换行空格”；空格和数据共享完全相同的字符格式，因此不会出现两侧引导线与中间文字下划线粗细、高度不同的问题。渲染器按本模板 14 磅宋体的实际字符宽度补齐，使三条线长度基本一致且数据居中；结构门必须拒绝重新引入 `w:tabs`。

“南京南瑞信息通信科技有限公司”为已确认的模板固定文案，不设占位符。

### 4.2 目录

目录不放业务占位符。一级章标题、动态网络分区标题和页码由 Word TOC/PAGEREF/编号域刷新生成。

### 4.3 第一节“概述”

概述使用固定句式和三个数据槽，不保存一份与元数据重复、容易失真的整段副本：

> 根据 `{{ report.client_name }}` 的要求，对 `{{ report.test_subject }}` 开展安全渗透测试，测试时间为 `{{ report.test_period }}`。本报告汇总纳入报告范围内的测试与人工验证结果。

| ID | 类型 | 正式槽位 | 来源 | 必填与规则 |
|---|---|---|---|---|
| `O-01` | `T` | `report.client_name` | `M` | 必填；被测单位 |
| `O-02` | `T` | `report.test_subject` | `M` | 必填；复用封面同一字段 |
| `O-03` | `T` | `report.test_period` | `D(M.test_start, M.test_end)` | 必填；概述中的实际测试区间，独立于封面按 Project 创建时间显示的日期 |

原文“主要以人工检测的方法进行”不保留，因为系统不能仅凭模板证明实际采用了该方法。

### 4.4 第二节“测试过程及方法”

整个第二节（2.1、2.2、2.3）均为已确认的模板固定正文，原封不动保留，不设任何占位符，也不从 Run 或其他数据生成工具清单。

### 4.5 第三节“测试结果与分析”——表 3-1

表 3-1 是模板中唯一业务表格。表头固定，数据行由 `summary_rows[]` 生成：

| ID | 列 | 类型 | 正式槽位 | 来源 | 规则 |
|---|---|---|---|---|---|
| `S-01` | 数据行循环 | `R` | `summary_rows[]` | `D(V)` | 只消费 included `confirmed` Verification |
| `S-02` | 序号 | `T` | `row.number` | `D` | 稳定排序后从 1 开始 |
| `S-03` | 安全问题 | `T` | `row.title` | `V.title` | 不拼危险等级后缀 |
| `S-04` | 关联资产/域名 | `T` | `row.assets_text` | `D(V.assets)` | 规范化、去重、稳定排序；资产默认显示 `IP:port` |
| `S-05` | 严重程度 | `T` | `row.severity_label` | `D(V.severity)` | critical/high/medium/low 显示为中文标签 |
| `S-06` | 空状态 | `R` | `summary_empty` | `D` | 0 条 confirmed 时显示跨四列的“本次纳入报告范围内无已确认漏洞”说明；它不算漏洞数据行 |

汇总身份使用稳定知识库 Entry ID 或人工 Verification 稳定键，不使用可编辑标题。相同漏洞跨 Runs/Zones 的重复资产只显示一次。

### 4.6 第三节——网络分区

`Zone` 是 **Network Zone（网络分区）** 的代码用语，对应报告里的“I区、II区、III区”或甲方定义的其他分区。模板正文只显示具体分区名称，不显示“Zone”英文标签。

Project 内的每个 Scan Run 必须且只能选择一个网络分区。创建扫描任务时提供 I区、II区、III区和自定义分区；报告按所属分区把该 Run 的接入记录、Verification 和 Evidence 写入第三节对应位置。

只输出至少包含一个纳入报告的 Run 或 included Verification 的网络分区。Project 只有一个或两个分区时，只生成实际存在的分区，不补空的 I/II/III 区标题。排序固定为 I区、II区、III区在前，自定义分区随后按 `position` 和名称稳定排序。

| ID | 位置 | 类型 | 正式槽位 | 来源 | 必填与规则 |
|---|---|---|---|---|---|
| `Z-01` | 网络分区区块起止 | `B` | `network_zones[]` | `D(P/R/V)` | 只包含实际有纳入内容的分区；按上述规则排序 |
| `Z-02` | 二级标题 | `T` | `network_zone.heading` | `D(P.name)` | 使用 Heading 2，编号和目录由 Word 刷新 |
| `Z-03` | 接入记录循环 | `B` | `network_zone.sessions[]` | `R` | 一个纳入报告的 Scan Run 对应一组段落 |
| `Z-04` | 运行名称 | `T` | `session.label` | `R.label` 或 `D` | 空值时派生稳定标签 |
| `Z-05` | 接入点 | `T` | `session.access_point` | `R.access_point` | Scan Run 必填；不得提升为 Zone 共用字段 |
| `Z-06` | 测试机 IP | `T` | `session.tester_ip` | `R.tester_ip` | Scan Run 必填 |
| `Z-07` | 测试范围 | `T` | `session.targets_text` | `D(R.targets)` | Scan Run 必填；保留可审计目标范围 |
| `Z-08` | 排除范围 | `T+B` | `session.exclusions_text` | `D(R.exclusions)` | 空值时整段隐藏 |
| `Z-09` | 备注 | `T+B` | `session.notes` | `R.notes` | 空值时整段隐藏 |

可见模板形态是普通段落，不是表格，而且必须完整结束在第四节标题之前：

```text
3.x {{ network_zone.name }}
接入记录：{{ session.label }}
接入点：{{ session.access_point }}
测试机 IP：{{ session.tester_ip }}
测试范围：{{ session.targets_text }}
排除范围：{{ session.exclusions_text }}   （可选）
备注：{{ session.notes }}                 （可选）
```

### 4.7 第三节——已确认漏洞区块

每条 included `confirmed` Verification 在所属网络分区内生成一个完整区块。

| ID | 位置 | 类型 | 正式槽位 | 来源 | 必填与规则 |
|---|---|---|---|---|---|
| `V-01` | 漏洞区块起止 | `B` | `network_zone.confirmed[]` | `V` | 按等级、人工 position、标题稳定排序 |
| `V-02` | 三级标题 | `T` | `verification.heading` | `D(V.title, V.severity)` | 标题中显示正式标题和等级；每条 confirmed 只生成一个 Heading 3，因此一个漏洞只有 3.x.1，两个漏洞才出现 3.x.2，以此类推 |
| `V-03` | 漏洞描述 | `T` | `verification.description` | 现有 `VulnerabilityDelivery.Description` ← `Catalog.Entry.Description` | 必填；复用已有知识库解析与匹配，不新建一套提取逻辑 |
| `V-04` | 漏洞详情 | `B+I` | 固定标题下循环 `verification.evidence[]` / `evidence.image` | `E` | “漏洞详情”就是证据截图区，只放图片；没有 `verification.detail` 文本，也没有“证据截图”二级标题或图片说明 |
| `V-05` | 关联资产 | `T` | `verification.assets_text` | `D(V.assets)` | 必填；稳定排序，可保留协议和来源上下文 |
| `V-06` | 修改建议 | `T` | `verification.remediation` | 现有 `VulnerabilityDelivery.Remediation` ← `Catalog.Entry.Remediation` | 必填；复用已有知识库解析与匹配 |

现有 `internal/knowledgebase/parse.go` 已从知识库固定章节“漏洞描述”“修复建议”解析为 `Entry.Description/Remediation`；`internal/report/vulnerability_delivery.go` 已把它们映射为 `VulnerabilityDelivery.Description/Remediation`。正式 DOCX 构建器必须复用这条链路，不增加 `detail` 字段来重复或覆盖知识库内容。

### 4.8 第三节——“其它漏洞验证不存在的截图”区块

每条 included `not_observed` Verification 位于所属网络分区的 confirmed 区块之后：

| ID | 位置 | 类型 | 正式槽位 | 来源 | 必填与规则 |
|---|---|---|---|---|---|
| `N-01` | 未发现区块起止 | `B` | `network_zone.not_observed[]` | `V` | 空数组时整个区块隐藏；`inconclusive` 不进入正式报告 |
| `N-02` | 分区汇总三级标题 | `T` | `network_zone.name` | `D(P.name)` | 有 `not_observed` 时，在该分区所有 confirmed 漏洞之后生成“`{{ network_zone.name }}`其它漏洞验证不存在的截图：”；使用与漏洞名相同的 Heading 3 大纲级别，但不显示、也不占用 3.x.y 编号；该区块结束后才进入下一个分区 |
| `N-03` | 图片前说明 | `T` | `verification.title`、`verification.ports_text` | `V.title`、`D(V.assets.port)` | 每条显示“`{{ verification.title }}`不存在证明，端口（`{{ verification.ports_text }}`）”，位于该条 Evidence 图片之前 |
| `N-04` | Evidence 循环 | `B` | `verification.evidence[]` | `E` | 至少 1 张，按 `position`；每条说明之后顺序插入图片 |
| `N-05` | Evidence 图片 | `I` | `evidence.image` | `E.relative_path` | 只放截图，不再增加图片 caption 或验证过程正文 |

源模板中的两张示例截图必须从实验模板中删除，不能作为占位图片保留。

### 4.9 第四节“渗透测试结论”

第四节由一段自动统计和一段固定整改文案组成，之后文档结束。模板句式必须是：

> 本次测试共测试`{{ report.client_name }}``{{ conclusion.network_zone_names_text }}`所有设备，共发现漏洞`{{ conclusion.total }}`个，其中严重漏洞`{{ conclusion.critical }}`个、高危漏洞`{{ conclusion.high }}`个、中危漏洞`{{ conclusion.medium }}`个、低危漏洞`{{ conclusion.low }}`个。其中问题主要集中在`{{ conclusion.focus_text }}`这几个方面。
>
> 需要及时整改，加强安全管理规范，并进行复测，预防安全事故发生。

| ID | 位置 | 类型 | 正式槽位 | 来源 | 必填与规则 |
|---|---|---|---|---|---|
| `K-01` | 被测单位 | `T` | `report.client_name` | `M` | 与概述复用同一字段 |
| `K-02` | 实际网络分区 | `T` | `conclusion.network_zone_names_text` | `D(network_zones[])` | 只列实际生成的分区，用 `、` 连接 |
| `K-03` | 漏洞总数 | `T` | `conclusion.total` | `D(V)` | 等于表 3-1 数据行数，不按资产数计 |
| `K-04` | 分级数量 | `T` | `conclusion.critical/high/medium/low` | `D(V.severity)` | 严格按批准句式输出严重/高/中/低四个计数；严重对应 Nuclei `critical`，不得降级到高危 |
| `K-05` | 问题集中方面 | `T` | `conclusion.focus_text` | `D(VulnerabilityDelivery.Title)` | 从 included confirmed 的知识库漏洞名称去等级、去重、稳定排序后用 `\` 连接；不引入人工自由文本，也不猜不存在的分类 |

## 5. Evidence 图片规则

- 模板中图片数量必须为 0；只显示 `{{ evidence.image }}` 文本槽。
- 渲染结果按 `Evidence.position` 升序插入。
- 同一 Verification 的图片不得穿插到另一条 Verification。
- 同时限制最大宽度和最大高度，在限制框内等比例缩放；禁止固定宽高拉伸。
- PNG/JPEG 的原始像素、SHA-256、媒体类型和宽高来自 Evidence 元数据；模板不读取任意绝对路径。
- 报告中不输出图片说明；“漏洞详情”标题下按顺序直接放图。

## 6. 非占位内容与保留门

下列内容不是业务占位符，模板制备和渲染必须保留：

- 3 个 `w:sectPr`；
- 3 个页眉 part；
- 7 个页脚 part；
- `footer6.xml` 的浮动图形结构；
- TOC、PAGEREF、PAGE、STYLEREF 等域结构；
- 模板样式、编号、主题、页面尺寸和页边距；
- 固定章节标题与固定说明文字。

结构保留不代表模板业务内容正确；必须先通过本文槽位契约，再制作模板并执行结构/视觉/WPS 验收。

## 7. 产品决定状态

1. **已确认——封面时间**：封面日期与页面底部月份都取 Project 创建时间，只做不同格式化；不新增 `report_date`。
2. **已确认——编制单位**：“南京南瑞信息通信科技有限公司”是固定模板文案。
3. **已确认——工具介绍**：2.2 原文固定不动，不新增 `manual_tools`，不从 Runs 生成。
4. **已修订——漏洞内容来源与详情区**：漏洞描述、修改建议复用已有知识库解析/匹配结果；“漏洞详情”仅放 Evidence 图片，不存在 `verification.detail` 文本和图片说明。
5. **已确认——网络分区**：每个 Project Scan Run 必须归属 I区、II区、III区或自定义分区；第三节只生成实际有纳入内容的分区。
6. **已确认——固定方法文案**：整个第二节（2.1、2.2、2.3）原封不动保留，不从数据库生成。

7. **已修订——漏洞编号**：只有 confirmed 漏洞使用 Heading 3；未发现项不得占用 3.x.y，因此编号数量严格等于该分区的已发现漏洞数。
8. **已修订——结论句式**：使用用户给定的两句文案与统计槽，包含 `conclusion.critical`、高/中/低计数；不再使用 `has_confirmed` 或人工 `conclusion.text` 槽。

正式模板已据此生成、验收并固化为 `tools/docx-render/templates/project-report.docx`。
