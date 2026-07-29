# 11 — 收敛文档与 Agent 执行契约

**What to build:** 消除版本、部署、ADR 和计划入口漂移，并让本地 Agent tracker 的 skill 入口与降级路径可执行。

**Blocked by:** 10 — 改善 Workbench 失败恢复与职责边界。

**Status:** done

**Execution skills:** `implement`、`code-review`、`ponytail`。

## 行为契约

- README 是唯一 quick start；deploy 只保留部署差异、DOCX、升级、备份和运行限制。
- project-status 表示当前基线；已完成计划位于 archive；删除悬空根 `STATE.md`。
- 修正 Project 不保存默认目标/端口/Profile 的旧文案及固定“双引擎”错误描述。
- ADR 提供权威索引、完整路径链接和 accepted/superseded/rolled-back 状态；新编号不再与 archive 冲突。
- issue tracker 明确 `implement/tdd/code-review` 的入口、输入、产物与 skill 不可用时的降级。
- `.pi/remote-pi` 保持本机状态，不成为产品配置。

## 测试 seam

- Markdown link/path checker。
- 文档文字只做针对已知失效契约的轻量检查，不建立脆弱全文快照。

## 验收

- [x] 删除 `STATE.md`，所有入口指向存在的当前或 archive 文档。
- [x] README/deploy 不重复维护完整 quick start。
- [x] project-status 版本来源不再要求手工同步程序常量。
- [x] ADR 索引能唯一定位当前和历史决策。
- [x] issue tracker 与实际可用 Matt skills 一致。
- [x] 全部 Markdown 相对路径检查通过。

## 验收记录

- `scripts/check_markdown_links.mjs` 检查仓库 Markdown 的本地相对链接，并已接入 `make pr-check`。
- `make pr-check` 和 `make doc-check` 已通过。

## 非目标

- 不重写历史计划正文，不删除有长期决策价值的研究。
