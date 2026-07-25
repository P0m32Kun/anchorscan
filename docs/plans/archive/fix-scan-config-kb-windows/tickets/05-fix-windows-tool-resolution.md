# 05 — 修复 Windows 工具解析

**What to build:** 使用标准库可执行文件解析语义支持 PATH 工具名和 Windows 可执行文件，同时保持缺失工具报错。

**Blocked by:** 04-trim-knowledgebase-separator

**Status:** done

- [x] PATH 中的工具名通过预检。
- [x] 缺失工具和目录仍不能通过预检。
- [x] runner 继续直接传递 binary 与 args，不经 shell 拼接。
- [x] 聚焦预检与工具测试通过。
