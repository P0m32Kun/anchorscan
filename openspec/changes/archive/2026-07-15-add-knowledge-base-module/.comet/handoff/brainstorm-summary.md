# Brainstorm Summary

- Change: add-knowledge-base-module
- Date: 2026-07-15

## 确认的技术方案

- 使用原生 Go 在 `internal/knowledgebase` 中实现具体、只读的 Catalog，不引入接口门面、数据库或新依赖。
- Catalog 在 `web.NewServer` 启动时从 `knowledge_base.path` 加载一次；失败仅改变 Catalog 状态，不阻断 Web、扫描和现有报告。
- 手册 v1 只接受漏洞描述、验证命令、修复建议三个固定章节；知识库模型不包含 Result 字段。
- 标题与 `anchorscan-entry` 元数据之间只允许空白行，不允许其他正文。
- Markdown 解析、YAML 严格解码、命令形状校验、索引、搜索和确定性匹配集中在一个包中；Web 只负责展示。
- 前端配置复用现有保存和备份流程，保存后提示重启生效。

## 关键取舍与风险

- 采用进程启动快照，手册更新需重启；首版不实现热刷新或数据库快照。
- 使用 Go 原生解析器直接消费手册，避免运行时依赖 Python 校验脚本或中间 JSON 文件。
- 局部坏条目被跳过并产生诊断；重复稳定 ID、目录版本错误等全局错误使 Catalog unavailable。
- Catalog 无写入方法，所有查询结果返回副本，避免调用方修改索引数据。

## 测试策略

- 用小型三章节 Markdown fixture 覆盖 ready、disabled、unavailable、degraded 四种状态。
- 分别覆盖严格 YAML、重复 ID、可选命令降级、搜索排序、匹配优先级与歧义。
- 覆盖配置路径保存、相对路径解析、重启提示、知识库列表/详情和服务降级。
- 完整 Go 测试不依赖外部仓库；最终以 Pentest-Playbook 合并提交 `eb4ee8c` 做人工兼容 smoke test。

## Spec Patch

- 必填章节从四个修正为三个：漏洞描述、验证命令、修复建议。
- 删除 Entry.Result 和所有结果说明展示要求。
- 明确标题与元数据之间允许空白行，但禁止其他内容。
