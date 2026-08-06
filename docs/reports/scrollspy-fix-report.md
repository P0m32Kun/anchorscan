# scroll-spy 导航高亮修复报告

> 任务书：`docs/scrollspy-ci-fix-brief.md`
> PR #36 `web-smoke` quality-gate 连续失败（`scripts/web-smoke.mjs:641`），修复 `internal/web/static/app.js` 的 `initScrollSpy`。

## 1. 改动前后 activate 逻辑对比

### 改动前（缺陷叠加）

```js
const visible = new Set();
const activate = () => {
  let current = pairs[0];
  const visiblePairs = pairs.filter(p => visible.has(p.section.id));
  if (visiblePairs.length > 0) {
    current = visiblePairs[0];                 // 缺陷B：永远选 DOM/nav 顺序第一个
    for (let i = 1; i < visiblePairs.length; i++) {
      const candidate = visiblePairs[i];
      if (current.section.contains(candidate.section)) current = candidate;  // 仅处理"嵌套"一种情况
    }
  } else {
    for (const pair of pairs) {
      if (pair.section.getBoundingClientRect().top < window.innerHeight * 0.5) current = pair;
    }
  }
  pairs.forEach(pair => pair.link.classList.toggle('active', pair === current));
};
const observer = new IntersectionObserver(entries => { /* 维护 visible 集合 */ activate(); },
  { rootMargin: '-20% 0px -60% 0px' });        // 缺陷A：观察带仅覆盖视口 20%–40%
```

- **缺陷 A**：观察带 `rootMargin: '-20% 0px -60% 0px'` 在 960px 视口下只覆盖 y∈[192,384] 一条窄带；而测试/用户的 `scrollIntoView({block:'center'})` 把目标滚到视口中央（50%=480），目标常落在带外不被标记 visible。
- **缺陷 B**：`visiblePairs[0]` 永远取 nav/DOM 顺序第一个相交者；多个 section 同时相交时只靠 `contains` 特判处理嵌套，其余情况选错。

两者都是**离散/二值判定**（在带内/带外、是第一个/不是），1px 布局差异即可翻转结果。

### 改动后（两段式纯几何，去掉 IntersectionObserver）

```js
const activate = () => {
  const viewportHeight = window.innerHeight;
  const viewportCenter = viewportHeight / 2;
  // 1) 优先：完整位于视口内的 section 中，选最靠上的（top 最小）。
  let best = null, bestTop = Infinity;
  for (const pair of pairs) {
    const rect = pair.section.getBoundingClientRect();
    if (rect.top >= 0 && rect.bottom <= viewportHeight && rect.top < bestTop) {
      best = pair; bestTop = rect.top;
    }
  }
  // 2) 兜底：所有相交 section 都跨越视口边界时，选中心最接近视口中心的
  //    （并列容差 1px 时取更矮/更具体者）。
  if (!best) {
    let bestDist = Infinity, bestHeight = Infinity;
    for (const pair of pairs) {
      const rect = pair.section.getBoundingClientRect();
      const center = (rect.top + rect.bottom) / 2;
      const dist = Math.abs(center - viewportCenter);
      const height = rect.height || 1;
      const clearlyCloser = dist < bestDist - 1;
      const tied = Math.abs(dist - bestDist) <= 1;
      const moreSpecific = tied && height < bestHeight;
      if (!best || clearlyCloser || moreSpecific) { best = pair; bestDist = dist; bestHeight = height; }
    }
  }
  pairs.forEach(pair => pair.link.classList.toggle('active', pair === best));
};
// 触发：scroll / resize 事件 + requestAnimationFrame 节流（替代 IntersectionObserver）
```

判定完全基于 `getBoundingClientRect()` 的连续几何量（top / bottom / center / height），不依赖 IntersectionObserver 的可见集合顺序，不依赖 DOM `contains`。

## 2. 本地实测

### 2.1 验收命令与输出（本地实测）

| 命令 | 结果 |
|---|---|
| `make web-smoke`（= `npm run build:web` + `go build` + `npm run test:web`） | **exit 0**，末行 `Web browser smoke test passed.`；`test:web` = `web-smoke.mjs && ticket-04-web-smoke.mjs` 两条 smoke 由 `&&` 短路保证均通过 |
| `npm run build:web`（`vue-tsc --noEmit && vite build`） | 通过，`✓ 33 modules transformed` / `built in 4xx ms` |
| `go test ./internal/web/` | `ok github.com/P0m32Kun/anchorscan/internal/web 2.745s`（含 `server_test.go` 对 `/static/app.js` 的 `copyReportData` 内容断言） |
| LSP diagnostics（`internal/web/static/app.js`） | `no diagnostics` |

