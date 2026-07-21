# DOCX 报告经 docxtpl 模板填充渲染

Status: accepted

Project Report 的 DOCX 交付要求是以用户提供的标准模板为蓝本，保持未参与渲染部分的版式和结构。调研结论（`docs/plans/project-engagement-report/docx-rendering-research.md`，一手来源）是：走**模板填充**路线——一次性制备干净模板，运行时只填充指定文本、复制指定表格数据行、插入/替换指定图片。Pandoc + reference.docx 被其官方手册原文否决（reference-doc 只提供样式表与页面属性，正文 XML 由 Pandoc 从 AST 重建，直接格式、节结构、浮动图形、域代码必丢）。

渲染实现采用 **python-docx + docxtpl** 作为 sidecar 进程：Go 主程序把 `ProjectReport` 序列化为 JSON context，经 `uv run --project tools/docx-render python render_docx.py ...` 启动渲染器。docxtpl 提供表格行 `{%tr for %}` 循环、`InlineImage` 插入/原位替换和段落控制标签；所有 Jinja 占位符必须在模板制备时保证位于同一 run。复杂 Zone 区块是否可由段落标签跨表格循环，必须以原型验证；不通过时使用子文档或调整模板结构。Go 生态无等价物（能力最全的 unioffice 为商业许可，其余 Go 库缺口见调研 §5.3）。

Python 运行时与依赖由 **uv** 管理，隔离在仓库内 `tools/docx-render/` 子项目，不进入 Go 构建：

- `tools/docx-render/pyproject.toml` 声明 `docxtpl`（及传递依赖 `python-docx`、`lxml`）；
- `tools/docx-render/.python-version` 锁定 Python 3.12 以获得可复现环境，`uv.lock` 锁全量依赖；docxtpl 当前也声明支持 Python 3.14；
- doctor 工具检查 `uv` 可用并能 `uv run --project tools/docx-render python -c "import docxtpl"`，缺失或失败时禁用 DOCX 导出并给出提示，不影响 HTML 导出；
- sidecar 调用是短生命 exec。发布包含 docxtpl 的运行环境前，按 LGPL-2.1 的再分发义务补齐许可证、源码获取等合规事项，并由发布方的法务流程确认。

## Consequences

- 仓库新增 `tools/docx-render/`（uv 子项目：`pyproject.toml`、`uv.lock`、`.python-version`、`render_docx.py`、制备好的干净模板、原型/测试 fixture）。用户与 CI 拉/克隆后用 `uv run` 即得到与开发者一致的 Python 版本和依赖。
- 最终用户机器需要 `uv` 与一次 Python 下载（uv 自动拉取锁定的解释器）；doctor 明确提示，DOCX 缺依赖时降级为只导 HTML。
- 模板制备是一次性、受版本管理的人工工序（在 Word 里把 `XX` 逐槽换成 Jinja 占位符、汇总表保留 1 个数据模板行并增加独立的循环控制行、删示例图、补 `<w:updateFields/>`），产物入库；Zone 重复策略以原型结果定稿。逐槽位地图见 `docx-rendering-research.md` §4。
- HTML 与 DOCX 共用同一份 `BuildProjectReport` 输出与完整性校验，不另建聚合模型。
- 降级路径：若部署方否决 Python sidecar，改用 `lukasjarosch/go-docx` + 模板预制上限行数/区块数（保真度打折，缺口清单见调研 §5.3）；不得临时自研 OOXML patcher，也不重启 Pandoc。
- 后续相关决策、模板实测与渲染验证记录在本 ADR 与 `docs/plans/project-engagement-report/` 下文档，跨 agent 协作以仓库文档为准。
