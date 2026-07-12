# AnchorScan UI/UX 深度重设计实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 全面提升 AnchorScan 控制台的美观度与易用性，包括高对比度、响应式侧边栏、纯 HTML/CSS 漏洞分布可视化条、扫描实时 Stepper、过滤自动联动以及 IDE 风格的漏洞证据块。

**Architecture:** 前端采用纯原生 HTML + CSS 渐变/百分比实现轻量化渲染，并通过在 `app.js` 中增加客户端统计和事件监听，实现免修改后端的敏捷前端重构，避免了修改 Go 后端结构体所带来的数据库兼容问题。

**Tech Stack:** Go HTML Template, Vanilla CSS, Vanilla Javascript

## Global Constraints
- 所有修改只允许发生在 `internal/web/static/` 和 `internal/web/templates/` 目录。
- 禁止修改 Go 后端业务逻辑与数据库结构。
- 新增 CSS 类和样式属性必须符合 `Stealth Obsidian & Amber` 主题设计。

---

### Task 1: 调色板与全局 CSS 优化

**Files:**
- Modify: `internal/web/static/style.css:1-62` (重定义核心 CSS 变量，调高对比度与背景深度)
- Modify: `internal/web/static/style.css:115-240` (新增移动端页眉、遮罩样式规则)

**Interfaces:**
- Consumes: 无
- Produces: CSS 全局颜色对比度改善、基础移动端容器样式就绪

- [ ] **Step 1: 修改全局 CSS 变量，调高对比度**

在 `internal/web/static/style.css` 头部替换变量：
```css
:root {
  color-scheme: dark;

  /* Theme colors: Stealth Obsidian & Amber */
  --bg: #090a0c;                 /* Deep obsidian black */
  --bg-dots: rgba(249, 115, 22, 0.012); /* Barely visible warm grid dots */
  --bg-accent: #0b0c0f;          /* Secondary bg for inputs/sidebar - darker */
  --panel: #121316;              /* Dark charcoal panel background */
  --panel-strong: #17181c;       /* Solid dark panel background */
  --border: #22242a;             /* Ultra-crisp thin border - slightly lighter */
  --border-strong: #2c2f37;      /* Stronger divider border */
  --text: #d2d5dc;               /* Soft titanium gray text */
  --muted: #8e94a0;              /* Muted text color - lighter for WCAG AA (4.71:1) */
  --heading: #f0f2f5;            /* Bright header text */
```

- [ ] **Step 2: 在 style.css 中追加移动端页眉和侧边栏背景遮罩基础样式**

在 `internal/web/static/style.css` 底部追加：
```css
/* 移动端页眉 & 遮罩 */
.mobile-header { display: none; }
.sidebar-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  z-index: 99;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.25s ease;
}
```

- [ ] **Step 3: 运行 doctor 自检，验证 CSS 文件路径与服务器启动正常**

Run: `go run ./cmd/anchorscan doctor`
Expected: `doctor` 校验全部 PASS。

- [ ] **Step 4: 提交变更**

Run:
```bash
git add internal/web/static/style.css
git commit -m "style: optimize global contrast palette and prepare responsive container layout"
```

---

### Task 2: 响应式侧边栏布局 (Mobile Responsive Sidebar)

**Files:**
- Modify: `internal/web/templates/base.html` (引入移动端头部、遮罩，编写折叠联动 JS)
- Modify: `internal/web/static/style.css` (实现 `@media (max-width: 1024px)` 的侧边栏滑出和 page-shell 边距压缩)

**Interfaces:**
- Consumes: Task 1 中添加的 CSS 容器样式
- Produces: 响应式侧边栏折叠/滑出交互效果

- [ ] **Step 1: 修改 base.html 模板，引入移动端顶栏和遮罩**

在 `internal/web/templates/base.html` 的 `<body>` 标签内，`aside` 标签上方插入结构：
```html
<body class="app-shell">
  <header class="mobile-header">
    <button id="sidebar-toggle" class="sidebar-toggle-btn" aria-label="打开侧边栏">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 1.5rem; height: 1.5rem;">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
      </svg>
    </button>
    <div class="mobile-title">AnchorScan</div>
  </header>
  <div class="sidebar-overlay" id="sidebar-overlay"></div>
  <aside class="sidebar">
```

