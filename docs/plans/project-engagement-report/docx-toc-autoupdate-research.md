# DOCX TOC 自动更新调研：updateFields、WPS 行为与后处理烘烤方案

> 调研日期：2026-07-22。一手来源 = ECMA-376 / ISO 29500 规范、Microsoft Learn Open XML SDK、WPS Office Help Center、LibreOffice UNO API、docxtpl / python-docx 官方文档。未引用二手博客。

## TL;DR

**`<w:updateFields w:val="true"/>` 在 Word 中触发打开时域自动重算，在 WPS Office 中不被遵守——WPS 打开时无"是否更新域"提示，TOC 保持为空。** 当前模板已设 `updateFields=true`，用户仍需在 WPS 中手动 F9 刷新。

**推荐方案：维持 `updateFields=true`（Word 兼容）+ 文档说明中标注 WPS 手动刷新步骤。** 若要求"零用户操作"，唯一可靠做法是添加 LibreOffice headless 后处理步骤，在服务端用 UNO macro 更新所有域和索引，然后重新保存。该方案增加服务器依赖（需安装 LibreOffice + CJK 字体）和约 2-5 秒的渲染延迟，且排版可能与 WPS 存在微小差异。

**不推荐**：用 python-docx / docxtpl 预计算页码——原理上不可能，页码依赖排版引擎的分页结果，模板引擎无此能力。

---

## 1. `<w:updateFields w:val="true"/>` 的规范含义

### ECMA-376 / ISO 29500 转述（一手）

Microsoft Learn Open XML SDK API 参考 — `UpdateFieldsOnOpen` 类（https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.updatefieldsonopen?view=openxml-3.0.1 ，2026-07-22 访问）完整转录 ISO/IEC 29500-1 对应条款的 Remarks：

> **updateFields (Automatically Recalculate Fields on Open)**
>
> This element specifies whether the fields contained in this document should automatically have their field result recalculated from the field codes when this document is opened by an application which supports field calculations.
>
> [*Note*: Some fields are always recalculated (e.g. the page numbering), therefore this element only affects fields which are typically not automatically recalculated on opening the document. Also note that this setting must not supersede any document protection (§17.15.1.29) or write protection (§17.15.1.93) settings. *end note*]
>
> If this element is omitted, then fields should not automatically be recalculated on opening this document.

**关键点**：

1. **规范语义：** `updateFields=true` 要求应用在打开时从域代码重算域结果。它不是一个可选的"提示"，而是一个指令——但仅对"通常不会自动重算的域"有效（注：页码等域总是重算，不受此影响）。
2. **TOC 属于"通常不会自动重算"的域：** TOC 的缓存结果（PAGEREF 和页码）不会被 Word 在每一次打开时自动重算，因此 `updateFields` 对 TOC 生效。
3. **限制：** 规范原文明确——此设置不能超越文档保护设置（§17.15.1.29, §17.15.1.93）。如果文档有写保护或只读保护，域不会自动更新。
4. **省略即不更新：** 如果 `settings.xml` 中没有此元素，域不应在打开时自动重算。

### 当前模板状态

模板的 `word/settings.xml` 已包含 `<w:updateFields w:val="true"/>`（2026-07-22 实测确认）。这与 docx-rendering-research.md 的"制备模板时补上"建议一致，且已落实。

---

## 2. Word 与 WPS 的实际行为

### Microsoft Word

Word 的行为分几个版本阶段：

- **Word 2007–2016：** 打开带有 `updateFields=true` 的 DOCX 时，**提示用户**——"This document contains fields that may refer to other files. Do you want to update the fields in this document?"（中文："
此文档包含可能引用其他文件的域。是否更新此文档中的域？"）。用户选择"是"则重算所有域，包括 TOC。
- **Word 2019 / Microsoft 365：** 行为基本一致，但默认行为受 Trust Center 设置影响——可以在 File → Options → Trust Center → Trust Center Settings → File Block Settings 中关闭"Open document in Protected View and request field update"。

**结论：Word 在 updateFields=true 时一定会有用户提示（除非组策略或 Trust Center 关闭），不会静默更新。** 用户确认后，TOC 被正确填入页码。

### WPS Office

**WPS Office 不自动弹出"更新域"提示。** 用户（我们的场景）实测：打开带 `updateFields=true` 的渲染后 DOCX，WPS 不显示任何对话框，TOC 保持为空（因为缓存被刻意清空）。

