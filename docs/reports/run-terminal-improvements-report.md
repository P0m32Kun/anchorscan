# Run 终端改进验收报告：自动滚动恢复 + 日志分级着色

- 日期：2026-08-07
- 范围：`docs/run-terminal-improvements-brief.md`
- 风险等级：低-中（前端 `RunDetail.vue` 渲染方式从纯文本改为逐行 span；后端新增一条 progress Emit）

## 改动摘要

| 文件 | 改动 |
|---|---|
| `internal/web/frontend/RunDetail.vue` | 需求 A（45s 自动恢复跟随）+ 需求 B（逐行渲染 + `lineClass` 分级着色） |
| `internal/web/static/style.css` | 新增 `--log-fingerprint` 变量 + 4 个 `.event-line-*` 类 |
| `internal/app/scan_target.go` | 需求 C：fathom findings 循环内新增 1 条 `progress.Emit` |
| `internal/app/scan_target_direct_test.go` | 新增 `TestScanTargetEmitsFathomFindingEvent`（锁定 fathom 命中事件格式） |

未改：`scan_events` 表 / `models.go`（level 自由字符串已够用）、其他页面、`dark.css`（所选色值在两种主题下均由 `:root` 变量继承，无需暗色覆写）。

## 需求 A：自动滚动恢复（45s）

- **时长选择：45s**。护网场景用户可能长时间停留在某一段日志上，30s 易误触发（任务书建议 45s）。
- **机制**（`onOutputScroll` 重写）：
  - 滚动到距底部 24px 内 → 清除恢复定时器并立即恢复 `followingOutput=true`；
  - 离开底部 → `followingOutput=false`，并 `clearTimeout + setTimeout(45s)`；
  - 滚动事件高频触发，每次离开底部的滚动都重置定时器 → 计时从用户**停止滚动那一刻**才开始，持续滚动（每 <45s 一次）保持手动模式；
  - 定时器到点 → `followingOutput=true` 并把日志滚回底部（带文本选中保护，避免打断复制），之后新事件继续自动跟随；
  - `onBeforeUnmount` 同时清理轮询 interval 与恢复定时器。
- 行为验证用 Playwright `page.clock` 确定性快进 45s 模拟（见下），非真实等待 45s。

## 需求 B：日志分级着色（逐行）

`output` computed 拆为：`lineText(event)`（文案，raw 行保持原指向文案）+ `lineClass(level, message)`（判定）+ `lines` computed（每行 `{ text, className }`，行尾自带 `\n`）。模板改为 `<pre>` 内每行一个 `<span :class>`；空状态仍显示「等待运行事件…」。`output` 保留纯文本供「复制输出」按钮使用，文案与改动前逐字节一致。

判定规则（顺序即优先级）：

| 类别 | class | 判定 | 色值 |
|---|---|---|---|
| 错误 | `event-line-error` | `level == 'error'` | `var(--danger)` #ef4444 红 |
| 警告 | `event-line-warning` | `level == 'warning'` | `var(--warning)` #f59e0b 黄 |
| 命中漏洞 | `event-line-hit` | `level == 'info'` 且消息含 `vulnerable`（覆盖 rdpscan/dameng 的 `VULNERABLE`），或匹配 fathom finding 行形如 `fathom <ip>:<port> <大写ID> (` | `var(--success)` #10b981 绿 |
| 指纹识别 | `event-line-fingerprint` | `level == 'info'` 且消息含 `services=\d+` / `service=` / `fingerprint` | `var(--log-fingerprint)` #0891b2 青 |
| 普通 | （无 class） | 其他 info / raw | `var(--text)` 主题主色（浅色深灰 #1d1d1f / 深色白 #f5f5f7） |

色值说明（诚实报告）：
- 红/黄/绿直接复用现有主题变量（`--danger/--warning/--success`），两主题一致，与既有 `.log-*` 配色族统一。
- **指纹青色新增 `--log-fingerprint: #0891b2`**（cyan-600）。选择依据：`#06b6d4`（ANSI 标准青）在浅色 `--code-bg`（#f2f2f7）上对比度仅 ~2:1 不可读；`#0891b2` 浅色 ~3.3:1、深色（#1a1a1e）~4.75:1，与设计内既有彩色文字标准（如 `--danger` 浅色约 3.4:1）一致。未改 `dark.css`——该变量定义在 `:root`，深色主题直接继承。
- **已知副作用（按任务书关键字字面执行）**：`target %s has no open ports; skip fingerprint and vulnerability checks` 一行含 `fingerprint` 关键字，会被着青色（语义上属指纹阶段消息，可接受；如需排除是一行正则的事）。

