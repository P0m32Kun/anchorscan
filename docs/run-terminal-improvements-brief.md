# 任务书：任务控制台虚拟终端改进 — 自动滚动恢复 + 日志分级着色

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。

## 背景

任务控制台页面（`/runs/<run_id>`，组件 `internal/web/frontend/RunDetail.vue`）的虚拟终端有两个体验问题：

1. **自动滚动不恢复**：初始 `followingOutput=true`（新日志自动滚到底部），但用户手动滚动离开底部 24px 后 `followingOutput=false`，**永久停住**——即使之后想继续跟踪也必须手动拖回底部。
2. **无分级着色**：`<pre class="event-log">` 用纯文本 join 渲染所有事件，单色（白/灰）。用户要求：**警告黄色、错误红色、命中漏洞或指纹绿色、一般信息白色/灰色**，突出重点。

## 必读

1. `AGENTS.md`（仓库根）
2. `internal/web/frontend/RunDetail.vue`（188 行，全量读）——`followingOutput`/`onOutputScroll`/`output` computed/`<pre ref="eventLog">`
3. `internal/app/scan_target.go`——progress.Emit 调用点（level/stage 语义；fathom findings 持久化 77-82 行目前**无 Emit**）
4. `internal/app/progress.go`——`Progress.Emit(level, stage, format, args)` 接口与事件落库
5. `internal/store/models.go` 56 行——ScanEvent.Level 字段
6. `internal/web/static/style.css` 1267 行附近——`.event-log` 样式

## 需求

### A. 自动滚动恢复（30-45 秒）

- 用户手动滚动离开底部（`followingOutput` 变 false）后，**如果用户在 30-45 秒内没有再次滚动**，自动恢复 `followingOutput=true`（继续跟踪最新输出）。
- 用户持续滚动（每 < 30-45s 滚动一次）则保持手动模式。
- 实现建议：`onOutputScroll` 里用户离开底部时启动/重置一个定时器（`setTimeout` 30-45s）；定时器到点置 `followingOutput=true`；用户滚回底部时清定时器 + 直接恢复。注意：**滚动事件会高频触发**，用 debounce 或每次滚动重置定时器；事件监听器在 `onBeforeUnmount` 清理。
- 选择 30s 或 45s 由你定，写在代码注释里（建议 45s——护网场景用户可能长时间看某一段日志）。

### B. 日志分级着色

`output` 目前是纯文本字符串 join，改为**按行渲染**（`<pre>` 内每行一个 `<span>` 或类似），按规则着色：

| 类别 | 判定规则 | 颜色 |
|---|---|---|
| 错误 | level == "error" | 红 |
| 警告 | level == "warning" | 黄 |
| **命中漏洞** | level=="info" 且消息含 `VULNERABLE`（如 `rdpscan ... VULNERABLE`、`dameng ... VULNERABLE default password`）或 `fathom ... vulnerable` | 绿 |
| **指纹识别** | 消息含 `services=N`（`fathom %s services=%d`）、`service=`、`fingerprint` | 绿（或青，与漏洞区分——建议指纹用青/cyan，漏洞用绿） |
| 普通 | 其他 info / raw | 白/灰 |

- 逐行判断，不要整个输出一个颜色。
- 建议新增一个 `lineClass(event)` 或 computed 返回每行的 class；CSS 定义 `.event-line-error/.event-line-warning/.event-line-hit/.event-line-fingerprint`（颜色用现有 CSS 变量或新增，dark.css 有暗色主题，注意两种主题都好看）。
- `raw` level 行保持现有文案（指向单工具页/报告），颜色灰。

### C. 后端补 fathom 命中事件（否则绿色无从显示）

fathom findings 持久化（`scan_target.go` 77-82 行 `for _, finding := range fathomResult.Findings`）目前**不 Emit progress**——终端看不到 redis-unauth/mysql-weak 等命中。补一条：

```go
progress.Emit("info", "fathom", "fathom %s:%d %s (%s)", finding.IP, finding.Port, strings.ToUpper(finding.ID), finding.Summary)
```

格式参考现有 VULNERABLE 风格（rdpscan/dameng 都是 `info` level + 消息含 VULNERABLE）。这样前端按"消息含 VULNERABLE/命中关键字"着色即可统一。**注意**：finding.ID 如 "redis-unauth" → 建议格式 `fathom 172.22.0.3:6379 REDIS-UNAUTH (fathom redis-unauth: redis_version:7.4.9)` 或类似，确保含可识别的命中标记。

### 不要做

- 不改事件存储结构（scan_events 表、models.go 不动——level 已经是自由字符串，够用）
- 不改其他页面
- 不做 git 操作
- 不引入新依赖

## 铁律

1. 前端改动限 `RunDetail.vue` + `style.css`（+ 必要的 CSS 变量）；后端只加 fathom finding 的 Emit（scan_target.go 一处）
2. 保持现有 web-smoke 断言通过（`make web-smoke`）——**特别是 web-smoke.mjs 641 行附近的 scroll-spy 断言**（之前因 1px 脆弱修过，不要破坏导航高亮逻辑）
3. 诚实报告：色值/恢复时长选择说明；如果运行 `make web-smoke` 刷新了 `docs/reports/ticket-04-playwright/` 截图产物，报告说明（编排方会还原）
4. 已知未跟踪文件（spikes/ 等）不得删除或修改
5. 完成后不得自行 commit
6. 报告文件：`docs/reports/run-terminal-improvements-report.md`

## 验收

1. `npm run build:web`（vue-tsc --noEmit + vite build）通过
2. `go build ./...` 通过（后端 Emit 改动）
3. `go test ./... -count=1` 全过
4. `make web-smoke` 通过
5. 前端行为（如实报告，视觉验收归编排方）：
   - 手动滚动离开底部 → 45s 无操作 → 自动恢复跟随
   - error 行红色、warning 行黄色、VULNERABLE 行绿色、fathom services=N 指纹行青色、普通行白/灰
6. 后端：fathom finding 命中时终端出现 `fathom ... VULNERABLE` 类事件行（lab 扫描可见）
7. 报告 `docs/reports/run-terminal-improvements-report.md`
