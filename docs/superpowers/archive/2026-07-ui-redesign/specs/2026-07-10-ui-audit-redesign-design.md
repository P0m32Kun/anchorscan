# 设计规格说明书: AnchorScan UI/UX 深度重设计 (2026-07-10)

本设计规格说明书针对 AnchorScan Web 控制台的视觉美化与交互体验（易用性）进行深度重构。项目在保留原有 Stealth Obsidian (深黑矿石) 硬核工业风的同时，重点解决低对比度、缺乏数据可视化、小屏遮挡、过滤复杂和运行状态盲区等核心痛点。

---

## 1. 痛点与重设计目标

1. **对比度提升**：将 `--muted` 色度提高，使浅色辅助字在黑底下的对比度达到 `4.5:1` 以上，符合 WCAG AA 规范。
2. **小屏响应式侧边栏**：窗口宽度 $\le 1024\text{px}$ 时自动折叠 Aside，通过汉堡菜单滑出，并为主要内容区（page-shell）增加紧凑边距，解决表格列破位溢出的问题。
3. **数据可视化条（漏洞分布）**：在报告页注入纯 HTML/CSS 堆叠胶囊条。**由前端 JS 动态解析 Findings 数据行生成**。这样可以在用户使用关键词/IP 过滤结果时，图表占比实现实时比例重绘与动态缩放，无需依赖后端。
4. **实时扫描 Stepper**：为实时监控页 (`run.html`) 引入阶段步进指示器，通过轮询解析事件 Stage，使执行进度可视化。
5. **折叠高级过滤与即时触发**：合并报告筛选字段，默认收起低频项；改变危险级别 Checkbox 或视图模式 Select 时，直接自动触发过滤提交。
6. **IDE-Style 证据展示框**：重构漏洞详情的 Evidence Pre 块，加入顶部栏说明、行号排版和一键复制证据文本按钮。

---

## 2. 影响范围