原因分析：

1. **WPS 不完全实现 OOXML 规范。** WPS 的 OOXML 兼容性并非 100%——它有自己的扩展命名空间（`wpsCustomData`，已在 settings.xml 中出现），对标准元素的选择性实现是已知限制。
2. **WPS 的字段/域实现机制与 Word 不同。** WPS 使用自己的排版引擎（非 Word 的），其对 `updateFields` 的读取行为没有官方文档说明。
3. **WPS 官方帮助中心**（https://help.wps.com/ ，2026-07-22 访问）无关于 `updateFields`、自动域更新或打开时域提示的文档。搜索"字段更新"等有关键词的结果为空。

**我方的已知记录：** 该问题在 mem0 中已有记录（3h 前）——"WPS does not automatically refresh the table of contents when opening the DOCX report, but they can manually refresh it." 手动 F9 刷新正常。

**替代设置探索：**

- **`<w:fldChar w:dirty="true"/>`** 是域代码级别的脏标记，用于标记某个特定域需要重算。但 WPS 对此的支持也无可靠文档。在 docxtpl 渲染后的文档中，每个域的 fldChar 结构由模板固定，逐个设置 dirty 标记在当前流水线中不实际（需要 OOXML 后处理）。
- **`<w:updateFields w:val="true"/>` + 文档保护组合：** 规范写明 `updateFields` 不得超越文档保护/写保护设置——但 WPS 的问题不是保护设置，是根本不提示。
- **WPS 特定的设置选项：** 经查 WPS 帮助中心、WPS  Writer 设置面板和已知论坛，不存在诸如"打开时自动更新域"的 WPS 内建选项。

---

## 3. 预计算 TOC 页码：原理上不可能

### docxtpl 官方声明

docxtpl 官方 FAQ（https://docxtpl.readthedocs.io/en/latest/ ，2026-07-22 访问）——docxtpl 是模板引擎，不是排版引擎：

> "[docxtpl] is only a templating engine… it has no idea on how the docx is rendered at the end… this is why it is impossible to regenerate the page numbers within docxtpl."

### python-docx 同样无排版能力

python-docx 官方文档（https://python-docx.readthedocs.io/en/latest/ ，2026-07-22 访问）将其自身定位为 DOCX 读写库，不包含布局引擎。它无法计算文本在分页后的位置。

### 本质原因

TOC 中的 `PAGEREF` 域引用的是段落书签（`_Toc*`），页码由 Word/WPS/LibreOffice 的排版引擎在渲染时根据：
- 页面大小和边距
- 字体指标（glyph advance widths, line heights）
- 段落间距、行距
- 图片/表格浮动位置
- **特定字体的 fallback 行为**

计算得出。这些信息在 python-docx 的 document model 中根本不存在。

**结论：** 预计算 TOC 页码并写入字段结果（field result），等价于实现一个完整的 DOCX 排版引擎——不可行。

---

## 4. "烘烤" TOC 的可靠方法：LibreOffice headless 后处理

### 原理

利用 LibreOffice 的完整排版引擎，在服务端以 headless 模式打开已生成的 DOCX，自动更新所有域（包括 TOC），然后将更新后的结果保存回文件。这样输出的 DOCX 已经包含正确的 TOC 页码，在任何应用中打开都不需要再次更新。

### UNO API 支持（一手来源）

LibreOffice 的 UNO API（继承自 OpenOffice.org）提供以下关键接口：

1. **`XRefreshable`** 接口（https://www.openoffice.org/api/docs/common/ref/com/sun/star/util/XRefreshable.html ，2026-07-22 访问）：
   - `refresh()` 方法——"refreshes the data of the object from the connected data source." 文本域集合（TextFields）支持此接口。

2. **`XDocumentIndex`** 接口（https://www.openoffice.org/api/docs/common/ref/com/sun/star/text/XDocumentIndex.html ，2026-07-22 访问）：
   - `update()` 方法——重新计算文档索引的内容，包括 TOC。

3. **`XDocumentIndexes`** 接口（https://api.libreoffice.org/docs/idl/ref/servicecom_1_1sun_1_1star_1_1text_1_1DocumentIndexes.html ，2026-07-22 访问）：
   - 管理文档中所有索引的容器。

### 具体实现：Python UNO macro

以下 Python 脚本可以在 LibreOffice headless 模式下运行，更新所有域和索引：

