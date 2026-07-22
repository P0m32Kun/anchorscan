# 05 — confirmed 漏洞自动纳入报告（issue 4）

**What to build:** `confirmed` 与 `not_observed` 验证创建/更新时默认 `Included = true`；`buildProjectDeliverable` 对这两种 outcome 直接纳入，不再受 `Included` 开关拦截；移除验证弹窗的"纳入报告"复选框。

**Blocked by:** None

**Status:** done

## 完成条件

- `web/verifications.go`：`createVerification` / `updateVerification` 在 outcome 为 `confirmed`/`not_observed` 时强制 `Included = true`（忽略请求体里的值）。
- `web/project_report.go:buildProjectDeliverable`：对 `confirmed`/`not_observed` 直接投影，不再 `if !v.Included { continue }`。
- `workbench.html` / `workbench.js`：移除 `verify-included` 复选框及相关字段；`project_detail.html` 扫描运行"纳入报告"开关保留（控制会话段，属另一维度）。
- 保留"无证据不可纳入"的校验：confirmed 仍须至少一张证据（既有 `CountVerificationEvidence` 校验不变）。
- 测试接缝：`verifications_test.go` 默认纳入、`project_report_test.go` 直接纳入两处 red-green。
