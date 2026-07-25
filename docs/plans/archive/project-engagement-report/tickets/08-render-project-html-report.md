# 08 — 生成项目正式单文件 HTML

**What to build:** 按 DOCX 参考模板的结构生成 Project 级离线 HTML，替换所有 XX、动态构建表 3-1，并按 Zone 输出已验证条目和截图。

**Blocked by:** 05 — 正向工作台；06 — 负向工作台。

**Status:** done

- [x] 元数据、Zone 接入信息和结论槽覆盖全部模板 XX；缺失时阻止导出。
- [x] 表 3-1 只从 included confirmed Verifications 派生标题、唯一 IP:port 和等级。
- [x] 相同漏洞跨 Runs/Zones 聚合为一行，详情仍按 Zone 展示。
- [x] Evidence 以内嵌 data URI 按顺序输出，等比例限制宽度且不分析内容。
- [x] not_observed 单独展示，inconclusive 不进入正式统计。
- [x] 工具与方法从纳入 Runs 的真实配置/执行事实生成，不照抄 DOCX 的旧工具示例。
- [x] HTML 单文件离线可读，并用多 Zone、多 Run、多 Evidence fixture 做浏览器/打印检查。
