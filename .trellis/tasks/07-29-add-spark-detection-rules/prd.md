# 补齐 Spark 服务检测规则

## Goal

让已被识别为 Spark Web UI/API 的服务进入默认 Nuclei tags 调度，并以安全、可复现的方式记录检测覆盖。

## Requirements

- 先确认 Nmap/httpx 可能产生的 Spark 服务名、产品名、技术标识和 URL 形态。
- 使用官方/外部模板库的 tags 调度，不使用仓库内置模板或默认规则 `template:`。
- 只覆盖 Spark Web UI/API 暴露面，不把所有 8080 端口视为 Spark。
- 明确是否包含默认登录、未授权访问或其他主动检查；遵守现有安全限制。
- 无匹配服务继续记录 `no_matching_rule`，匹配服务按现有 DetectionCheck 契约记录状态。

## Dependencies

- 先完成 `07-29-remove-default-nuclei-template-rules`：默认扫描必须已拒绝 `template:`，Spark 规则才能只依赖 tags 契约实现。
- 不依赖 SSL/TLS 覆盖调查；其结论不得阻塞 Spark Web UI/API 的独立、低风险 tags 路由。

## Non-goals

- 不实现通用未知服务识别。
- 不内置 Spark Nuclei 模板。
- 不声称覆盖所有 Spark 组件或部署模式。

## Acceptance Criteria

- [ ] 受支持的 Spark 指纹可稳定匹配到预期 Nuclei tags 和目标地址。
- [ ] 非 Spark 的 8080/未知服务不会误触发 Spark 规则。
- [ ] 配置加载、匹配、执行参数和 DetectionCheck 结果有自动化测试。
- [ ] 使用项目支持版本的外部模板库完成可复现验证，或明确记录环境阻断。
- [ ] 文档说明覆盖范围、模板来源和剩余盲区。
