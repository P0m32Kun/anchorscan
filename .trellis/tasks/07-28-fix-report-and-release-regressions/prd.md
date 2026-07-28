# 修复验证分区、DOCX 多网段与发布版本同步

## Goal

修复本轮扫描暴露的三个发布级回归，使项目验证可以保存证据、同一网络分区的多个扫描范围能在 DOCX 中无歧义地交付，并使 Web/CLI 版本号由发布 tag 自动注入。

## Background

- 正向验证创建失败：`internal/web/frontend/Workbench.vue` 的新建请求直接序列化 `ZoneID` 等 PascalCase 字段，而 `internal/web/verifications.go` 的请求契约使用 `zone_id` 等 snake_case JSON 字段。后端收到空 `zone_id`，在项目分区校验处返回 `zone_id is not part of this project`；截图上传尚未开始。
- 漏洞候选模型已按网络分区和漏洞键聚合：同一区多个 run 发现的同一漏洞会合并资产，不需要再按扫描任务拆分。DOCX 仍以 `network_zone.sessions` 重复输出每个 run 的接入信息，fixture 与测试也仅覆盖每区一个 session；这会让扫描任务结构泄漏到正式报告，并且缺少清晰的分区级多值表达。
- Git 最新 tag 为 `v2.0.1`，`internal/version.Version` 仍硬编码为 `1.9.2`。发布 workflow 只把 tag 传给归档文件名，没有注入 Go 二进制，因此 Web 左下角和 CLI 都显示旧版本。
- CVE-2023-45648 手册条目明确不在本任务范围内，由用户在独立手册项目处理。

## Requirements

- R1：正向候选创建 verification 时必须使用后端公开的 snake_case JSON 契约，并保留正确的项目、分区、资产和来源信息。
- R2：同一项目、同一网络分区内来自多个扫描 run 的候选必须能够创建 verification，随后上传 PNG/JPEG 证据，不再出现错误的分区归属拒绝。
- R3：DOCX context、fixture、正式模板和结构检查必须共同支持同一区多个 included runs，但正式报告仍只生成一个网络分区章节，不按扫描任务创建子分区或子章节。
- R4：每个网络分区在 DOCX 中以分区级多值字段记录其全部接入点、测试设备 IP 和目标网段；值需稳定排序或按 run 的稳定顺序输出，并避免相同值重复出现。
- R5：同一区多个 run 发现的相同漏洞必须在验证台中维持一个候选，资产列表合并；不同网络分区的相同漏洞仍是不同候选。
- R6：发布构建必须从 `v*` tag 派生无前缀的版本字符串并注入 `internal/version.Version`；CLI `anchorscan version` 与 Web footer 必须显示同一值。
- R7：发布 workflow/构建检查必须在版本未正确注入时失败，避免以后新 tag 再次发布旧版本号。
- R8：保留现有非发布开发构建的可读版本行为，不引入运行时网络查询或读取 Git 仓库的依赖。

## Acceptance Criteria

- [ ] AC1：覆盖同一区至少两个 included runs 的回归测试能够创建正向 verification，并成功上传一张截图；保存的 `ZoneID` 等于该项目中的分区 ID。
- [ ] AC2：前端新建与更新 verification 使用一致的 snake_case payload；后端仍拒绝真正不属于项目的分区。
- [ ] AC3：验证台回归测试覆盖同一区两个 included runs、相同漏洞键和不同资产，结果只有一个候选，且候选包含两个资产和两个来源 run。
- [ ] AC4：DOCX Go context 测试覆盖同一区两个 included runs，输出一个网络分区对象；该对象的接入点、测试设备 IP 和目标网段字段均包含两个 run 的值且不重复。
- [ ] AC5：DOCX sidecar/模板测试渲染上述分区，文档只出现一个“I区”章节，并在该章节内完整显示多个接入点、多个测试设备 IP 和多个目标网段；不生成扫描任务子章节。
- [ ] AC6：最终 DOCX fixture 经 PNG 全页渲染检查，无新增裁切、重叠、破表或异常分页。
- [ ] AC7：以测试版本（例如 `v9.8.7`）执行发布构建时，产物 CLI 输出 `9.8.7`，Web footer 使用同一版本；tag 前缀 `v` 不出现在 footer 的重复 `vv...` 中。
- [ ] AC8：现有 Go、前端、DOCX 测试与构建检查通过。

## Out of Scope

- CVE-2023-45648 或其他漏洞手册内容。
- 修改网络分区的数据模型、扫描 run 的归属规则或历史数据迁移。
- 改版 DOCX 的整体视觉设计、章节结构或用户源模板中未参与渲染的内容。
- 自动创建、推送 tag 或发布 GitHub Release。