- [ ] **Step 2: 在 base.html 的 script 标签内追加侧边栏 toggle 的交互逻辑**

在 `internal/web/templates/base.html` 底部的 DOMContentLoaded script 中追加：
```javascript
      // 侧边栏折叠与遮罩控制
      const toggleBtn = document.getElementById("sidebar-toggle");
      const sidebar = document.querySelector(".sidebar");
      const overlay = document.getElementById("sidebar-overlay");
      if (toggleBtn && sidebar && overlay) {
        const toggleSidebar = () => {
          sidebar.classList.toggle("open");
          overlay.classList.toggle("active");
        };
        toggleBtn.addEventListener("click", toggleSidebar);
        overlay.addEventListener("click", toggleSidebar);
        document.querySelectorAll(".nav-item").forEach(item => {
          item.addEventListener("click", () => {
            sidebar.classList.remove("open");
            overlay.classList.remove("active");
          });
        });
      }
```

- [ ] **Step 3: 在 style.css 中追加大屏幕隐藏和窄屏媒体查询逻辑**

在 `internal/web/static/style.css` 底部追加：
```css
/* 窄屏断点适配 (max-width: 1024px) */
@media (max-width: 1024px) {
  .app-shell { flex-direction: column; }
  .mobile-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 56px;
    background: #0d0e11;
    border-bottom: 1px solid var(--border);
    padding: 0 1.25rem;
    position: sticky;
    top: 0;
    z-index: 98;
  }
  .mobile-title {
    font-size: 1.05rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--heading);
  }
  .sidebar-toggle-btn {
    background: transparent;
    border: none;
    color: var(--text);
    cursor: pointer;
    padding: 0.25rem;
    display: flex;
    align-items: center;
  }
  .sidebar {
    position: fixed;
    left: -240px;
    top: 0;
    bottom: 0;
    height: 100vh;
    transition: left 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    z-index: 100;
  }
  .sidebar.open { left: 0; }
  .sidebar-overlay.active { opacity: 1; pointer-events: auto; }
  .page-shell {
    width: 100%;
    padding: 1.25rem 1rem;
  }
}
```

- [ ] **Step 4: 运行全部单元测试确保没有模板编译错误**

Run: `make test`
Expected: 测试套件全部 PASS。

- [ ] **Step 5: 提交变更**

Run:
```bash
git add internal/web/templates/base.html internal/web/static/style.css
git commit -m "feat: add mobile header, sidebar overlay, and responsive media queries"
```

---

### Task 3: 漏洞分布可视化条与前端解析

**Files:**
- Modify: `internal/web/templates/report.html` (在漏洞列表上方插入可视化图表容器)
- Modify: `internal/web/static/style.css` (定义可视化条和 Legend 的布局结构样式)
- Modify: `internal/web/static/app.js` (编写前端 Findings 遍历统计与图表动态重绘函数)

**Interfaces:**
- Consumes: 报告页面已存在的 `.severity-badge` 进行前端 DOM 元素遍历
- Produces: 动态渲染的 HTML/CSS 漏洞分布占比图，随即时筛选自动刷新比例

- [ ] **Step 1: 在 report.html 漏洞面板头部插入可视化条的空容器**

在 `internal/web/templates/report.html` 的漏洞 Findings 面板中（`<h3>已发现的安全风险 (Findings)</h3>` 下方）插入容器：
```html
  <div class="vuln-distribution-container" id="distribution-container" style="display: none; margin-top: 0.85rem;">
    <div class="vuln-distribution-bar" id="distribution-bar"></div>
    <div class="vuln-distribution-legend" id="distribution-legend"></div>
  </div>
```

- [ ] **Step 2: 在 style.css 中注入可视化组件样式**

