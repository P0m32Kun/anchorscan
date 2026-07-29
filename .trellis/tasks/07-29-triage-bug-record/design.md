# Bug 记录处置设计

## 边界

父任务只维护 `bug记录.md` 的处置结论、子任务树、依赖和最终集成验收；不直接修改产品代码。每个可独立验证的产品行为归属一个子任务，研究任务只能产出证据和后续建议，不能夹带扫描管线修改。

## 任务映射

| 子任务 | 交付类型 | 依赖 | 产物 |
| --- | --- | --- | --- |
| `enhance-report-service-filters` | P1 Web/后端功能 | 无 | 多服务筛选、未识别快捷排除与一致导出 |
| `fix-console-shanghai-time` | Web 展示修复 | 无 | 两处 Console 共用上海时间格式化 |
| `remove-default-nuclei-template-rules` | 后端配置/扫描契约清理 | 无 | 默认规则拒绝 `template:`，只走 tags |
| `research-ssl-tls-coverage` | 安全覆盖研究 | 无 | 覆盖矩阵、版本证据、后续建议 |
| `add-spark-detection-rules` | 后端规则功能 | 移除默认模板能力 | 经验证的 Spark 指纹到 tags 映射 |
| `research-unknown-service-enrichment` | 高风险研究 | 无 | 预算化 MVP 建议或明确不实施 |

## 关键契约

- 默认扫描规则的 `template:` 是废弃且非法字段；读取到它必须失败并提示改用 `nuclei_tags`。单工具 `--template` 不属于该配置契约。
- 报告筛选仅投影视图，不重写 Fingerprint、Finding 或历史 DetectionCheck；同一 `url.Values` 进入页面、HTML export、assets.txt 与命令端点，保证筛选含义一致。
- ScanEvent 在存储/API 中仍为 UTC RFC3339；仅 Vue Console 文本投影转换为 `Asia/Shanghai`。
- 研究结论必须包含工具或模板版本、可复现方式、风险边界和未覆盖条件；无证据不得声称覆盖。

## 依赖与顺序

先规划并实施可独立、低风险的报告筛选、上海时间和模板契约清理；Spark 在模板契约清理合入后再开始。TLS 与未知服务研究可并行，但其输出必须经过独立批准才可形成新的实施任务。

## 风险控制

- 不将 `tcpwrapped` 当作普通 unknown，也不在研究期间增加主动探测。
- 不采用静默兼容旧 `template:` 配置，避免扫描看似成功却降低覆盖。
- 不因页面时区需求改变数据库序列化、事件 ID 或轮询顺序。
