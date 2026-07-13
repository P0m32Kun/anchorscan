---
comet_change: modularize-web-presentation
role: technical-design
canonical_spec: openspec
archived-with: 2026-07-13-modularize-web-presentation
status: final
---

# Web 表现层模块化技术设计

## 前置条件与原则

本设计细化 `openspec/changes/modularize-web-presentation/`，并且必须最后实施。开始前要求前两个 change 已完成、相关 UI 目录工作区干净、Go/Node/打包基线通过。

这是结构迁移，不是视觉或交互改版。`/static/app.js`、`/static/style.css`、Web 路由、DOM、模板数据、报告格式和单二进制交付保持稳定。默认不拆；只有能证明独立职责和加载等价时才创建叶子资源。

## 静态报告模板迁移

新增 `internal/report/templates/report.html`，内容从 `html.go` 的 `htmlTemplate` 原始字符串逐字节机械迁移，不格式化、不修正文案、不调整首尾空白。

`internal/report/html.go` 使用标准库：

- 私有 `//go:embed templates/report.html`
- 私有 `embed.FS`
- `template.ParseFS`
- `ExecuteTemplate`

`WriteHTML(path, scanReport)` 的签名、文件创建时机、每次调用时解析模板、错误返回和调用方均保持不变。不增加全局缓存或 `template.Must`，避免改变错误发生时机。

迁移前扩展 `html_test.go`，用固定 `ScanReport` 断言完整输出 SHA-256；迁移后同一断言必须继续通过，并验证二进制无需外部模板文件。

## JavaScript 叶子边界

当前 `app.js` 的明确页面职责只有三组候选：

- 工具表单：`setupToolForm` → `tool-form.js`
- 报告分布：`renderVulnDistribution` → `report-ui.js`
- 运行状态：事件刷新相关代码和 `updateStepper` → `run-status.js`

`app.js` 继续保留共享时间格式、复制等跨页面能力，并作为稳定入口。叶子脚本仍是经典脚本，由对应现有模板在 `app.js` 后按原依赖顺序显式加载。若某组依赖无法在不引入命名空间或重复状态的前提下独立执行，则整组留在 `app.js`。

`app.test.mjs` 保持唯一 Node 测试入口，通过依次读取并在同一 `vm` context 中执行实际加载脚本来镜像浏览器顺序。除非至少两个测试段需要同一设置，否则不新增测试 helper 文件。

## CSS 边界

CSS 不按长度拆分。只有与已经提取的工具、报告或运行页面职责一一对应，且能完整移动连续规则块时，才新增同名叶子 CSS。`style.css` 继续保留全局主题、布局、通用控件和稳定入口。

模板中的 `<link>` 顺序必须维持原级联语义：`style.css` 先加载，页面叶子样式后加载；选择器和声明不修改。若截图或计算样式出现差异，撤销该 CSS 抽取，不用覆盖规则补丁掩盖差异。

## 实施顺序

1. 记录 Go、Node、打包和固定页面基线。
2. 先完成报告模板哈希测试与 `embed.FS` 迁移。
3. 逐个评估并提取 JavaScript 候选，每次同步模板加载与 Node 测试。
4. 仅为已经确认的页面职责移动对应 CSS 规则。
5. 运行全量测试、打包、关键 DOM 和固定视口视觉比较。

每次只移动一个叶子职责。失败时回退该职责，不引入 bundler、模块系统或兼容层。

## 验收标准

- 固定输入的 `WriteHTML` 输出 SHA-256 完全不变。
- 没有运行时模板文件依赖，`make package` 仍生成可用单二进制。
- `/static/app.js` 与 `/static/style.css` 继续可访问。
- 脚本全局可见性、执行顺序、页面交互、DOM 和视觉无预期外变化。
- 不增加第三方依赖、构建步骤、ES module 或前端框架。

