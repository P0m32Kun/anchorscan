---
change: modernize-web-console
role: technical-design
spec: docs/plans/modernize-web-console/spec.md
---

# 现代化 Web Console 技术设计

## 当前结构

Web Console 使用以下静态前端结构：

- `internal/web/templates/base.html` 提供所有控制台页面共享外壳。
- `internal/web/templates/*.html` 由 Go `html/template` 与 `base.html` 一起解析。
- `internal/web/static/style.css` 是唯一全局样式入口。
- `app.js`、`report-ui.js`、`run-status.js` 和 `tool-form.js` 提供原生交互。
- `internal/report/templates/report.html` 是独立、自包含的导出报告模板，与 Web Console 模板分离。

该结构已经满足产品规模。问题是样式接口分散、内联展示规则过多，而不是模块数量不足。因此本计划深化现有样式模块，不引入新的前端运行时或构建边界。

## 样式模块与接口

`style.css` 继续作为共享样式模块。它向模板暴露三层小接口：

1. **设计令牌**：颜色、文字、间距、圆角、阴影、层级和动效。
2. **共享原语**：按钮、字段、表格、状态、提示、disclosure、工具栏和分页。
3. **页面语义类**：报告筛选区、项目摘要、运行日志等确实具有独立布局的结构。

模板只需要知道这些语义类，不应重复颜色、边框、间距和字体实现。删除样式模块后，这些规则会重新散落到所有模板，说明该模块提供了真实 locality 和 leverage。

不创建通用卡片工厂、CSS 工具类集合或模板组件库。只有两个以上页面实际共享的视觉行为才进入共享原语；单页布局留在带页面语义的类中。

## CSS 组织

保持单文件，按以下顺序整理：

1. Reset 与设计令牌。
2. 文本、链接和焦点。
3. 桌面应用外壳与导航。
4. 按钮、表单、状态、表格、disclosure、分页等共享原语。
5. 报告页面。
6. 项目与扫描页面。
7. 运行、工具、导入、知识库与配置页面。
8. 打印和 reduced-motion 规则。

删除旧深色令牌、橙色品牌规则、移动端媒体查询和已无调用的选择器。桌面布局必须在浏览器窗口为 1280px 时不产生页面级横向溢出；不得通过 `html`/`body` 的固定最小宽度抵消滚动条占用的可用内容宽度。不新增窄屏适配。

纯展示内联样式迁移到语义类。允许保留的内联值仅包括：

- 数据计算产生的宽度或比例；
- Go 模板条件无法通过类名简单表达的单一动态值；
- 独立报告必须内嵌的自包含 CSS。

## 模板迁移

`base.html` 首先迁移为新的桌面外壳：保留路由入口和当前页状态逻辑，删除移动页眉、遮罩、侧栏 toggle 和对应事件。盾牌 SVG 简化但不改变产品名称。

页面按 ticket 逐步替换展示结构。迁移时必须保持：

- 表单 `method`、`action`、字段 `name` 和当前默认值；
- 链接、查询参数和下载 URL；
- Go 模板字段、条件和循环语义；
- JS 依赖的稳定 ID 与 `data-*` hook，除非在同一 ticket 中同步迁移和测试。

不为了视觉复用增加新的 Go view model，除非模板无法在不复制业务逻辑的前提下表达已批准布局。单纯计数、排序或筛选仍由现有后端负责。

## JavaScript 迁移

现有 JavaScript 文件继续按行为领域划分，不合并成框架式入口：

- `app.js`：共享复制、筛选 popover 和通用交互。
- `report-ui.js`：报告分布和分页行为。
- `run-status.js`：运行状态轮询。
- `tool-form.js`：工具表单行为。

将 `base.html` 中的内联导航脚本移入现有 `app.js`，删除移动侧栏逻辑。新增交互优先使用原生 `details`、`dialog`、sticky positioning 和已有 `data-*` hook。只有原生能力不能满足已批准行为时才增加少量 JS。

CSS 类只负责呈现；JS hook 使用 `data-*` 或稳定 ID。行为变化必须在 `app.test.mjs` 留下一个最小可运行检查。

## 报告页改造

报告页在不改变 handler 数据和 URL 的前提下重新组织模板：

- 页面摘要、风险分布、操作区、筛选工作栏和视图切换形成稳定顺序。
- 筛选表单继续提交现有 `ip`、`q`、`severity`、`port`、`service`、`source` 和 `view` 参数。
- 端口、主机和漏洞聚合视图继续消费当前 view model。
- 表格和分页只改变标记结构与类名，不改变分页大小、链接生成或导出范围。
- 复制和命令生成保留当前 `data-copy-*`、`data-batch-*` 与请求行为。
- sticky 工具栏和表头由 CSS 实现；如需处理层级，仅增加必要的类或 `data-*` 状态。

报告页当前大量内联样式优先清理，但不进行与视觉无关的 handler 重构。

## 独立导出报告

`internal/report/templates/report.html` 继续输出单个 HTML 文件。它不加载 Web Console 的 `/static/style.css`，因为报告必须离线可用。

为避免引入生成器，导出模板内保留一份小型自包含令牌表和报告专用样式。需要人工保持一致的内容限制为颜色、字体、风险标签、表格和打印规则；不复制导航、表单或 Web 交互样式。最终 ticket 通过 Web 报告与导出报告并排截图检查偏差。

## 测试与验收 seam

### 自动检查

- Go handler 测试验证页面仍包含现有字段、表单和链接，并保持所有行为路径。
- `node --test internal/web/static/app.test.mjs` 验证共享与报告交互。
- 独立报告测试验证模板仍可生成并包含全部数据。
- 每个 ticket 运行聚焦测试；最终运行 `go test ./...`、JS 测试和仓库现有静态检查。

不为颜色和 DOM 排列建立大面积字符串快照。测试观察业务接口，不锁死视觉实现。

### 视觉检查

- 使用真实本地数据在 1440px 生成 ticket 指定截图。
- 在 1280px 检查页面级溢出、工具栏、表单和报告表格。
- 检查 hover、focus-visible、disabled、loading、empty、error、popover、展开项和长文本状态。
- 每阶段截图经用户确认后才完成 ticket。

### TDD 范围

只有交互行为或模板公共契约发生变化时使用 `tdd`。纯 CSS 迁移不先写无意义的选择器测试。每个交互 seam 只增加能证明行为的最小测试，不建立视觉快照框架。

## 风险与控制

- **全局 CSS 回归**：共享原语按阶段迁移，每阶段检查所有已迁移页面和一个尚未迁移页面，避免选择器意外外溢。
- **模板行为回归**：保留表单字段、URL、查询参数和 `data-*` hook；相关 handler/JS 测试先行。
- **报告复杂度**：先完成批准的报告原型，再迁移完整视图；不在视觉 ticket 中重写后端报告模型。
- **视觉不一致**：设计令牌批准后冻结，后续变更先更新 `product-design.md`。
- **导出报告漂移**：保持令牌复制范围很小，通过最终并排截图验收，而不是增加代码生成。

## 回滚

各 ticket 按页面范围独立提交。原型 ticket 不修改生产代码；后续 ticket 可按阶段回滚。没有数据库迁移、配置迁移或数据格式变化。