修改涉及以下 5 个前端文件（均位于 `internal/web/` 目录下）：
1. [internal/web/static/style.css](file:///Users/kun/DEV/new-Anchor/internal/web/static/style.css) (CSS 样式扩展与断点媒体查询)
2. [internal/web/static/app.js](file:///Users/kun/DEV/new-Anchor/internal/web/static/app.js) (动态 Stepper、可视化图表前端统计与即时联动 JS 逻辑)
3. [internal/web/templates/base.html](file:///Users/kun/DEV/new-Anchor/internal/web/templates/base.html) (移动端头部与汉堡按钮、遮罩引入)
4. [internal/web/templates/run.html](file:///Users/kun/DEV/new-Anchor/internal/web/templates/run.html) (在事件日志上方插入 Stepper 组件容器)
5. [internal/web/templates/report.html](file:///Users/kun/DEV/new-Anchor/internal/web/templates/report.html) (插入可视化分布条、重构过滤表单折叠结构、IDE 证据框 HTML 化)

---

## 3. 具体实施规格与代码细节

### 3.1 调色板与全局 CSS 样式优化 (`style.css`)
微调颜色变量，增加折叠状态、进度条及 IDE 证据块的样式：

```css
:root {
  /* ... 原有变量保持不变 ... */
  /* 优化 muted 的对比度，从 #666a73 提升为 #8e94a0，对比度达 4.71:1 */
  --muted: #8e94a0;
  --border: #22242a;             /* 边界线微加深 */
  --bg-accent: #0b0c0f;          /* 底色微变暗，增强层次 */
}

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

/* 进度条 Stepper 样式 */
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

/* 过滤布局折叠 */
.filter-main-row {
  display: flex;
  gap: 0.65rem;
  align-items: center;
  flex-wrap: wrap;
}
.filter-main-row .input-with-icon {
  flex: 1;
  min-width: 200px;
  position: relative;
}
.filter-main-row input {
  padding-left: 2.15rem;
}
.search-icon {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: var(--muted);
  font-size: 0.85rem;
}
.filter-advanced-panel {
  padding-top: 1rem;
  border-top: 1px dashed var(--border);
  margin-top: 0.85rem;
  animation: slideDown 0.2s cubic-bezier(0.4, 0, 0.2, 1) forwards;
}

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

---

## 3.2 基础模板调整 (`base.html`)
引入页眉、遮罩与侧边栏控制脚本：

在导航和 Aside 外引入：
```html
  <header class="mobile-header">
    <button id="sidebar-toggle" class="sidebar-toggle-btn" aria-label="打开侧边栏">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 1.5rem; height: 1.5rem;">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
      </svg>
    </button>
    <div class="mobile-title">AnchorScan</div>
  </header>
  <div class="sidebar-overlay" id="sidebar-overlay"></div>
```

在 JS 块中加入开关逻辑：
```javascript
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

---

## 3.3 运行页调整 (`run.html`)
在 `event-log` 流面板之上注入 Stepper 骨架：

```html
<div class="scan-stepper" id="scan-stepper">
  <div class="step" id="step-discover">
    <div class="step-icon">1</div>
    <div class="step-label">主机存活探测</div>
  </div>
  <div class="step-line"></div>
  <div class="step" id="step-portscan">
    <div class="step-icon">2</div>
    <div class="step-label">端口开放发现</div>
  </div>
  <div class="step-line"></div>
  <div class="step" id="step-fingerprint">
    <div class="step-icon">3</div>
    <div class="step-label">服务指纹识别</div>
  </div>
  <div class="step-line"></div>
  <div class="step" id="step-vuln">
    <div class="step-icon">4</div>
    <div class="step-label">漏洞与指纹双检测</div>
  </div>
  <div class="step-line"></div>
  <div class="step" id="step-report">
    <div class="step-icon">5</div>
    <div class="step-label">报告生成落库</div>
  </div>
</div>
```

---

## 3.4 报告页调整 (`report.html`)
插入前端分析驱动的漏洞危害统计条，并改动数据过滤为折叠结构，以及更换 IDE 证据结构：

1. **头部 Findings 前面插入可视化容器**：
```html
<div class="vuln-distribution-container" id="distribution-container" style="display: none;">
  <div class="vuln-distribution-bar" id="distribution-bar"></div>
  <div class="vuln-distribution-legend" id="distribution-legend"></div>
</div>
```

2. **重新设计过滤面板 (`report-filter`)**：
改用 `filter-grid-layout` 与 `filter-main-row` + `filter-advanced-panel` 结构，提供 `id="btn-toggle-advanced"`。

3. **证据详情框 HTML 改进**：
在 `Findings` 的展开详情行 (`details-row`)：
```html
<tr class="details-row" id="finding-details-{{$index}}" style="display: none;">
  <td colspan="6">
    <div class="details-expanded-content">
      {{if .Output}}
      <div class="evidence-container">
        <div class="evidence-header">
          <div style="display: flex; align-items: center; gap: 0.45rem;">
            <span>📄</span>
            <span>漏洞验证证据报文 (RAW RESPONSE)</span>
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
      <p class="evidence-empty">暂无原始证据输出。</p>
      {{end}}
    </div>
  </td>
</tr>
```

---

## 3.5 交互与状态控制器更新 (`app.js`)
在 JS 中实现 Stepper 驱动、可视化危害条的前端解析与重绘、一键复制证据、以及即时表单过滤联动：

#### **动态更新运行 Stepper 逻辑**
在 `refreshEvents` 的回调中，轮询最新事件，调用并更新 Stepper：
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

#### **前端 Findings 统计与动态危害百分比绘图**
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
```

#### **DOM 加载控制与一键复制**
```javascript
document.addEventListener('DOMContentLoaded', () => {
  // 高级筛选面板折叠
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

  // 联动即时过滤
  const viewSelect = document.getElementById('filter-view-select');
  if (viewSelect) {
    viewSelect.addEventListener('change', () => viewSelect.closest('form').submit());
  }
  document.querySelectorAll('.severity-filter-chip input[type="checkbox"]').forEach(box => {
    box.addEventListener('change', function() {
      this.parentElement.classList.toggle('active', this.checked);
      this.closest('form').submit();
    });
  });

  // 证据一键复制
  document.querySelectorAll('[data-copy-target-id]').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      e.preventDefault();
      const targetId = btn.getAttribute('data-copy-target-id');
      const targetEl = document.getElementById(targetId);
      if (!targetEl) return;

      const text = targetEl.innerText || targetEl.textContent || '';
      btn.disabled = true;
      const originalText = btn.innerHTML;
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
        btn.innerHTML = originalText;
      }, 1200);
    });
  });

  // 渲染漏洞条
  renderVulnDistribution();
});
```

---

## 4. 验证与测试计划

1. **功能联动性验证**：
   - 输入检索条件进行过滤，观察顶部漏洞统计条是否实时更新。
   - 点击危重级别 chip，观察页面是否无需点击“开始过滤”按钮而瞬间发生表单联动过滤。
2. **样式适配性验证**：
   - 小屏幕下 Aside 侧边栏是否隐藏，汉堡按钮和蒙层响应是否灵敏。
   - 漏洞响应包 Evidence 展示的 IDE 化，文本换行与溢出表现是否正常。
3. **监控功能验证**：
   - 运行中任务的 Stepper 是否会随着 `events` 所含的 Stage 自动切换激活节点（1 -> 2 -> 3 -> 4 -> 5），以及在完成时是否全部转变为勾选（✓）状态。