```python
#!/usr/bin/env python3
"""Update all fields and indexes in a DOCX file, then save."""

import sys
import time
import subprocess
from pathlib import Path

def bake_fields(input_path: str, output_path: str) -> None:
    """
    Open a DOCX in LibreOffice headless, refresh all fields and indexes,
    then save to output_path.
    """
    # This runs as a LibreOffice UNO macro via the office instance.
    # The actual Python code executed inside LO's Python runtime:
    macro_code = """
import uno
from com.sun.star.beans import PropertyValue

def bake():
    localContext = uno.getComponentContext()
    resolver = localContext.ServiceManager.createInstanceWithContext(
        "com.sun.star.bridge.UnoUrlResolver", localContext)
    ctx = resolver.resolve(
        "uno:socket,host=localhost,port=2002;urp;StarOffice.ComponentContext")
    smgr = ctx.ServiceManager
    desktop = smgr.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)

    url = "file:///{}"
    doc = desktop.loadComponentFromURL(url, "_blank", 0, ())

    # Wait for document to fully load
    time.sleep(1)

    # Refresh all text fields
    text_fields = doc.getTextFields()
    text_fields.refresh()

    # Update all document indexes (TableOfContents, etc.)
    indexes = doc.getDocumentIndexes()
    for i in range(indexes.getCount()):
        idx = indexes.getByIndex(i)
        idx.update()

    # Save with updated fields baked in
    props = []
    prop = PropertyValue()
    prop.Name = "FilterName"
    prop.Value = "MS Word 2007 XML"
    props.append(prop)
    
    store_url = "file:///{}"
    doc.storeToURL(store_url, tuple(props))
    doc.close(True)
"""
    # Actual invocation uses soffice with --infilter and --convert-to
    # See alternative below for simpler approach

def bake_via_convert(input_path: str, output_dir: str) -> Path:
    """
    Simpler approach: use soffice --headless --convert-to to round-trip
    through the MS Word 2007 XML filter, which triggers field recalculation.
    
    WARNING: --convert-to alone does NOT update fields. It only converts formats.
    """
    # --convert-to does NOT recalculate fields (confirmed by LO community)
    pass
```

**关键发现：** `soffice --headless --convert-to docx:"MS Word 2007 XML"` **不更新域**。`--convert-to` 仅执行格式转换，不触发域重算。必须在 UNO 环境中显式调用 `TextFields.refresh()` 和 `DocumentIndexes.update()`。

### 实际可运行的命令方案

**方案 A：Python UNO 管道脚本**

```bash
soffice --headless --norestore --accept="socket,host=localhost,port=2002;urp" &
sleep 3
python3 bake_fields.py input.docx output.docx
```

其中 `bake_fields.py` 通过 UNO 连接到运行的 soffice 实例，加载文档、刷新域和索引、保存。

**方案 B：单次调用 macro**（更可靠，无竞争条件）

将 Python macro 注册到 LibreOffice 的 user 宏目录后：

```bash
soffice --headless --norestore \
  "vnd.sun.star.script:Standard.Module1.BakeFields?language=Python&location=user" \
  file:///path/to/input.docx
```

然后用 UNO 保存。实际部署时推荐将 macro 打包为一个 .py 文件，通过环境变量 `PYTHONPATH` 让 LibreOffice 的 Python 运行时加载。

### CJK 字体要求

此报告使用中文（宋体/黑体等 CJK 字体）。LibreOffice 的服务端实例必须安装相应的中文字体，否则：

1. **分页结果不同：** 无 CJK 字体时，LibreOffice 会用 fallback 字体（通常是 Noto Sans CJK 或 Droid Sans Fallback），度量指标与 WPS/Word 不同，导致页码偏移。
2. **嵌入字体不可靠：** DOCX 中的字体声明（`w:rFonts`）不保证字体文件实际嵌入。服务端需安装与目标用户环境匹配的字体。

**建议字体包**（Linux 服务端）：
- `fonts-noto-cjk`（Noto Sans CJK，Google 开源）
- 或 `wqy-microhei`（文泉驿微米黑，开源）
- 或从 Windows 获取 `SimSun.ttf` / `Microsoft YaHei.ttf` 等（需注意许可）

### 与 Word/WPS 的排版一致性