在 `internal/web/static/style.css` 底部追加：
```css
/* 漏洞分布条样式 */
.vuln-distribution-container {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 1.15rem 1.25rem;
  margin-bottom: 1.25rem;
}
.vuln-distribution-bar {
  display: flex;
  height: 10px;
  border-radius: 5px;
  overflow: hidden;
  background: #191b22;
  border: 1px solid rgba(255, 255, 255, 0.03);
}
.vuln-bar-segment {
  height: 100%;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  cursor: pointer;
}
.vuln-bar-segment:hover::after {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(255, 255, 255, 0.15);
}
.vuln-bar-segment.critical { background: var(--sev-critical); }
.vuln-bar-segment.high { background: var(--sev-high); }
.vuln-bar-segment.medium { background: var(--sev-medium); }
.vuln-bar-segment.low { background: var(--sev-low); }
.vuln-bar-segment.info { background: var(--sev-info); }

.vuln-distribution-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem 1.5rem;
  margin-top: 0.75rem;
}
.legend-item {
  font-size: 0.8rem;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.legend-dot.critical { background: var(--sev-critical); }
.legend-dot.high { background: var(--sev-high); }
.legend-dot.medium { background: var(--sev-medium); }
.legend-dot.low { background: var(--sev-low); }
.legend-dot.info { background: var(--sev-info); }
.legend-count {
  font-weight: 700;
  color: var(--heading);
  font-family: var(--mono);
}
```

- [ ] **Step 3: 在 app.js 中实现前端提取与渲染逻辑，并在 DOMContentLoaded 中调用它**

在 `internal/web/static/app.js` 底部加入漏洞分布条统计与计算逻辑：
```javascript
function renderVulnDistribution() {
  const container = document.getElementById('distribution-container');
  const bar = document.getElementById('distribution-bar');
  const legend = document.getElementById('distribution-legend');
  if (!container || !bar || !legend) return;

  const badges = document.querySelectorAll('.severity-badge');
  if (badges.length === 0) {
    container.style.display = 'none';
    return;
  }

  const counts = { critical: 0, high: 0, medium: 0, low: 0, info: 0 };
  badges.forEach(badge => {
    const text = badge.textContent.trim().toLowerCase();
    if (counts.hasOwnProperty(text)) {
      counts[text]++;
    }
  });

  const total = Object.values(counts).reduce((a, b) => a + b, 0);
  if (total === 0) {
    container.style.display = 'none';
    return;
  }

  container.style.display = 'block';

  let barHTML = '';
  let legendHTML = '';

  const labelMap = {
    critical: '严重 (Critical)',
    high: '高危 (High)',
    medium: '中危 (Medium)',
    low: '低危 (Low)',
    info: '信息 (Info)'
  };

  Object.entries(counts).forEach(([sev, count]) => {
    if (count > 0) {
      const pct = ((count / total) * 100).toFixed(1);
      barHTML += `<div class="vuln-bar-segment ${sev}" style="width: ${pct}%;" title="${labelMap[sev]}: ${count} (${pct}%)"></div>`;
    }
    legendHTML += `
      <span class="legend-item">
        <span class="legend-dot ${sev}"></span>
        ${labelMap[sev]}: <span class="legend-count">${count}</span>
      </span>
    `;
  });

  bar.innerHTML = barHTML;
  legend.innerHTML = legendHTML;
}

// 绑定 DOM 载入回调
document.addEventListener('DOMContentLoaded', () => {
  renderVulnDistribution();
});
```

- [ ] **Step 4: 运行 `make test` 确保无测试报错**

Run: `make test`
Expected: PASS

- [ ] **Step 5: 提交变更**

Run:
```bash
git add internal/web/templates/report.html internal/web/static/style.css internal/web/static/app.js
git commit -m "feat: implement dynamic HTML/CSS vulnerability distribution bar"
```

---

### Task 4: 实时扫描阶段进度条 (Dynamic Stepper)

**Files:**
- Modify: `internal/web/templates/run.html` (在日志流上方插入步进器骨架)
- Modify: `internal/web/static/style.css` (编写 Stepper / Phase 步进器的线条与圆点样式)
- Modify: `internal/web/static/app.js` (实现 `updateStepper()`，基于事件 Stage 动态激活节点)

**Interfaces:**
- Consumes: `app.js` 轮询返回的 `events` 中的 `e.stage` 及任务状态 `runStatus`
- Produces: 运行时的 5 步进度指示器

- [ ] **Step 1: 修改 run.html 模板，在 pre 标签的父容器上方插入进度指示器**

