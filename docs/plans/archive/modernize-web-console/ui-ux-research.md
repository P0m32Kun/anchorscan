# Web Console 交互与暗色主题调研

> 调研日期：2026-07-23  
> 范围：桌面优先的安全扫描、任务状态和报告控制台。本文只记录规范依据与设计建议，不包含实现方案。除明确标注为“规范要求”的内容外，其余均为结合 AnchorScan 工作流作出的产品设计推论。

## 调研结论

1. 全局主题应采用 `跟随系统 / 浅色 / 深色` 三态，而不是只有明暗二态。默认跟随系统，显式选择后记住用户偏好；所有控制台页面、表单控件、浮层、代码区和滚动条使用同一主题。CSS 规范把系统偏好与页面声明设计为相互配合的两层机制：[Media Queries Level 5: `prefers-color-scheme`](https://www.w3.org/TR/mediaqueries-5/#prefers-color-scheme)、[CSS Color Adjustment: `color-scheme`](https://www.w3.org/TR/css-color-adjust-1/#color-scheme-prop)。
2. 主要流程必须可以只用键盘完成，并且焦点始终可见、顺序可预测、不被吸顶工具栏遮挡。WCAG 2.2 要求功能可通过键盘操作、焦点可见，且获得焦点的组件不能被作者内容完全遮挡：[2.1.1 Keyboard](https://www.w3.org/WAI/WCAG22/Understanding/keyboard.html)、[2.4.7 Focus Visible](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html)、[2.4.11 Focus Not Obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html)。
3. 扫描表单不应为了“简洁”改成多步向导。高频且决定扫描范围的字段直接展示，低频高级参数使用单层 disclosure；只有当流程存在真实、不可跳过的阶段时才拆成多页。WAI 建议用标签、说明和语义分组降低表单理解成本，多页表单则应标明当前步骤和总体进度：[Form Instructions](https://www.w3.org/WAI/tutorials/forms/instructions/)、[Grouping Controls](https://www.w3.org/WAI/tutorials/forms/grouping/)、[Multi-page Forms](https://www.w3.org/WAI/tutorials/forms/multi-page/)。
4. 扫描是可离开页面的长任务，反馈应是“真实状态 + 当前阶段 + 可计算的数量 + 最新事件”，不能只显示旋转图标，也不能伪造百分比。已知进度使用 determinate，未知进度使用 indeterminate；必要时还需文字说明当前操作：[Microsoft progress controls](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/progress-controls)、[WHATWG `progress` element](https://html.spec.whatwg.org/multipage/form-elements.html#the-progress-element)。
5. 风险等级、Run 状态和检测覆盖不能只靠颜色表达。文字对比度、控件边界和焦点环必须在明暗两套主题中分别验收；普通文本至少 `4.5:1`，大文本至少 `3:1`，必要的非文本视觉信息至少 `3:1`：[1.4.1 Use of Color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html)、[1.4.3 Contrast Minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)、[1.4.11 Non-text Contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html)。

## 1. 全局主题与暗色模式

### 规范依据

| 依据 | 对本项目的含义 |
|---|---|
| 用户代理通过 `prefers-color-scheme` 暴露用户对浅色或深色界面的偏好。[W3C Media Queries Level 5](https://www.w3.org/TR/mediaqueries-5/#prefers-color-scheme) | “跟随系统”应是一个长期存在的选择，而不是首次访问时把系统结果复制成固定的浅色或深色。系统偏好改变时，仍选择“跟随系统”的页面应随之改变。 |
| `color-scheme` 告诉浏览器页面支持哪些主题，并影响浏览器提供的表单控件、滚动条和系统颜色。[W3C CSS Color Adjustment](https://www.w3.org/TR/css-color-adjust-1/#color-scheme-prop) | 只替换页面背景和文字不算完成全局暗色模式；原生输入、选择框、滚动条和浏览器绘制区域也必须一致。 |
| 浅色和深色描述的是一类配色而非固定调色板；规范特别提醒前景色和背景色要成对指定以保证可读性。[W3C CSS Color Adjustment](https://www.w3.org/TR/css-color-adjust-1/#preferred) | 不应对浅色值做机械反相。应以 `canvas / surface / raised / text / muted / separator / accent / focus / risk-*` 等语义角色分别定义两套值。 |
| `localStorage` 为同源文档提供跨浏览会话保存数据的能力。[WHATWG Web Storage](https://html.spec.whatwg.org/multipage/webstorage.html#the-localstorage-attribute) | 主题属于浏览器本地显示偏好，不需要进入项目配置、数据库或 URL；其作用域限于当前浏览器的同源存储。 |
| Dark Mode 是面向低光环境的系统级外观选择。[Apple HIG: Dark Mode](https://developer.apple.com/design/human-interface-guidelines/dark-mode) | 暗色主题的目标是降低环境亮度和长时间工作的疲劳，不是制造“黑客终端”氛围；避免纯黑、霓虹、高饱和发光和大面积风险色。 |

### 建议的主题模型

- 偏好只有三种：`system`、`light`、`dark`。首次访问为 `system`；用户显式选择后跨页面、跨刷新保留。
- 设置页提供唯一的“外观”设置，直接显示三个互斥选项及当前值。主题是低频偏好，不在每个页面标题区重复开关；如后续真实使用反馈显示切换频繁，再增加侧栏快捷入口。
- `system` 是偏好，当前解析出的 `light` 或 `dark` 是结果；界面文案不能把两者混成同一个状态。
- 切换立即生效，不需要“保存”步骤，也不重载页面、不改变当前滚动位置和键盘焦点。
- 全局范围包括应用外壳、页面内容、表格、吸顶栏、popover、dialog、表单控件、代码/日志、空状态、加载和错误状态。任何孤立的白色块都会破坏全局模式。
- 独立导出报告属于可打印交付物，建议暂不继承当前浏览器的本地主题偏好，继续以浅色打印版为权威输出。若以后把独立报告也定义为交互式阅读器，应作为单独范围决定屏幕暗色与打印浅色的关系。

### 暗色视觉约束

- 用表面明度、细边界和有限阴影建立层级；不要靠连续发光描边区分每张卡片。
- 正文、次要文字、分隔线、输入边界、焦点环和语义色都要在暗色背景上重新校验，不能沿用浅色透明度。
- Accent 与 risk colors 是两组独立语义。导航选中、主按钮和焦点继续使用蓝色；critical/high/medium/low/info 只表达风险。
- 风险标签使用“文字 + 色块/图标”，整行不铺风险底色。暗色主题中优先使用低饱和容器色配高对比文字，避免橙红色产生视觉灼光。
- 浏览器强制色彩模式不应被任意覆盖。CSS Color Adjustment 将 forced colors 作为用户代理与作者协商的一部分；除非某个信息确实无法表达，不应强制退出用户颜色方案：[W3C Forced Color Palettes](https://www.w3.org/TR/css-color-adjust-1/#forced-colors-mode)。

## 2. 键盘操作与焦点

### 最小键盘行为矩阵

| 场景 | 预期行为 | 依据 |
|---|---|---|
| 页面导航与普通控件 | `Tab` / `Shift+Tab` 按视觉和 DOM 阅读顺序移动；链接、按钮和原生输入使用平台默认按键，不增加正数 `tabindex`。 | [WCAG 2.4.3 Focus Order](https://www.w3.org/WAI/WCAG22/Understanding/focus-order.html)、[WAI Keyboard Interface Practice](https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/) |
| 高级选项 disclosure | 触发按钮可由 `Enter` 或 `Space` 展开/收起，状态可被辅助技术识别；展开内容进入正常 Tab 顺序。 | [WAI Disclosure Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/)、[WHATWG `details`](https://html.spec.whatwg.org/multipage/interactive-elements.html#the-details-element) |
| 筛选 popover 或 dialog | 打开后焦点进入浮层；`Escape` 关闭；关闭后焦点返回触发按钮。模态 dialog 内焦点不逃逸到背景。 | [WAI Modal Dialog Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/) |
| 动态更新 | 轮询、日志追加、复制成功和状态变化不抢走用户焦点；仅用状态消息向辅助技术通报必要变化。 | [WCAG 4.1.3 Status Messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html) |
| 吸顶工具栏和表头 | 键盘聚焦的控件不能被固定层完全遮住；页面滚动应为固定层留出可见空间。 | [WCAG 2.4.11 Focus Not Obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html) |

### 焦点设计建议

- 所有可操作元素都有清晰的 `focus-visible`。焦点环不能只靠很淡的阴影，在浅色与暗色相邻背景上都应达到必要的非文本对比度。[WCAG 1.4.11](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html)
- 不因鼠标点击隐藏焦点语义；也不让所有静态卡片可聚焦。焦点只进入能执行操作、输入数据或改变展开状态的元素。
- 打开 Run、应用筛选、切换报告视图后，把焦点留在被操作控件或移到新内容的稳定标题；不要每次轮询后跳回页面顶部。
- 错误提交时先呈现错误摘要并把焦点移到摘要或首个无效字段；若错误字段位于收起的高级选项中，先自动展开该组。WAI 要求错误与成功反馈可被识别，且应保留已输入数据：[WAI Form Notifications](https://www.w3.org/WAI/tutorials/forms/notifications/)。
- 桌面控制台的小图标按钮和分页控件至少满足 `24 × 24 CSS px` 的目标尺寸或等效间距，不用极小的点击热区追求密度。[WCAG 2.5.8 Target Size](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)
- 暂不增加单字符全局快捷键。若以后增加 `/` 聚焦搜索等快捷键，必须允许关闭、重新映射，或仅在相关控件获得焦点时生效。[WCAG 2.1.4 Character Key Shortcuts](https://www.w3.org/WAI/WCAG22/Understanding/character-key-shortcuts.html)

## 3. 表单渐进披露与减少操作步骤

### 信息分层

扫描创建页建议保持单页，按以下三层组织：

1. **任务上下文**：Project、Zone、目标。它们决定本次 Run 的归属和范围，始终可见。
2. **常用扫描配置**：Profile、端口或当前工作流中高频修改的参数，始终可见并使用安全默认值。
3. **高级选项**：低频工具参数、并发和原始参数，放进一个单层 disclosure，不再嵌套第二层。

这种分层是项目设计推论；语义实现应继续遵守 WAI 的标签、说明和分组要求：[Labeling Controls](https://www.w3.org/WAI/tutorials/forms/labels/)、[Form Instructions](https://www.w3.org/WAI/tutorials/forms/instructions/)、[Grouping Controls](https://www.w3.org/WAI/tutorials/forms/grouping/)。

### 交互规则

- Disclosure 标题除“高级选项”外，还显示已修改项数量或关键摘要，例如“高级选项 · 已修改 2 项”。收起不会让用户忘记隐藏状态。
- 用户已修改高级字段、字段校验失败或通过深链接恢复表单时，相关分组保持展开。
- 必填、风险较高、会明显改变扫描范围或可能造成误操作的字段不能仅因低频而隐藏。
- 帮助文字紧邻字段，优先解释允许格式、默认值和影响；不要把所有说明塞进 tooltip。触屏、键盘和辅助技术都不应依赖 hover 才能获得关键说明。
- 主操作在页面内只有一个稳定位置，文案描述结果，如“开始扫描”；次要动作视觉降级。提交后保留按钮尺寸，阻止重复提交，并用可读状态说明正在创建 Run。
- 有条件出现的字段应紧邻其控制项出现，不跳到页面远处；隐藏时从 Tab 顺序和辅助技术树中同时移除。
- 不为当前扫描表单增加步骤条或向导。只有当后续流程出现必须依次完成、每阶段内容明显不同且需要中途保存的真实任务时，才采用 WAI 的多页表单模式：[WAI Multi-page Forms](https://www.w3.org/WAI/tutorials/forms/multi-page/)。

## 4. 长任务反馈

### 反馈层级

扫描启动后应立即进入 Run 详情；任务在后台继续，用户可以离开并从扫描历史返回。Run 详情按稳定层级展示：

1. **生命周期状态**：`running / completed / completed_with_errors / failed / canceled / interrupted`，使用完整文字，不能只显示圆点颜色。
2. **当前阶段**：例如端口发现、服务识别、HTTP 探测、漏洞检测、报告整理。
3. **真实进度**：只有分母可靠时才展示百分比或 `已完成目标数 / 总目标数`；不知道剩余工作量时显示 indeterminate，不估算一个看似精确的数字。[Microsoft progress guidance](https://learn.microsoft.com/en-us/windows/apps/develop/ui/controls/progress-controls)
4. **当前发生的事**：最新一条有意义的事件、开始时间和已用时；完整日志放在可展开区域。
5. **下一步动作**：运行中提供取消，结束后根据实际结果提供查看报告、重试或返回历史。

### 动态更新规则

- 状态区域使用非打断式状态消息通报“Run 已创建”“阶段改变”“运行完成/失败”等里程碑，不把每条日志都播报给辅助技术。ARIA `status` 的隐式 live 区域适合非紧急更新：[WAI-ARIA `status`](https://www.w3.org/TR/wai-aria-1.2/#status)、[WCAG 4.1.3](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html)。
- 轮询更新不刷新整页、不重置滚动、不关闭用户展开的区域，也不移动焦点。
- 自动滚动日志提供“暂停自动滚动”；暂停只影响视图跟随，不停止后台任务。持续移动或自动更新且与其他内容并列的信息需要可暂停、停止或隐藏：[WCAG 2.2.2 Pause, Stop, Hide](https://www.w3.org/WAI/WCAG22/Understanding/pause-stop-hide.html)。
- 动画只用于表明 `running`，并服从 `prefers-reduced-motion`；关闭动画后仍通过文字和静态图标表达状态：[MDN `prefers-reduced-motion`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@media/prefers-reduced-motion)。
- `completed_with_errors` 不能伪装成普通成功，`interrupted` 也不能显示为可断点恢复。界面文案必须沿用 `CONTEXT.md` 的领域含义。
- 完成扫描不等于“目标安全”。`completed` 只说明执行结束；Finding 和 Detection Coverage 分别表达发现与实际执行覆盖，不用绿色整页暗示安全保证。

## 5. 表格、状态颜色与报告操作

### 表格结构

- 报告数据继续使用原生数据表格语义，提供描述表格目的的 caption，并用表头单元格及 `scope` 建立行列关系；不要仅靠视觉网格模拟表格。[WAI Tables Tutorial](https://www.w3.org/WAI/tutorials/tables/)
- 排序是表头中的明确按钮，并暴露当前排序方向；筛选不是列名本身的隐藏点击行为。[WAI Sortable Table Example](https://www.w3.org/WAI/ARIA/apg/patterns/table/examples/sortable-table/)
- 主识别字段保持可扫描：IP/主机、端口/服务、Finding 标题优先；来源、协议、时间和 ID 降低视觉权重但保持可读。
- 行级操作使用有名称的按钮或链接，鼠标 hover 只是增强，键盘 focus 时同样可见。不要让整行点击成为唯一入口。
- 固定表头和吸顶筛选栏合计只形成一个清楚的固定层级，不能遮住行内按钮的焦点。[WCAG 2.4.11](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html)

### 状态表达

| 信息 | 推荐表达 | 避免 |
|---|---|---|
| Finding severity | 明确文字标签 + 语义色；必要时增加稳定图标/形状 | 只显示彩色圆点、把整行染红 |
| Run lifecycle | 状态文字 + 图标；运行中可有克制动画 | 用绿色代表 completed 并暗示目标安全 |
| Detection Coverage | 引擎名称、完成/跳过/失败/取消/中断数量 | 用单个百分比冒充漏洞覆盖率 |
| 选择与 hover | 独立的蓝色/中性表面状态 | 复用 severity 颜色表示选中 |
| 错误与警告 | 图标 + 标题 + 可操作说明 | 只有红边或短暂 toast |

上述“文字 + 视觉线索”来自 WCAG 不得只依赖颜色的要求：[WCAG 1.4.1 Use of Color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html)。所有文本、图标、边界、选中态和焦点态还需在浅色、深色、高对比/强制色彩下分别检查，而不是只验证令牌本身：[WCAG 1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)、[WCAG 1.4.11](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html)。

## 建议优先级

### P0：先建立全局一致性

- 把现有“深色模式不在范围内”的已批准决定改为三态全局主题，并建立成对的浅色/深色语义令牌。
- 完成所有主要流程的纯键盘走查、可见焦点和固定层遮挡检查。
- 统一 Run、Finding、Detection Coverage 的“文字 + 视觉线索”状态语言。

### P1：减少高频操作成本

- 把扫描创建页整理为“任务上下文 / 常用配置 / 单层高级选项”，保留单页提交流程。
- 报告页保持搜索常驻、次要筛选收纳、已应用筛选可见且可单独移除。
- 把扫描进度从 spinner 升级为生命周期状态、当前阶段、真实计数、最新事件和下一步动作。

### P2：验证后再增加

- 不预先增加大量全局快捷键、密度切换器、可拖拽列或自定义主题色。这些都会增加学习和维护成本；只有可用性测试证明现有方案仍造成重复操作时再加入。
- 不为高级选项建立多层 accordion，也不把当前扫描表单拆成向导。

## 设计验收清单

- 在 `system / light / dark` 三种偏好下检查所有控制台页面；`system` 能响应系统主题变化，显式选择跨刷新保留。
- 明暗主题分别检查正文、次要文字、风险标签、输入边界、禁用状态、焦点环、图标和图表的对比度。
- 仅使用键盘完成：创建扫描、展开高级选项、修正校验错误、取消 Run、筛选报告、切换报告视图、分页、展开证据和复制命令。
- 打开和关闭 disclosure、popover、dialog 后焦点去向可预测；吸顶栏和固定表头不遮挡焦点。
- 风险等级、Run 状态、错误和检测覆盖在移除颜色后仍可理解。
- 运行中的页面允许离开再返回；轮询不会抢焦点、重置滚动或覆盖用户的展开状态。
- 已知进度才显示数值；未知进度使用 indeterminate 和阶段文字；结束状态给出明确下一步。
- 高级选项收起时仍提示已修改内容；隐藏区域出现错误时自动展开并可直接定位。

## 对现有计划的影响

现有 `spec.md` 的 Approved Product Decisions、Out of Scope 和 Acceptance Criteria，以及 `product-design.md` 的初始浅色令牌，都仍把深色模式排除在当时范围外。该计划已经完成，不回写其历史决定；后续实施由 `docs/plans/evolve-web-console/` 明确取代“无暗色模式、无前端框架”的旧边界，并把三态主题与上述交互验收项拆入新的执行 ticket。
