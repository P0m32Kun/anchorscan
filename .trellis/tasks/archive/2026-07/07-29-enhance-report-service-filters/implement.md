# 执行计划

1. 在 `report_filters` 写 RED 测试：未识别集合、facet 不自过滤、Finding/DetectionCheck 联动。
2. 扩展 query 解析、共享读取模型和纯 view model；实现页面控件与 URL 重置分页。
3. 覆盖 report handler 的页面、HTML export、assets.txt 使用相同筛选。
4. 运行 Go 聚焦测试、前端 typecheck/build、`make pr-check`；复核不写持久化事实。