在 `internal/web/templates/run.html` 中 `<h3>引擎级联流水线实时监控</h3>` 下方插入：
```html
  <div class="scan-stepper" id="scan-stepper">
    <div class="step" id="step-discover">
      <div class="step-icon">1</div>
      <div class="step-label">主机存活</div>
    </div>
    <div class="step-line"></div>
    <div class="step" id="step-portscan">
      <div class="step-icon">2</div>
      <div class="step-label">端口扫描</div>
    </div>
    <div class="step-line"></div>
    <div class="step" id="step-fingerprint">
      <div class="step-icon">3</div>
      <div class="step-label">服务指纹</div>
    </div>
    <div class="step-line"></div>
    <div class="step" id="step-vuln">
      <div class="step-icon">4</div>
      <div class="step-label">脆弱性检测</div>
    </div>
    <div class="step-line"></div>
    <div class="step" id="step-report">
      <div class="step-icon">5</div>
      <div class="step-label">报告生成</div>
    </div>
  </div>
```

- [ ] **Step 2: 在 style.css 中追加 Stepper 进度条的定位和点亮样式**

在 `internal/web/static/style.css` 底部追加：
```css
/* Stepper 进度条 */
.scan-stepper {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--panel-strong);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 1.25rem 2rem;
  margin-bottom: 1.5rem;
}
.step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  z-index: 2;
  flex: 1;
}
.step-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: #17181c;
  border: 2px solid var(--border-strong);
  color: var(--muted);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8rem;
  font-weight: 700;
  font-family: var(--mono);
  transition: all 0.3s ease;
}
.step-label {
  font-size: 0.78rem;
  color: var(--muted);
  font-weight: 600;
  text-align: center;
  transition: all 0.3s ease;
}
.step-line {
  flex-grow: 1;
  height: 2px;
  background: var(--border-strong);
  margin: 0 -0.5rem;
  margin-top: -18px;
  z-index: 1;
  transition: all 0.3s ease;
}
.step.active .step-icon {
  border-color: var(--primary);
  color: var(--primary);
  box-shadow: 0 0 10px var(--primary-glow);
  background: var(--primary-soft);
}
.step.active .step-label {
  color: var(--heading);
}
.step.completed .step-icon {
  border-color: var(--success);
  color: #000000;
  background: var(--success);
}
.step.completed .step-label {
  color: var(--success);
}
.step-line.completed {
  background: var(--success);
}
```

- [ ] **Step 3: 在 app.js 中实现 updateStepper，并在 refreshEvents 内触发**

在 `internal/web/static/app.js` 的 `refreshEvents()` 轮询中（在 `box.innerHTML = ...` 这一行后）调用更新：
```javascript
    // 触发 Stepper 更新
    const statusText = document.getElementById('run-status')?.textContent || window.anchorRunStatus || '';
    updateStepper(events, statusText);
```

并在 `app.js` 底部加入 `updateStepper` 实现：
```javascript
function updateStepper(events, runStatus) {
  const steps = {
    discover: document.getElementById('step-discover'),
    portscan: document.getElementById('step-portscan'),
    fingerprint: document.getElementById('step-fingerprint'),
    vuln: document.getElementById('step-vuln'),
    report: document.getElementById('step-report')
  };
  const lines = document.querySelectorAll('.step-line');
  if (!steps.discover) return;

  if ((runStatus || '').toLowerCase() === 'completed') {
    Object.values(steps).forEach(s => {
      s.className = 'step completed';
      s.querySelector('.step-icon').innerHTML = '✓';
    });
    lines.forEach(l => l.className = 'step-line completed');
    return;
  }

  let currentStage = 'init';
  events.forEach(e => {
    const stage = (e.stage || '').toLowerCase();
    const msg = (e.message || '').toLowerCase();

    if (stage === 'nmap' && msg.includes('alive')) {
      currentStage = 'discover';
    } else if (stage === 'rustscan') {
      currentStage = 'portscan';
    } else if (stage === 'nmap' && !msg.includes('alive')) {
      currentStage = 'fingerprint';
    } else if (stage === 'httpx' || stage === 'nse' || stage === 'nuclei') {
      currentStage = 'vuln';
    } else if (stage === 'report') {
      currentStage = 'report';
    }
  });

  const stageOrder = ['discover', 'portscan', 'fingerprint', 'vuln', 'report'];
  const currentIndex = stageOrder.indexOf(currentStage);

  stageOrder.forEach((stageName, idx) => {
    const stepEl = steps[stageName];
    if (!stepEl) return;

    if (idx < currentIndex) {
      stepEl.className = 'step completed';
      stepEl.querySelector('.step-icon').innerHTML = '✓';
      if (lines[idx - 1]) lines[idx - 1].className = 'step-line completed';
    } else if (idx === currentIndex) {
      stepEl.className = 'step active';
      stepEl.querySelector('.step-icon').innerHTML = idx + 1;
      if (lines[idx - 1]) lines[idx - 1].className = 'step-line completed';
    } else {
      stepEl.className = 'step';
      stepEl.querySelector('.step-icon').innerHTML = idx + 1;
      if (lines[idx - 1]) lines[idx - 1].className = 'step-line';
    }
  });
}
```

