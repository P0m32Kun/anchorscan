# 任务书：修复 scroll-spy 导航高亮缺陷（CI quality-gate 失败）

> 本任务书由编排方（Hermes）下达，可直接阅读执行。实施完成后不要执行任何 git 提交/推送操作，由编排方审查后统一处理。

## 背景：CI 连续两次失败（非 flaky）

PR #36（feat/nmap-viewer-feature-consolidation）quality-gate 的 `web-smoke` 连续两次在同一断言失败（原始 run 31106479786 + rerun 均失败）：

```
scripts/web-smoke.mjs:641  AssertionError [ERR_ASSERTION]: timeouts should be active after scroll
```

本地 `make web-smoke` 通过，但这是**差 1px 的侥幸**（见下），CI 渲染布局略异即失败。

## 必读

1. `AGENTS.md`（仓库根，编排约定与硬约束）
2. `internal/web/static/app.js` 第 13–50 行：`initScrollSpy()`——本次要修的函数
3. `internal/web/templates/config.html`：设置页 DOM（`#config-timeouts` 嵌套在 `#config-engine` 内）
4. `internal/web/templates/report.html` 第 382 行附近：`data-scroll-spy` 的第二个使用方（报告浮动大纲），改动必须兼容
5. `scripts/web-smoke.mjs` 第 629–642 行：失败断言上下文
6. `internal/web/server_test.go` 第 188 行：`/static/app.js` 内容断言（含 `copyReportData` 字样，改动不得删除该函数）

## 根因（编排方已实测，探针数据如下）

`initScrollSpy` 两个缺陷叠加：

**缺陷 A：观察带与 scrollIntoView(center) 不匹配。**
IntersectionObserver 的 `rootMargin: '-20% 0px -60% 0px'` 使观察带只覆盖视口高度 20%–40% 的横条。而测试/用户操作 `scrollIntoView({block: 'center'})` 把目标 section 滚到视口中央（50% 处）。视口 960px 时观察带为 y∈[192, 384]，被滚到中央的元素常落在带外，永远不被标记 visible。

**缺陷 B：activate() 多 section 同时可见时选错。**
```js
let current = visiblePairs[0];                    // 选 DOM 顺序第一个
for (let i = 1; i < visiblePairs.length; i++) {
  if (current.section.contains(candidate.section)) current = candidate;  // 只处理"嵌套"一种情况
}
```
当多个 section 与观察带相交时，永远选 DOM 靠前的（config 页即 `#config-appearance`），而不是用户当前正在看的 section。

**实测证据（viewport 1440×960，探针 evaluate 结果）：**

| 场景 | appearance top/bottom | timeouts top/bottom | 观察带 [192,384] 相交 | 结果 |
|---|---|---|---|---|
| 本地 scroll 到 timeouts 后 | -499 / -289 | **383 / 602** | timeouts 恰好相交 1px（383<384） | activate 靠"engine 嵌套包含 timeouts"选中 ✓ |
| CI 布局（appearance 更高或滚动量略异） | 仍在带内 | 落入带外或与 appearance 并存 | appearance 与带相交 | activate 选 appearance ✗ |

本地 timeouts 的 `top=383` 距带下界 `384` 仅 **1px**——任何布局/字体/滚动差异都会翻转结果。CI 上 appearance 与观察带相交（或 timeouts 落带外而 appearance 在带内）即触发断言失败。

## 修复方向（HOW）

重写 `initScrollSpy` 的激活判定，使其基于**几何位置**而非 IntersectionObserver 的可见集合顺序：

- **目标行为**：任一时刻高亮"用户当前正在看的 section"——即其**垂直中心最接近视口中心**的 section（或等价规则：越过观察带上界的最后一个 section）。`scrollIntoView(center)` 滚到的目标必须成为 active。
- 嵌套场景（`#config-timeouts` 在 `#config-engine` 内）：滚动到 timeouts 时高亮 timeouts（更具体），滚动到 engine 其他区域时高亮 engine。用几何位置自然实现，勿依赖 DOM 顺序或 contains 特判。
- IntersectionObserver 可保留用于收集可见性（或直接去掉改用 scroll 事件 + rAF/节流），但**最终激活判定必须基于 rect 几何**，不得再用"第一个可见的"。
- 观察带 rootMargin 可同步调整（例如收窄到视口中心附近），但验收以测试断言为准，不纠结具体数值。
- 兼容 report.html 浮动大纲（同一函数，注意 report 页面大纲可能无嵌套结构）。

## Scope

**要做**：
- 修改 `internal/web/static/app.js` 的 `initScrollSpy`（及相关 helper）
- 如确有必要，微调 `internal/web/static/app.css` 或模板中的 scroll-spy 相关样式/结构，并在报告中说明理由

**不要做**：
- **禁止修改 `scripts/web-smoke.mjs` 的断言**（第 641/642 行行为是合理用户期望：滚到哪高亮哪）
- 禁止改动 config.html / report.html 的 section id 与 nav 链接结构（除非有充分理由并说明）
- 禁止删除 `copyReportData` 等现有函数（server_test.go:188 有内容断言）
- 不实现任何未来功能，不做无关重构
- 不做 git 操作（commit/push/checkout 一律禁止，完成后编排方会核对 git log）

## 铁律

1. 零新依赖，纯原生 JS（ES 语法与现有文件一致），不引入框架
2. 诚实报告：报告必须把「本地实测」与「静态推断」分列；无法实测的项如实标注
3. 已知未跟踪文件（spikes/、docs/plans/fathom-integration/ 等）为既往会话/编排方产物，**不得删除或修改，无需确认**
4. 完成后不得自行 commit；`git status` 应只剩你的改动 + 上述既有未跟踪文件
5. 报告文件：`docs/reports/scrollspy-fix-report.md`，内容含：改动前后 activate 逻辑对比、本地实测命令与输出（含 `npm run test:web` 两条 smoke 全过）、report 大纲验证方式与结果、遗留风险

## 验收

1. `make web-smoke` 本地全过（含 `scripts/web-smoke.mjs` 与 `scripts/ticket-04-web-smoke.mjs` 两条 smoke）
2. `npm run build:web` 通过（vue-tsc --noEmit && vite build）
3. `go test ./internal/web/` 通过（含 server_test.go 的 app.js 内容断言）
4. report.html 浮动大纲场景验证：打开报告页滚动，大纲高亮跟随滚动位置（至少说明验证方式；有脚本/自动化更好）
5. 报告文件存在且含第 4 节要求的各项
6. 说明修复如何消除 1px 脆弱性（为什么新算法对布局差异不敏感）
