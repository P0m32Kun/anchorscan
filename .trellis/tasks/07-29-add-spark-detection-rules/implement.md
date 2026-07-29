# 执行计划

1. 在模板契约清理合入后，收集 Spark Nmap/httpx fixture 与外部模板版本/tag 证据。
2. 写 RED 匹配测试：Spark 命中预期 tags/URL，非 Spark 8080 不命中。
3. 增加最小 tags 规则与执行参数、DetectionCheck 测试；明确排除高风险 tags。
4. 在受控环境验证，或记录模板/环境阻断；更新覆盖文档并运行完整检查。