- [ ] **Step 4: 运行 `make test` 确保无异常**

Run: `make test`
Expected: PASS

- [ ] **Step 5: 提交变更**

Run:
```bash
git add internal/web/templates/run.html internal/web/static/style.css internal/web/static/app.js
git commit -m "feat: implement real-time dynamic stepper for run monitor"
```

---

### Task 5: 报告过滤面板折叠与联动即时过滤

**Files:**
- Modify: `internal/web/templates/report.html` (重构过滤表单为折叠面板)
- Modify: `internal/web/static/style.css` (高级过滤折叠与动画滑出样式)
- Modify: `internal/web/static/app.js` (高级面板折叠交互绑定、选择框与 Chip 的即时 submit() 联动)

**Interfaces:**
- Consumes: 过滤表单的 `#report-filter-form` DOM 提交事件
- Produces: 支持折叠的高级面板，点击危险 Chip 或视图模式直接刷新数据

- [ ] **Step 1: 修改 report.html 表单结构，划分为核心搜索与高级面板**

将 `internal/web/templates/report.html` 的数据过滤表单（在 `<form class="filter-grid" method="get">` 到 `</form>` 之间）重构为：
```html
  <form class="filter-grid-layout" method="get" id="report-filter-form">
    <!-- 核心过滤行 -->
    <div class="filter-main-row">
      <div class="input-with-icon" style="flex: 1; min-width: 200px; position: relative;">
        <span class="search-icon">🔍</span>
        <input name="ip" placeholder="检索 IP 地址 (如 192.168.1.1)" value="{{.Filters.IP}}" style="padding-left: 2.15rem;">
      </div>
      <div class="input-with-icon" style="flex: 1; min-width: 200px; position: relative;">
        <span class="search-icon">🔍</span>
        <input name="q" placeholder="检索漏洞 ID 或描述关键词" value="{{.Filters.Keyword}}" style="padding-left: 2.15rem;">
      </div>
      <button class="button button-secondary" type="button" id="btn-toggle-advanced">
        <span>高级筛选</span>
        <svg class="chevron-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 0.75rem; height: 0.75rem; transition: transform 0.2s ease;">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
        </svg>
      </button>
      <button class="button button-primary" type="submit">开始筛选</button>
    </div>

    <!-- 高级隐藏面板 -->
    <div class="filter-advanced-panel" id="advanced-filter-panel" style="display: none; padding-top: 1rem; border-top: 1px dashed var(--border); margin-top: 0.85rem;">
      <div class="form-grid">
        <label>
          <span>过滤端口</span>
          <input name="port" placeholder="特定端口 (如 80)" value="{{.Filters.Port}}">
        </label>
        <label>
          <span>服务精确匹配</span>
          <input name="service" placeholder="服务名称 (如 redis)" value="{{.Filters.Service}}">
        </label>
        <label>
          <span>数据源</span>
          <input name="source" placeholder="数据源 (nuclei)" value="{{.Filters.Source}}">
        </label>
        <label>
          <span>视图模式</span>
          <select name="view" id="filter-view-select">
            <option value="ports" {{if eq .AssetView "ports"}}selected{{end}}>按端口列表</option>
            <option value="hosts" {{if eq .AssetView "hosts"}}selected{{end}}>按主机聚合</option>
          </select>
        </label>
      </div>
    </div>

    <!-- 等级过滤 -->
    <div class="severity-filter-container" style="margin-top: 0.85rem; border-top: 1px solid var(--border); padding-top: 0.85rem;">
      <span class="severity-filter-title" style="font-size: 0.8rem; font-weight: 600; color: var(--muted);">危险等级过滤</span>
      <div class="severity-filter-group" style="display: flex; flex-wrap: wrap; gap: 0.6rem; margin-top: 0.4rem;">
        <label class="severity-filter-chip critical{{if contains .Filters.Severities "critical"}} active{{end}}">
          <input type="checkbox" name="severity" value="critical" {{if contains .Filters.Severities "critical"}}checked{{end}}>
          <span class="severity-dot"></span>
          critical
        </label>
        <label class="severity-filter-chip high{{if contains .Filters.Severities "high"}} active{{end}}">
          <input type="checkbox" name="severity" value="high" {{if contains .Filters.Severities "high"}}checked{{end}}>
          <span class="severity-dot"></span>
          high
        </label>
        <label class="severity-filter-chip medium{{if contains .Filters.Severities "medium"}} active{{end}}">
          <input type="checkbox" name="severity" value="medium" {{if contains .Filters.Severities "medium"}}checked{{end}}>
          <span class="severity-dot"></span>
          medium
        </label>
        <label class="severity-filter-chip low{{if contains .Filters.Severities "low"}} active{{end}}">
          <input type="checkbox" name="severity" value="low" {{if contains .Filters.Severities "low"}}checked{{end}}>
          <span class="severity-dot"></span>
          low
        </label>
        <label class="severity-filter-chip info{{if contains .Filters.Severities "info"}} active{{end}}">
          <input type="checkbox" name="severity" value="info" {{if contains .Filters.Severities "info"}}checked{{end}}>
          <span class="severity-dot"></span>
          info
        </label>
      </div>
    </div>
  </form>
```
> 注：原 `report.html` 中 `onchange` checkbox 属性去除，统一由 `app.js` 完成优雅的监听器劫持。