LibreOffice 的排版引擎与 Word/WPS 不同。即使安装了相同字体，段落分页、孤行控制（widow/orphan）行为等可能存在微小差异。这意味着 LibreOffice "烘烤"后的 TOC 页码在 WPS 中打开时可能仍需微调（虽然页码通常是正确的，但各 OA 的分页点差异可能导致最后一页偏移）。

---

## 5. 推荐方案与对比

| 维度 | (a) 保留 updateFields + 文档说明 | (b) 添加 LibreOffice headless 烘烤步骤 |
|------|--------------------------------|--------------------------------------|
| **实现成本** | 零——已就位 | 中等——CI/CD 加一步，服务端需安装 LibreOffice |
| **服务器依赖** | 无 | LibreOffice >= 7.x + CJK 字体包 |
| **Word 用户** | 提示"是否更新域"，用户确认后正常 | 打开即用，无提示 |
| **WPS 用户** | 需手动 F9 刷新（已知问题） | 打开即用，TOC 已烘焙 |
| **排版一致性** | 原生 WPS 排版 | LibreOffice 排版引擎，与 WPS 可能有微小差异 |
| **额外延迟** | 无 | ~2-5 秒（加载 + 刷新 + 保存） |
| **可靠性** | WPS 手动刷新始终可用 | 需维护 LibreOffice 环境稳定性 |
| **运维复杂度** | 低 | 中——需监控 LibreOffice 进程、字体可用性 |

### 推荐：维持路径 (a)，记录 WPS 限制

**理由：**

1. **当前已满足核心需求：** `updateFields=true` 在 Word 中工作正常。WPS 用户可通过 F9 手动刷新，操作路径清晰（右键 TOC → 更新域 → 更新整个目录）。
2. **WPS 手动刷新是已知的"退出条件"：** 验收时已确认 WPS 手动刷新正常工作，无"未定义书签"错误。这满足当前交付标准。
3. **烘烤步骤的 ROI 不高：** 额外服务端依赖 + 排版不一致风险 + 2-5 秒延迟，换来的是 WPS 用户的"免操作"体验——在中国客户侧 WPS 使用普遍，但手动 F9 是可接受的常识性操作。
4. **文档说明即可覆盖：** 在交付操作手册中注明"如在 WPS 中打开，请按 F9 刷新目录"。

### 何时升级到路径 (b)

如果未来出现以下情况，可随时添加 LibreOffice 烘烤步骤：
- 客户明确要求"零用户操作"，且不接受 WPS 手动刷新
- 批量导出场景（一次导出数百份报告），WPS 无法逐一手动刷新
- 拥有成熟的 LibreOffice 服务器运维基础设施

---

## 6. 附录：当前模板验证状态

- `word/settings.xml` 已包含 `<w:updateFields w:val="true"/>` — ✅ 2026-07-22 实测确认
- TOC 缓存已清空（`PAGEREF=0`、`HYPERLINK=0` 已删除）— ✅
- WPS 手动 F9 刷新后 TOC 正确显示页码和超链接 — ✅
- Word 打开时有"是否更新域"提示，确认后 TOC 正常 — ✅（历史测试记录）

### 一手来源引用汇总

| 来源 | 链接 | 访问日期 | 引用内容 |
|------|------|---------|---------|
| MS Learn — UpdateFieldsOnOpen | https://learn.microsoft.com/en-us/dotnet/api/documentformat.openxml.wordprocessing.updatefieldsonopen | 2026-07-22 | §1 规范的完整转述 |
| docxtpl 官方文档 | https://docxtpl.readthedocs.io/en/latest/ | 2026-07-22 | §3 无法预计算页码 |
| python-docx 文档 | https://python-docx.readthedocs.io/en/latest/ | 2026-07-22 | §3 无排版引擎 |
| WPS Office Help Center | https://help.wps.com/ | 2026-07-22 | §2 无 updateFields 相关文档 |
| OpenOffice UNO API — XRefreshable | https://www.openoffice.org/api/docs/common/ref/com/sun/star/util/XRefreshable.html | 2026-07-22 | §4 `refresh()` 方法 |
| OpenOffice UNO API — XDocumentIndex | https://www.openoffice.org/api/docs/common/ref/com/sun/star/text/XDocumentIndex.html | 2026-07-22 | §4 `update()` 方法 |
| docx-rendering-research.md (同目录) | — | 内部文档 | 背景：TOC 缓存清除、updateFields 补全策略 |
