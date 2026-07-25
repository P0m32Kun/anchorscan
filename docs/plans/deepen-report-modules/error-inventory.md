# Project Report 当前中文错误清单

本清单记录架构重构前的用户可见错误契约。ticket 01 必须保持文本与触发语义；ticket 03 再决定统一措辞、错误码或本地化方式。

## Project Report 组装与验证（400）

| 当前文本 | 触发条件 |
| --- | --- |
| `报告元数据不完整，缺失：<字段列表>` | Project 缺少被测单位、测试对象、测试开始日期、测试结束日期或测试人员；字段以 `、` 连接。 |
| `纳入报告的验证项“<标题>”缺少证据` | included Verification 没有 Evidence。 |
| `纳入报告的验证项“<标题>”没有有效网络分区` | Verification 的 Zone 无效，且无法从 included Run 或唯一 Zone 推断。 |
| `正式报告缺少<字段>` | `ProjectDeliverable` 缺少报告标题、被测单位、测试对象、测试开始日期、测试结束日期或测试人员。 |
| `正式报告缺少项目创建时间` | Project 创建时间为空。 |
| `正式报告暂不支持 critical 结论口径，请先调整严重级别` | 汇总或 included confirmed Verification 含 critical。 |
| `纳入报告的运行“<标签>”缺少测试设备接入点` | included Run 缺少 AccessPoint。 |
| `纳入报告的运行“<标签>”缺少测试设备 IP` | included Run 缺少 TesterIP。 |
| `纳入报告的运行“<标签>”缺少测试范围` | included Run 缺少 Targets。 |
| `纳入报告的已确认验证项“<标题>”缺少漏洞描述` | confirmed Verification 缺少 description。 |
| `纳入报告的已确认验证项“<标题>”缺少修改建议` | confirmed Verification 缺少 remediation。 |
| `纳入报告的已确认验证项“<标题>”缺少关联资产` | confirmed Verification 没有资产。 |
| `纳入报告的已确认验证项“<标题>”缺少证据` | confirmed Verification 在最终交付验证时没有 Evidence。 |
| `纳入报告的未发现验证项“<标题>”缺少端口资产` | not_observed Verification 没有资产。 |
| `纳入报告的未发现验证项“<标题>”缺少证据` | not_observed Verification 在最终交付验证时没有 Evidence。 |

## DOCX adapter（503）

| 当前文本 | 触发条件 |
| --- | --- |
| `DOCX 导出未配置：缺少 docxtpl sidecar 或模板路径，请先运行 doctor 检查。` | 未配置 sidecar project 或模板路径。 |
| `DOCX 模板不存在：<路径>` | 配置的模板文件不存在。 |
| `DOCX 渲染失败（docxtpl sidecar 未安装或出错）：<stderr>` | `uv run ... render_docx.py` 执行失败。 |

## 暂不直接展示的内部错误

Store 读取、Evidence 文件读取、JSON 编码、临时目录/文件写入及 HTML 渲染错误当前直接以 `err.Error()` 返回 500。ticket 03 应单独决定这些内部错误是否需要稳定的用户文案，以及详细原因应记录到日志还是返回给操作者。

## 后续改造约束

- 先定义稳定错误分类或错误码，再统一文案；不要继续以字符串匹配决定 HTTP 状态。
- 不得把 Evidence、Verification 或 Project Report 的领域含义压缩成泛化的“数据错误”。
- 内部路径、sidecar stderr 与数据库错误是否对用户展示，需要单独评估信息泄漏风险。
- 文案变更必须同步 Web 回归测试与操作文档。