> 失败复现：改动前首次实现仅用"中心最近"单规则，`make web-smoke` 在 `scripts/web-smoke.mjs:404`（report 浮动大纲初始断言）超时失败——证明报告页初始态需要高亮首项，促成本报告第 3 节的两段式方案。

### 2.2 几何探针数据（本地实测，viewport 1440×960，vc=480）

临时探针（已删除）启动真实 binary，按 web-smoke 流程构造报告/配置页，对每个 nav 目标 `scrollIntoView({block:'center'})` 后读取所有 section 的 rect 与实际 active。

**config.html（`#config-timeouts` 嵌套在 `#config-engine` 内）**

| 操作 | scrollY | 候选几何（top–bottom, h, 完整?） | 实际 active | 测试期望 |
|---|---|---|---|---|
| 初始 / scroll appearance | 0 | appearance 158–368(完整) / engine 368–2014 / timeouts 1040–1259 / … | **appearance** | appearance ✓ |
| scroll engine | 653 | engine −285–1361(跨越) / **timeouts 387–606(完整)** | **timeouts** | （未测，见风险） |
| scroll timeouts | 657 | engine −289–1357(跨越) / **timeouts 383–602(完整, 唯一完整)** | **timeouts** | timeouts ✓ |
| scroll yaml | 1625 | engine −1257–389(跨越) / yaml 413–1002(跨越) | **yaml**(中心 dist227 最近) | yaml ✓ |
| scroll ports | 2369 | yaml −331–258(跨越) / **ports 282–951(完整)** | **ports** | ports ✓ |

> CI 失败点（scroll timeouts）实测：timeouts 是视口内唯一完整 section → 被选中，不再依赖"engine 嵌套 contains timeouts"或观察带 1px 相交。

**report.html（浮动大纲，平铺 section）**

| 操作 | scrollY | 实际 active | 说明 |
|---|---|---|---|
| goto 报告页（初始，web-smoke:404 断言） | 0 | **report-risk** | risk(155–279) 与 coverage(295–464) 都完整在视口，选最靠上 risk ✓ |
| scrollTo report-assets 中心 | 3537 | **report-assets** | assets 跨越视口，中心最近 ✓ |
| scrollTo report-findings 中心 | 5128 | **report-findings** | findings 完整在视口(604–949) ✓ |
| scrollTo report-coverage 中心 | 0（clamp） | report-risk | 见风险 R2 |

## 3. report 浮动大纲验证方式与结果

**方式**：临时 Playwright 探针（已删除，逻辑见 2.2）复用 web-smoke 的报告构造流程（导入 Nmap XML + seed fingerprints），打开 `/reports/<runID>?view=hosts`，对大纲每个锚点 `scrollTo` 到其垂直中心，读取 `.report-outline a.active`。

**结果**：
- 初始 / risk / assets / findings 四处大纲高亮**正确跟随**滚动位置。
- `report-coverage` 为边界情况（见风险 R2），非缺陷。
- `web-smoke.mjs:404–407` 的报告页初始断言（首个大纲链接 active）在修复后通过。

> 说明：探针用 `window.scrollTo` 而非 `scrollIntoView` 触发报告页滚动——因为报告主体挂载了 Vue 应用 `[data-report-interactions]`，`scrollIntoView` 在大表格 section 上会触发异步重排导致滚动量不稳定；`scrollTo` 精确可控，且本修复的 scroll-spy 监听的是 `window` `scroll` 事件，对任意滚动来源（滚轮/scrollTo/scrollIntoView/锚点跳转）一视同仁地响应。

## 4. 如何消除 1px 脆弱性（为什么新算法对布局差异不敏感）

旧算法的两个判定都是**离散二值**：
- 观察带相交（在 [192,384] 带内 / 带外）——timeouts `top=383` 距带下界 `384` 仅 1px；
- `visiblePairs[0]`（是 nav 第一个相交 / 不是）。

布局差 1px 就能把目标踢出观察带、或改变谁是"第一个相交"，从而翻转 active。

新算法用**连续几何量** + 两段优先级，每一段都对 1px 级差异不敏感：

