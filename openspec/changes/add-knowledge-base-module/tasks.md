## 1. 失败优先测试

- [x] 1.1 用小型合成 fixture 覆盖目录注释、严格元数据 schema、四级风险、漏洞描述/验证命令/修复建议三个固定章节及工具目标操作数契约
- [x] 1.2 覆盖版本声明错误、重复 ID、非法核心条目、非法可选命令及四种 Catalog 状态
- [x] 1.3 覆盖包含搜索、稳定排序、有限 Tool 命名空间、工具 ID/CVE/名称优先级以及歧义不得首选
- [x] 1.4 覆盖配置页保存 `knowledge_base.path`、相对路径解析及“保存后重启生效”提示

## 2. 配置与 Catalog

- [x] 2.1 在现有 Config、默认配置和前端配置页增加 `knowledge_base.path`，复用保存与备份流程
- [x] 2.2 使用已有 `yaml.v3` 实现 v1 严格加载、错误分级和可定位诊断
- [x] 2.3 实现固定 Entry/Commands/MatchKeys 模型及不可变多键索引
- [x] 2.4 实现 `Status`、`Diagnostics`、`Search`、`Entry` 和基于 Observation 的 `Match`

## 3. Web 装配与知识库页面

- [x] 3.1 在 `web.NewServer` 启动时从配置构建一次 Catalog，并保持加载失败不阻断服务
- [x] 3.2 注册 `/kb` 和 `/kb/{id}`，实现列表、包含搜索和详情处理
- [x] 3.3 增加知识库模板与导航入口，复用现有模板转义和复制交互
- [x] 3.4 为 disabled/unavailable/degraded 状态展示对应提示和诊断摘要

## 4. 验证

<!-- external smoke: Pentest-Playbook commit eb4ee8cf21955b038aa2e0aa883ce3eccaec413c; handbook blob 74519602b889bad4a148ed6809481d2dddedaa77; no entry-count threshold -->
<!-- review skipped: current environment has no callable background reviewer dispatcher; go vet ./... and git diff --check passed. -->

- [x] 4.1 运行配置、Catalog、handler、模板及完整 Go 测试
- [x] 4.2 确认未配置手册时所有现有扫描与报告流程不变
- [x] 4.3 在进入检测命令实现前，使用当前 Pentest-Playbook 手册验证全部条目和命令均满足 v1；记录已验证 commit/blob，但不把外部仓库纳入自动化测试依赖