## 需求 C：fathom 命中事件

```go
progress.Emit("info", "fathom", "fathom %s:%d %s (%s)", finding.IP, finding.Port, strings.ToUpper(finding.ID), finding.Summary)
```

实际落库消息示例：`fathom 127.0.0.1:6379 REDIS-UNAUTH (fathom redis-unauth: redis_version:7.4.9)` —— 与 rdpscan/dameng 相同的 `info` level + 大写命中标记风格；`strings` 包原有 import，未新增依赖。Emit 放在 `persistFinding` 成功之后（与 rdpscan/dameng 命中行位置一致，避免持久化失败时出现"命中但未落库"的误导事件）。

## 验收执行记录

| 验收项 | 命令 | 结果 |
|---|---|---|
| 1. 前端构建 | `npm run build:web`（vue-tsc --noEmit + vite build） | 通过（33 modules，465ms） |
| 2. 后端构建 | `go build ./...` | 通过（无输出） |
| 3. 全量 Go 测试 | `go test ./... -count=1` | 全过（17 包 ok，含新增测试） |
| 4. web-smoke | `make web-smoke` | 通过（`Web browser smoke test passed.`，含 web-smoke.mjs 与 ticket-04-web-smoke.mjs） |

**关于 web-smoke 刷新产物（需编排方还原）**：运行 `make web-smoke` 后，`docs/reports/ticket-04-playwright/` 下 8 张 PNG（manual/needs-review/optional/safe-command 各 before/after）+ `server.log` + `trace.zip` 被 ticket-04 脚本重新生成并标记为 modified（`console.log`、`result.txt` 内容未变）。均为本次测试运行产物，与代码改动无关，请编排方还原。`git status` 中另有预存在的未跟踪项 `config/default.yaml.bak.20260807-122531`（12:25 生成，早于本次会话）与 `spikes/`，均未触碰。

## 前端行为实测（headless Playwright，走真实 dist/anchorscan 二进制 + 种子数据）

8 项断言全部 PASS：

1. 浅色主题逐行着色：7 行事件 → class 序列 `['', fingerprint, hit, hit, warning, error, '']`；普通/raw = `rgb(29,29,31)`（--text），指纹 = `rgb(8,145,178)`，fathom 命中与 rdpscan `VULNERABLE` = `rgb(16,185,129)`，警告 = `rgb(245,158,11)`，错误 = `rgb(239,68,68)`；raw 行保留「原始终端输出…」文案。
2. 深色主题（`emulateMedia dark`）：同上，普通/raw = `rgb(245,245,247)`，其余色值一致。
3. 初始跟随：80 条事件载入后 `scrollTop == scrollHeight-clientHeight`（1207=1207）。
4. 手动滚离底部 → 停住（scrollTop=0 保持）。
5. 手动模式下新到 5 条事件（count 80→85）**不**自动滚动（scrollTop 仍 0）。
6. `page.clock.fastForward(45_000)` → 自动恢复并滚到底部（1305=1305）。
7. 恢复后再来 1 条事件 → 继续跟随（count 86，滚到底部）。
8. 滚回底部 → 立即恢复（清定时器），下一条事件即跟随（count 87）。

需求 C 验证：Go 单测 `TestScanTargetEmitsFathomFindingEvent` 断言 vulnerable check 产生含 `REDIS-UNAUTH` 与 `fathom redis-unauth` 的 info/fathom 事件；前端实测中同格式行（`fathom 172.22.0.3:6379 REDIS-UNAUTH (...)`）渲染为绿色命中行。未运行真实 lab 扫描（无真实工具链），事件格式与扫描管线同一条 Emit 路径，行为一致。

## 诚实报告 / 剩余项

- **色值**：红/黄/绿复用现有主题变量；指纹青新增 `#0891b2`（浅色 3.3:1、深色 4.75:1），理由见上；普通行保持 `var(--text)`（浅色深灰、深色白），未引入额外灰色。
- **恢复时长**：45s，理由见需求 A；45s 计时通过 Playwright clock 快进确定性验证（等价于 45s 真实等待的定时器语义）；「持续滚动重置计时」由实现保证（每次离开底部滚动即重置），未做 45s 真实等待。
- **web-smoke 截图产物**已被测试运行刷新，需编排方还原（见上）。
- **`gofmt`** 对 `scan_target_direct_test.go` 做了整文件对齐（含一处既有 struct 字段对齐修正，纯空白）。
- 未执行 git commit / push（按任务书）。
- 视觉最终验收归编排方；IPv6 目标的 fathom 命中行正则（`^fathom \S+:\d+`）理论上可匹配，但未在真实 IPv6 扫描中实测。