- [ ] **Step 2: 在 style.css 中追加高级隐藏面板滑出动画**

在 `internal/web/static/style.css` 底部追加：
```css
/* 高级筛选折叠样式 */
.search-icon {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: var(--muted);
  font-size: 0.85rem;
  pointer-events: none;
}
.filter-advanced-panel {
  animation: slideDown 0.2s cubic-bezier(0.4, 0, 0.2, 1) forwards;
}
@keyframes slideDown {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}
```

- [ ] **Step 3: 在 app.js 的 DOMContentLoaded 中加入事件折叠展开和即时联动**

在 `internal/web/static/app.js` 的 `DOMContentLoaded` 监听内追加：
```javascript
  // 1. 高级过滤选项折叠/展开
  const toggleAdvBtn = document.getElementById('btn-toggle-advanced');
  const advPanel = document.getElementById('advanced-filter-panel');
  if (toggleAdvBtn && advPanel) {
    toggleAdvBtn.addEventListener('click', (e) => {
      e.preventDefault();
      const isHidden = advPanel.style.display === 'none';
      advPanel.style.display = isHidden ? 'block' : 'none';
      toggleAdvBtn.querySelector('.chevron-icon').style.transform = isHidden ? 'rotate(180deg)' : 'rotate(0deg)';
    });
  }

  // 2. 联动即时筛选
  const viewSelect = document.getElementById('filter-view-select');
  if (viewSelect) {
    viewSelect.addEventListener('change', () => viewSelect.closest('form').submit());
  }
  document.querySelectorAll('.severity-filter-chip input[type="checkbox"]').forEach(box => {
    // 恢复初次渲染状态
    box.parentElement.classList.toggle('active', box.checked);
    box.addEventListener('change', function() {
      this.parentElement.classList.toggle('active', this.checked);
      this.closest('form').submit();
    });
  });
```

- [ ] **Step 4: 运行 `make test` 确保无异常**

Run: `make test`
Expected: PASS

- [ ] **Step 5: 提交变更**

Run:
```bash
git add internal/web/templates/report.html internal/web/static/style.css internal/web/static/app.js
git commit -m "feat: implement collapsible filter panel and auto-filtering link"
```

---

### Task 6: IDE-style 证据验证块与一键复制

**Files:**
- Modify: `internal/web/templates/report.html` (重构漏洞原始证据详情行的展开 HTML，更换为 IDE-style 证据外框)
- Modify: `internal/web/static/style.css` (编写精细的 IDE 代码框、顶部复制条样式)
- Modify: `internal/web/static/app.js` (在 DOMContentLoaded 注入一键复制监听及提示重置 JS)

