## 为什么

Pentest-Playbook 已完成 anchorscan-catalog v1 的基础结构迁移，用于维护漏洞描述、修复建议和通用检测命令；在 AnchorScan 实现前还需要让现有命令通过本文收紧后的工具目标操作数校验。若报告聚合和检测命令分别解析手册，配置、格式校验和匹配规则会重复并逐渐分叉。

新增一个只读知识库 Catalog，让知识库浏览、报告聚合和检测命令共享同一次启动加载及同一套确定性匹配规则。手册继续由独立仓库维护，AnchorScan 不保存第二份内容。

## 变更内容

- 新增具体、只读的 `internal/knowledgebase.Catalog`，服务启动时从配置路径加载一次 anchorscan-catalog v1 手册并构建内存索引。
- 在现有 YAML 配置和前端配置页中增加 `knowledge_base.path`；保存后提示重启生效，不提供热刷新。
- 严格解析目录版本、条目元数据、三个固定正文章节和固定命令段；使用项目已有的 `yaml.v3`，不增加依赖。
- 提供面向人的包含搜索，以及面向扫描结果的确定性匹配；匹配结果显式区分成功、未匹配和歧义，不选择“第一个命中”。
- 新增 `/kb` 列表/搜索页和 `/kb/{id}` 详情页，展示手册内容及保留占位符的通用检测命令。
- 手册未配置、整体不可用或局部异常时可见降级，不阻断扫描和现有技术报告。

## 能力

### 新增能力

- `vulnerability-knowledge-base`：定义本地 v1 手册的配置、启动加载、校验、查询浏览和确定性 finding 匹配行为。

### 修改能力

- 无。报告聚合与检测命令分别由后续 change 接入 Catalog。

## 影响

- 影响配置模型、配置页、Web 服务装配、导航和新增知识库页面。
- 新增 `internal/knowledgebase` 包，但不预设多层门面或单实现接口。
- 不修改数据库和 finding 持久化格式，不远程拉取、不写回、不嵌入完整手册。
- `add-vulnerability-aggregate-report` 和 `add-vulnerability-detection-commands` 必须删除各自的手册解析与匹配设计，统一依赖本能力。
