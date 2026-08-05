# 01 — 发布 catalog v2 跨仓库协议

**What to build:** 在 Pentest-Playbook 发布可供 AnchorScan 消费的 catalog v2：正式 schema、从 `verify` 派生的 canonical `command`、版本门控与真实产物测试。

**Blocked by:** None — 跨仓库前置条件。

**Status:** draft

**Owner repository:** `Pentest-Playbook`。本 ticket 完成前，AnchorScan 不开始 JSON consumer 实现。

## 行为契约

- `handbook-v2/schema/catalog.schema.json` 定义 catalog v2 顶层和条目消费契约。
- `dist/catalog.json` 使用 `version: 2`、`source: handbook-v2`，且 `entry_count` 与 entries 长度一致。
- 有 `verify` 的条目同时有由 generator 派生的 `command`；无 `verify` 的条目两者都没有。
- `command` 与同一条目生成 Markdown 的命令块逐字节一致。
- `safety`、`status`、`sources`、`generated`、`verify` 和 `command` 都出现在 catalog 投影中；`safety.cleanup` 保留。
- 影响 command、safety、match、status、来源或消费含义的未来变更必须提升 catalog version。

## 测试 seam

- producer schema/生成器 unit；
- `build_handbook.py --check` 对真实全部条目；
- 真实 `-code`、optional、manual-gated + cleanup、needs-review、无 verify 条目契约断言。

## 验收

- [ ] catalog schema 覆盖顶层版本、source、计数及所有 consumer 字段。
- [ ] generator 只从 `verify` 生成 `command`，不允许手工维护 command。
- [ ] catalog command 与 v2 Markdown 命令块逐字节相等。
- [ ] 当前真实 catalog 通过 schema、源级校验和 `build_handbook.py --check`。
- [ ] 发布 artifact 与 schema 一起提交和推送，供 AnchorScan 固定为测试 fixture。

## 非目标

- 不在此 ticket 修改 AnchorScan。
- 不改变 handbook-v2 条目正文的知识结构。
