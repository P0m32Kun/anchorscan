# Spec — Web 控制台排版美化与视觉重构

**Status:** completed

本文定义了对 AnchorScan Web 控制台的四个模板（`config.html`、`home.html`、`project_detail.html`、`report.html`）、五个视图区块（全局设置、控制面板、项目扫描历史、网络分区、扫描报告）的视觉美化与排版优化范围。

## 1. 背景与目标

目前 AnchorScan 的 Web 控制台功能完备，但在排版布局上较为拥挤，排版缺乏层次，部分列表和表格内容存在“平铺、堆叠”的混乱感，导致用户难以快速抓取核心指标。
本项目旨在借鉴市面上优秀 SaaS 产品（如 Vercel, Linear, Datadog）与成熟安全平台（如 Snyk, Nessus）的 UI/UX 设计，在不破坏现有 Go-templates 和 Vue 架构、不增加冗余前端依赖的前提下，对样式和排版逻辑进行深度重构，使其呈现为一款**专业、克制、且极具现代化质感**的安全工具控制台。

## 2. 目标页面与优化范围

本次美化重构将涵盖以下五个核心页面：

| 页面 | 原版痛点 | 优化方向 |
|---|---|---|
| **1. 全局设置** (`config.html`) | 设置项多且杂，多项超时输入框垂直堆叠；高级 YAML 编辑器和端口预设区显得杂乱，排版缺乏整体感。 | 引入**左栏局部锚点导航** + **右栏卡片化设置区**；超时配置转换为 Grid 布局；代码/配置编辑区重构为 IDE 风格。 |
| **2. 控制面板** (`home.html`) | 整体界面平淡，缺乏视觉焦点；项目列表和扫描任务列表呈单调的平铺结构，没有数据可视化或状态微动效。 | 引入**英雄概览区光晕背景**；增加**系统就绪状态呼吸灯动效**；重构项目与任务列表为毛玻璃微卡片，增加 hover 悬浮和 meta 信息。 |
| **3. 项目扫描历史** (`project_detail.html`) | 多个 Zone 下的所有任务表格纵向平铺，Zone 多时页面冗长；长 IP 字符可能撑高表格行。 | 引入 **Zone Tabs 局部过滤切换**（定为唯一方案，不采用 Accordion）：选中某个 Zone 时单屏只渲染该分区的表格；目标资产列沿用 CSS 截断 + 原生 `title` 悬浮提示（不新增 Popover 组件）。 |
| **4. 网络分区管理** (`project_detail.html`) | 分区列表与删除动作占位大，新增分区的 Input 与按钮堆叠在一起，缺乏一致性。 | 整合成"分区胶囊流"（Zone Pills）：已有分区渲染为 Pill，删除动作内联在 Pill 内（保留 `删除` 按钮文案与 `.project-zone-item` 结构，冒烟测试依赖），新增分区表单保持 inline 紧凑布局。 |
| **5. 扫描报告** (`report.html`) | 报告非常长，页面滚动时容易迷失。 | 引入**右侧常驻大纲悬浮目录**（sticky 布局列，不使用 fixed 定位，杜绝遮挡）；升级现有漏洞占比堆叠图的视觉质感；证据报文终端框补齐深色模式下的边界层次。**注：现状已有 `report-ui.js` 渲染的堆叠占比图和证据盒一键复制按钮，本次为视觉升级而非从零新增。** |

## 3. 页面改动点与涉及文件

重构工作主要涉及 CSS 重构与 HTML/Vue 的排版层次调整，不涉及任何后端 API 接口逻辑的改变。

### 涉及的源文件

- 核心样式：
  - [style.css](../../../internal/web/static/style.css) (CSS 样式主文件，重构的核心)
  - [dark.css](../../../internal/web/static/dark.css) (深色模式覆盖文件)
  - [app.js](../../../internal/web/static/app.js) (Zone Tabs 过滤与锚点滚动高亮的 vanilla JS，**注意**：`app.test.mjs` 断言全文件只允许一个 `DOMContentLoaded` 回调，新初始化逻辑必须挂在既有回调内)
- Go HTML 模板：
  - [base.html](../../../internal/web/templates/base.html) (应用外壳/侧边栏)
  - [home.html](../../../internal/web/templates/home.html) (仪表盘)
  - [config.html](../../../internal/web/templates/config.html) (全局设置)
  - [project_detail.html](../../../internal/web/templates/project_detail.html) (项目详情/Zone扫描历史/分区管理)
  - [report.html](../../../internal/web/templates/report.html) (扫描报告)
- Vue 前端组件（视排版微调需要）：
  - [ReportInteractions.vue](../../../internal/web/frontend/ReportInteractions.vue) (报告筛选交互，如必要)

## 4. 验收标准与测试规范

1. **三态主题一致性**：在 `system / light / dark` 模式下，重构后的所有页面阴影、描边、背景色和文字对比度皆需通过视觉验收，无局部穿帮。深色模式下黑色终端证据盒必须通过描边或色差与画布分层，不可"隐形"。
2. **纯键盘可达与焦点可见**：新引入的 Zone Tabs、侧边锚点导航、大纲目录均须可以通过 `Tab` 键聚焦，且有高可读性的焦点环（Focus Ring）。
3. **滚动与遮挡防范**：大纲目录与锚点导航一律使用 sticky 布局列（禁用 fixed 覆盖定位），滚动吸顶时不可遮挡任何输入字段、表头或正在操作的行；锚点跳转目标须设置 `scroll-margin-top`。
4. **动效无障碍**：呼吸灯、hover 平移、平滑滚动等所有动效必须尊重 `prefers-reduced-motion: reduce`（沿用 style.css 既有全局护栏，新增动画无需单独处理，但须验证生效）。
5. **新交互行为纳入验证**：Zone Tabs 过滤、锚点滚动高亮（IntersectionObserver）为新增 JS 行为，实施时须在 `scripts/web-smoke.mjs` 中补充对应断言（如：点击 Zone Tab 后其余分区表格隐藏），或列入人工验收清单逐项签字。
6. **不破坏既有契约**：必须保留冒烟测试依赖的选择器与结构——`.project-zone-item`、分区删除按钮（角色名 `删除`）、`select[name="zone_id"]`、`.evidence-copy-btn`、`#distribution-bar` / `#distribution-legend`、各表单字段 name。等宽字体一律使用既有字体栈（JetBrains Mono 仅为栈首候选项，缺失时回退系统等宽字体），**不引入任何外部字体资源或 CDN**。
7. **单元测试与冒烟测试**：重构完成后运行 `npm run test:web` 与 `go test ./internal/...`，确保测试全绿通过。
