# 报告服务筛选设计

## 边界与数据流

`runReportReading` 已是 Console、HTML export、assets.txt 和命令端点共享的过滤边界。扩展 `reportFilters`：保留单一 `service` 精确选择，增加只读的未识别排除语义（建议查询键 `exclude_unidentified=1`）。筛选先作用于 Fingerprint，再以筛后 Fingerprint 关联 Finding 与 DetectionCheck；不写数据库。

## 契约

未识别原始服务集合固定为 `tcpwrapped`、`unknown`、空字符串。服务 facet 由应用了 IP/port/keyword 等其他资产筛选、但未应用 service 与未识别排除的 Fingerprint 计算；以稳定排序输出 `{raw_value, label, count}`，空值显示“未识别（空）”。筛选变更的 URL 必须删除 `assets_page` 与 `findings_page`；分页/导出 URL 保留所有筛选。

## 风险与回滚

不新增 API，不迁移数据。回滚仅移除新的 query/UI，旧 `service` 行为不变。
