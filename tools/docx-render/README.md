# DOCX 渲染可行性原型

问题：以用户简化模板的副本验证 docxtpl 是否能保留未填充结构，同时清晰显示所有占位符、生成 0/1/多条表 3-1 数据行，并在第三节按实际网络分区重复 Scan Run、Verification 和顺序 Evidence 图片。

运行：

```sh
uv run --project tools/docx-render python tools/docx-render/prototype.py
```

固定输入是 `fixtures/project_report.json`；不读数据库、不启动 Web。`fixtures/source-template.docx` 是用户源文件的只读副本，脚本只生成以下实验产物：

- `out/project-report-placeholder-template.docx`：供人检查槽位的 docxtpl 模板，Jinja 占位符可见，正文示例截图及其 media part/relationship 已删除；
- `out/project-report.docx`：固定 fixture 的能力证明，Jinja 占位符已被数据和测试图片消费，不能把它当模板；
- `out/table-0.docx`、`out/table-1.docx`、`out/table-3.docx`：表 3-1 的 0/1/多行结构样例。

当前简化源模板的网络分区示例区没有表格，因此原型不会凭空制造分区表格。每个 Scan Run 必须归属 I区、II区、III区或自定义分区；报告只输出实际有纳入内容的分区，缺失分区不生成空标题。

槽位识别规则很简单：实验模板中只有 `{{ ... }}`（值或图片）和 `{% ... %}`（循环/条件控制）是有效占位符。表 3-1 循环内使用短别名 `r.no / r.title / r.assets / r.level`，避免字段名被窄列拆散；它们仍对应 `summary_rows` 的正式字段。

封面三个值槽保留源模板下划线，月份槽显式居中。“漏洞描述 / 修改建议”槽在正式实现中复用现有知识库交付字段；“漏洞详情”标题下只循环 Evidence 图片，不存在 `verification.detail`、“证据截图”标题或 caption。只有 confirmed 漏洞使用三级标题，not_observed 使用未编号普通段落，因此 3.x.y 的数量严格等于该分区的已发现漏洞数。第四节使用用户指定的统计句式与固定整改文案。

`check_structure.py` 以源副本为基线，断言占位符完整可见、封面下划线与月份居中、正文示例图为 0、截图区无多余文本、只有 confirmed 消耗漏洞编号、网络分区循环完整位于第四节之前，并校验缺失 II 区不输出、自定义分区输出、第二章原文、结论句式、节、页眉/页脚、页脚 6 浮动图形、域代码、`updateFields`、表格行数、文本 run 属性和 Evidence 图像顺序/比例。

这是原型，不是 Ticket 11 的正式导出器；它不连接 `ProjectReport` 的 Go 模型，也不替代最终模板的 Word 手工制备与 WPS 验收。