**Interfaces:**
- Consumes: `evidence-pre-{{$index}}` 的 innerText
- Produces: 精美的 IDE 代码响应阅读器，带复制功能

- [ ] **Step 1: 修改 report.html 漏洞详情详情行 HTML，套入 IDE-style 容器**

在 `internal/web/templates/report.html` 中 Findings 展开详情部分（即 `<tr class="details-row" id="finding-details-{{$index}}">` 内）替换为：
```html
        <tr class="details-row" id="finding-details-{{$index}}" style="display: none;">
          <td colspan="6">
            <div class="details-expanded-content" style="padding: 0.85rem 1rem;">
              {{if .Output}}
              <div class="evidence-container">
                <div class="evidence-header">
                  <div style="display: flex; align-items: center; gap: 0.45rem;">
                    <span>📄</span>
                    <span>漏洞验证证据报文 / 响应 RAW OUTPUT</span>
                  </div>
                  <button class="evidence-copy-btn" type="button" data-copy-target-id="evidence-pre-{{$index}}">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" style="width: 0.85rem; height: 0.85rem;">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125v-9.75a1.125 1.125 0 011.125-1.125h3.375m1.5 1.5h9.75a1.125 1.125 0 011.125 1.125v-9.75a1.125 1.125 0 01-1.125-1.125h-9.75a1.125 1.125 0 01-1.125 1.125v9.75s0 .375.375.375h.75" />
                    </svg>
                    <span>复制数据</span>
                  </button>
                </div>
                <div class="evidence-body">
                  <pre class="evidence-pre" id="evidence-pre-{{$index}}">{{.Output}}</pre>
                </div>
              </div>
              {{else}}
              <p class="evidence-empty" style="color: var(--muted); font-size: 0.85rem; margin: 0; padding: 0.5rem 0;">暂无原始证据输出。</p>
              {{end}}
            </div>
          </td>
        </tr>
```

- [ ] **Step 2: 在 style.css 中追加精美的 IDE-like 代码阅读器外观样式**

在 `internal/web/static/style.css` 底部追加：
```css
/* IDE 证据框 */
.evidence-container {
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-lg);
  overflow: hidden;
  margin-top: 0.75rem;
  background: #050608;
  text-align: left;
}
.evidence-header {
  background: #131418;
  border-bottom: 1px solid var(--border);
  padding: 0.5rem 0.85rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.75rem;
  color: var(--muted);
  font-weight: 600;
  letter-spacing: 0.02em;
}
.evidence-pre {
  margin: 0;
  padding: 1rem;
  overflow-x: auto;
  font-family: var(--mono);
  font-size: 0.8rem;
  line-height: 1.6;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 24rem;
}
.evidence-copy-btn {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  font-size: 0.72rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 0.35rem;
  transition: all 0.15s ease;
}
.evidence-copy-btn:hover {
  color: var(--primary);
}
```

- [ ] **Step 3: 在 app.js 中实现一键复制与复制成功状态逻辑**

在 `internal/web/static/app.js` 的 `DOMContentLoaded` 监听内追加：
```javascript
  // 证据一键复制
  document.querySelectorAll('[data-copy-target-id]').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      e.preventDefault();
      const targetId = btn.getAttribute('data-copy-target-id');
      const targetEl = document.getElementById(targetId);
      if (!targetEl) return;

      const text = targetEl.innerText || targetEl.textContent || '';
      btn.disabled = true;
      const originalHTML = btn.innerHTML;
      try {
        await writeClipboard(text.trimEnd());
        btn.innerHTML = `
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 0.85rem; height: 0.85rem; color: var(--success);">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
          <span style="color: var(--success); font-weight: 700;">已复制!</span>
        `;
      } catch (err) {
        btn.innerHTML = `<span style="color: var(--danger);">复制失败</span>`;
      }
      setTimeout(() => {
        btn.disabled = false;
        btn.innerHTML = originalHTML;
      }, 1200);
    });
  });
```

- [ ] **Step 4: 运行 `make test` 确保无异常**

Run: `make test`
Expected: PASS

- [ ] **Step 5: 提交变更**

Run:
```bash
git add internal/web/templates/report.html internal/web/static/style.css internal/web/static/app.js
git commit -m "feat: design IDE-style code layout and click-to-copy trigger for findings evidence"
```