1. **完整在视口优先**（`rect.top >= 0 && rect.bottom <= viewportHeight`）：config `timeouts` 完整区间 383–602，下界距视口顶 383px、上界距视口底 358px——布局差几十 px 仍完整在视口，依旧被选中。report 初始 `risk.top=155` 与 `coverage.top=295` 相差 140px，"选最靠上"不会因 1px 翻转。
2. **兜底中心最近**（`|center − viewportCenter|`）：`scrollIntoView(center)` 让目标 `center ≈ viewportCenter`（dist≈0），而其它 section 的 dist 通常 ≥ 数十 px（实测 timeouts dist=12，engine dist=54，差距 42px）。连续距离比较下，1px 布局漂移不会改变谁的距离最小。
3. **并列容差 1px + 取更矮**：仅在两个 section 中心几乎重合时触发，用"高度更小=更具体"做纯几何 tie-break，覆盖 `#config-timeouts` 嵌套场景，不依赖 `contains`。

即：旧算法在"边界附近"用二值判定（脆弱），新算法在"边界附近"用连续量的比较 + 大余量（鲁棒）。

## 5. 兼容性

- **config.html**：nav 结构、section id、`#config-timeouts` 嵌套于 `#config-engine` 均未改动；仅替换激活算法。
- **report.html**：`data-scroll-spy` 浮动大纲共用同一 `initScrollSpy`，平铺 section 同样适用；section id / nav 链接未改动。
- **`copyReportData` 等函数**：未触碰（`server_test.go:188` 内容断言通过）。
- **`scripts/web-smoke.mjs` 断言（641/642 行）**：未改动。
- **零新依赖**：纯原生 JS（`getBoundingClientRect` / `scroll` / `resize` / `requestAnimationFrame`），ES 语法与原文件一致。
- 去掉了 `IntersectionObserver` 依赖与 `'IntersectionObserver' in window` 特性检测——改用的 API（scroll/resize/rAF）兼容性更广。

## 6. 遗留风险

- **R1（config，未测但需说明）**：`scrollIntoView(config-engine, center)` 时高亮 `config-timeouts` 而非 `config-engine`。原因：`#config-engine` 是 1646px 的大面板，`scrollIntoView(center)` 把其几何中心居中后，视口实际显示的是其下半部的 timeouts 区（timeouts 是视口内唯一完整 section）。语义上这反映"用户当前看到的内容"，但与"点击'引擎路径' nav 应高亮 engine"的朴素预期不同。web-smoke 不测 engine 单独滚动，无测试影响；若后续要求点击 engine nav 高亮 engine，需引入"点击 nav 临时锁定"机制（超出本次 scope）。
- **R2（report，物理限制非缺陷）**：`scrollTo(report-coverage, center)` 高亮 `report-risk` 而非 coverage。原因：coverage 中心(379) < 视口中心(480)，页面已处顶部无法继续上滚（scrollY 被 clamp 到 0），此刻 risk 与 coverage 同时完整在视口、几何状态等同于初始——算法一致地选首项 risk。用户继续下滚使 risk 离开视口后 coverage 即被选中。这是"页面顶部首项优先"与"目标无法被居中"的共同结果，不可消除且合理。
- **R3（静态推断，无法本地实测）**：CI 渲染环境（字体回退/DPI）可能与本地布局略有差异。本修复基于连续几何量与较大余量（见第 4 节），对 1px 乃至数十 px 的布局漂移不敏感；但极端字体差异若使某 section 高度剧变（如表格换行数翻倍），理论上仍可能改变"哪个 section 完整在视口"——届时兜底的"中心最近"仍会选出用户视野中心的 section，不会退回到旧算法的 DOM 顺序错选。
- **R4（预存基础设施，非本次改动）**：`internal/store/sqlite.go:27` 以 `path+"?_pragma=busy_timeout(5000)&_txlock=immediate"` 作 DSN，当测试以相对路径运行 db 时会在 cwd 残留名为 `?_pragma=...` 的垃圾 sqlite 文件。本次跑验收测试时产生过该文件，已清理；该问题与 scroll-spy 无关，未在本次修复。

## 7. Scope 与 git status

- **改动文件**：仅 `internal/web/static/app.js`（`initScrollSpy` 函数体 + 注释）。
- 未改动 `scripts/web-smoke.mjs`、`config.html`、`report.html`、`app.css`，未删除任何现有函数。
- **git status**（清理验收测试副产物后）：`M internal/web/static/app.js` + 既有未跟踪 `spikes/`、`docs/plans/fathom-integration/`。验收测试运行期间产生的 sqlite 垃圾文件（R4）与 ticket-04 截图快照覆盖已清理还原。
- 未执行任何 git commit / push / checkout / restore。
