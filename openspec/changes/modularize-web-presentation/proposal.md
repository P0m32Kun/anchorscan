## Why

Web 页面模板、`app.js`、`style.css`、Node 测试以及静态 HTML 报告的职责集中在少数大文件中，导致彼此无关的界面维护和报告维护容易互相干扰。当前 UI 工作稳定后再进行一次纯结构整理，可以在不改变任何外部行为的前提下缩小后续修改的影响范围。

## What Changes

- 按“修改原因”而不是文件行数整理现有 Web 模板、JavaScript、CSS 与对应 Node 测试；只有职责和验证边界明确时才拆分文件。
- 保留现有 Web 资源入口、DOM 契约和加载方式，确保页面视觉、交互、路由、表单与模板数据行为不变。
- 将 `internal/report/html.go` 中的大型 HTML 字符串迁移为包内嵌模板，复用项目已有的 `embed.FS` 与 Go 标准库模板模式。
- 为结构迁移补充最小回归验证，覆盖静态资源行为和相同输入下的 HTML 报告输出。
- 不引入 React、Vue、打包器、代码生成器或新依赖；不包含任何视觉重设计。
- 本 change 在当前未提交的 UI 工作稳定后最后实施，避免结构迁移与界面设计同时改动相同文件。

## Capabilities

### New Capabilities

- `web-presentation-modularity`: 约束 Web 表现层资源和静态报告模板按独立修改原因组织，并在重组过程中保持现有外部表现与输出兼容。

### Modified Capabilities

- 无。

## Impact

- 主要影响 `internal/web/templates/`、`internal/web/static/app.js`、`internal/web/static/style.css`、相关 Node 测试，以及 `internal/report/html.go` 与新增的包内模板文件。
- `report.WriteHTML` 的函数签名、调用方、Web 路由、HTTP 行为、JSON 格式、HTML 报告内容和现有资源 URL 均不改变。
- 不改变数据库、扫描流程、构建方式或生产依赖；实施前需要先确认当前 UI 工作已经稳定并具备可比较基线。
