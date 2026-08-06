# 02 — 建立 catalog v2 模型与双格式 loader

**What to build:** 扩展 AnchorScan knowledgebase 模型以保留 v3 的审计与安全语义，实现 catalog v2 JSON loader，并将旧 Markdown 映射为 fail-closed 的 legacy 条目。

**Blocked by:** 01 — 发布 catalog v2 跨仓库协议。

**Status:** done

**Execution skills:** `tdd`、`implement`、`code-review`、`ponytail`。

## 行为契约

- `Entry` 区分 `Catalog.Status` 与条目的 `ReviewStatus`；后者至少包含 stable、needs-review、legacy-unknown。
- `Entry.Safety` 保留 mode、effects、cleanup；`Sources` 与 `Generated` 保留 producer 审计数据。
- JSON loader 只接受 catalog v2，并校验 JSON、source、entry_count、唯一 ID、固定 sections、status、safety、verify 与 command。
- 非法 safety/status/展示字段使条目跳过并 degraded；非法 command/verify 保留展示条目但清空命令并 degraded。
- `.json` 走 JSON；其他后缀仍走 Markdown。相对路径继续以配置文件目录为基准。
- legacy Markdown 条目必须标记 `legacy-unknown`，不得默认 safe；固定章节之间允许扩展知识小节。

## 测试 seam

- `internal/knowledgebase` 单元测试：完整 v2 fixture、文件级 unavailable、所有条目级 diagnostic、JSON/Markdown 分派、重复 ID、零有效条目。
- 静态 producer fixture：内嵌 catalog v2 与同源 v2 Markdown 对等子集，记录 artifact checksum/生成日期，不在运行时读外部仓库。

## 验收

- [x] v2 fixture 在 JSON 路径加载为 ready，legacy 字段和 command 与同源 Markdown 对等子集一致。
- [x] `safety`、`status`、sources、generated、cleanup 不被静默丢弃。
- [x] JSON 条目缺 safety 或非法 safety 不会获得可用命令。
- [x] Markdown 扩展知识小节不破坏固定章节解析。
- [x] legacy Markdown 产生 `legacy-unknown`，而非 safe。
- [x] 聚焦 `go test ./internal/knowledgebase/...` 通过。

## 非目标

- 不在本 ticket 生成/执行命令，也不修改 Web handler。
