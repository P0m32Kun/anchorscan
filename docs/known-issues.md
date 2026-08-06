# 已知问题

> 活文档：解决的问题删除，未解决的留下。其他 agent 可直接读取本文件获取待修问题。
>
> 解决一条后直接从本文件删除对应条目，并在提交消息中引用其 ID。
>
> 新增条目时分配 `ISSUE-NNN` 编号，写清：现象、复现步骤、影响、来源 ticket、建议方向。

---

## ISSUE-001 — sqlite DSN 拼接产生垃圾文件

**现象：** `internal/store/sqlite.go` 用 `path + "?_pragma=..."` 拼接 DSN。某些场景下 modernc sqlite 把 `?_pragma=...` 当作文件名，在工作目录创建垃圾文件（如 `?_pragma=busy_timeout(5000)&_txlock=immediate`）。

**复现步骤：**
1. 运行 `make pr-check`（含 `package-test`）。
2. 检查工作目录是否出现名为 `?_pragma=...` 的文件。

**影响：** 低。不导致功能错误或数据损坏，但在测试和 CI 中产生无意义垃圾文件。

**来源：** nmap-viewer 收敛 Ticket 08（加固）和 Ticket 09（最终 QA）均发现。非收敛计划引入，是既有缺陷。

**建议方向：**
- 检查 DSN 拼接逻辑，确认 modernc sqlite 的 `?_pragma=` 语法是否应改用 `?` query 参数或其他 DSN 格式。
- 受影响文件：`internal/store/sqlite.go`（约第 27 行 DSN 拼接处）。
- 验收：`make pr-check` 后工作目录无垃圾文件。

---
