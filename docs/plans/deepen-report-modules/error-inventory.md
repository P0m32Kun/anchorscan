# Project Report 错误分类

本清单记录 ticket 03 实施后的稳定错误分类。旧文案见下方新旧映射表。

## 稳定错误码

所有 Project Report 模块（`internal/app`）错误实现为 `*app.ProjectReportError`，携带稳定 `Code` 与用户中文 `Message`。Web adapter 按 Code 映射 HTTP 状态，不做字符串匹配。

| 错误码 | HTTP | 用户消息摘要 | 触发条件 |
| --- | --- | --- | --- |
| `PROJECT_NOT_FOUND` | 404 | 项目不存在或已删除，请检查项目列表。 | `GetProject` 返回 `sql.ErrNoRows`。 |
| `PROJECT_REPORT_INVALID` | 400 | 报告数据不完整/不符合交付要求 + 返回项目工作台修正建议。 | 元数据缺失、验证项缺证据/网络分区、`ValidateProjectDeliverable` 失败。 |
| `PROJECT_REPORT_UNAVAILABLE` | 500 | 暂时无法生成报告，请稍后重试。 | Store 读取失败或 Evidence 文件读取失败；内部错误仅写 `log.Printf`，不回传客户端。 |
| `PROJECT_REPORT_DOCX_UNAVAILABLE` | 503 | DOCX 导出不可用 + 检查部署环境建议。 | sidecar/模板未配置、模板文件缺失、sidecar 执行失败。 |

## 内部错误处理

- 数据库读/写错误：`log.Printf` 记录详情后，返回 `PROJECT_REPORT_UNAVAILABLE` 通用消息。
- Evidence 文件读取失败：同上。
- DOCX sidecar stderr：`log.Printf` 记录详情后，返回 `PROJECT_REPORT_DOCX_UNAVAILABLE` 通用消息。
- DOCX JSON 编码 / 临时文件写入错误：`log.Printf` 记录后，返回 500 通用消息。

## 新旧映射

| 旧文案（ticket 01/02） | 新文案（ticket 03） | 新错误码 |
| --- | --- | --- |
| `报告元数据不完整，缺失：<字段>` | `报告元数据不完整，缺失：<字段>。请返回项目工作台补齐后重新导出。` | `PROJECT_REPORT_INVALID` |
| `纳入报告的验证项"<标题>"缺少证据` | `纳入报告的验证项"<标题>"缺少证据，请返回项目工作台补充后重新导出。` | `PROJECT_REPORT_INVALID` |
| `纳入报告的验证项"<标题>"没有有效网络分区` | `纳入报告的验证项"<标题>"没有有效网络分区，请返回项目工作台修正后重新导出。` | `PROJECT_REPORT_INVALID` |
| 其他 `ValidateProjectDeliverable` 错误（如 "正式报告缺少…"） | 原文 + `。请返回项目工作台修正后重新导出。` | `PROJECT_REPORT_INVALID` |
| `err.Error()` 直接返回 500（Store/Evidence 内部错误） | `暂时无法生成报告，请稍后重试。`（仅 `PROJECT_REPORT_UNAVAILABLE`） | `PROJECT_REPORT_UNAVAILABLE` |
| `DOCX 导出未配置：缺少 docxtpl sidecar …` | `[PROJECT_REPORT_DOCX_UNAVAILABLE] DOCX 导出未配置，请先运行 doctor 检查部署环境。` | `PROJECT_REPORT_DOCX_UNAVAILABLE` |
| `DOCX 模板不存在：<路径>` | `[PROJECT_REPORT_DOCX_UNAVAILABLE] DOCX 模板不可用，请检查部署环境。` | `PROJECT_REPORT_DOCX_UNAVAILABLE` |
| `DOCX 渲染失败（docxtpl sidecar …）：<stderr>` | `[PROJECT_REPORT_DOCX_UNAVAILABLE] DOCX 渲染失败，请检查部署环境。`（stderr 仅日志） | `PROJECT_REPORT_DOCX_UNAVAILABLE` |

## 兼容说明

- HTTP 状态码映射不变（404/400/500/503）。
- 400 错误仍可按旧关键字（"缺失"、"缺少"、"测试设备 IP"、"critical"）在 Web 测试中匹配；新增的"项目工作台"后缀不影响现有匹配。
- 503 错误现在以 `[PROJECT_REPORT_DOCX_UNAVAILABLE]` 开头；旧测试中的 `docxtpl` 关键字不再出现。
- 旧 sentinel errors（`ErrProjectReportNotFound`、`ErrInvalidProjectReport`）已删除，由 `*ProjectReportError` 的 `Code` 字段替代。
