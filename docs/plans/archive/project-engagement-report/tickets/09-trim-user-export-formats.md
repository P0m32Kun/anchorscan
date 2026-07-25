# 09 — 收敛用户导出为 HTML

**What to build:** 在项目正式 HTML 可用后删除用户可见 JSON/CSV 报告和资产 CSV，只保留 HTML、复制操作与必要 TXT 清单，同时保护内部 JSON 流水线。

**Blocked by:** 08 — 生成项目正式单文件 HTML。

**Status:** done

- [x] 删除报告页 ExportJSON、ExportCSV 和资产 CSV 按钮/视图字段。
- [x] 删除 Web JSON/CSV export handler 分支与不再使用的 CSV helper/tests。
- [x] 保留 Run HTML 与 Project HTML。
- [x] 保留内部 WriteJSON/report.json、SQLite、CLI 所需兼容行为。
- [x] 保留复制 IP/IP:port/URL 与批量命令依赖的 TXT 文件。
- [x] 回归测试证明用户格式收敛没有破坏扫描完成和报告重建。
