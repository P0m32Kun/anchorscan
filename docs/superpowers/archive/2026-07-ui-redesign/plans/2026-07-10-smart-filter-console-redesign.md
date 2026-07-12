# 智能搜索控制台重设计实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重构漏洞与资产检索面板，通过智能单搜索框、Popover 下拉选择器和动态 Filter Badges，为用户提供极其一致、美观且符合现代 SaaS 体验的检索控制台。

**Architecture:** 完全在前端（HTML 模板、CSS、JS）中闭环实现，由 JS 动态抓取表单状态并统计渲染，主搜索框根据正则路由自动分配参数，保持与已有 Go 后端过滤逻辑的完全兼容。

**Tech Stack:** Go HTML Template, Vanilla CSS, Vanilla Javascript

## Global Constraints
- 所有修改只允许发生在 `internal/web/static/` 和 `internal/web/templates/` 目录。
- 禁止修改 Go 后端业务逻辑与数据库结构。
- 交互和颜色设计必须完全符合 `Stealth Obsidian & Amber` 暗黑工业风规范。

---

### Task 1: HTML 模板重构

**Files:**
- Modify: `internal/web/templates/report.html:45-94` (重构数据过滤卡片为智能检索控制台骨架)

**Interfaces:**
- Consumes: 后端绑定的 `.Filters` 属性
- Produces: 含有 ID `smart-search-input`、Popover 面板以及 `active-filter-badges` 的 HTML 视图

- [ ] **Step 1: 在 report.html 中替换原有的 report-filter 表单部分**

在 `internal/web/templates/report.html` 中，将 `<section class="panel report-filter">` 这一整段卡片内容（大致在 45 行至 94 行）完全重构为：
```html
<section class="panel report-filter" style="padding: 1.25rem;">
  <form class="search-console-form" method="get" id="report-filter-form">

    <!-- 1. 智能检索主栏 -->
    <div class="search-console-bar">
      <div class="search-input-wrapper">
        <span class="search-icon">🔍</span>
        <input type="text" id="smart-search-input" placeholder="输入主机 IP、网段或漏洞关键词检索，回车或点击应用..." value="{{if .Filters.IP}}{{.Filters.IP}}{{else}}{{.Filters.Keyword}}{{end}}">
        <input type="hidden" name="ip" id="hidden-ip" value="{{.Filters.IP}}">
        <input type="hidden" name="q" id="hidden-q" value="{{.Filters.Keyword}}">
      </div>
      <button class="button button-primary search-submit-btn" type="submit">应用检索</button>
    </div>

    <!-- 2. 下拉维度过滤浮窗行 -->
    <div class="filter-popover-row">

      <!-- ⚠️ 危险级别 Popover -->
      <div class="popover-wrapper">
        <button class="popover-trigger-btn" type="button" data-popover-target="popover-severity">
          <span>⚠️ 危险级别</span>
          <span class="trigger-active-count" id="active-severity-count" style="display: none;">0</span>
          <svg class="chevron-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 0.7rem; height: 0.7rem; transition: transform 0.15s ease;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
          </svg>
        </button>
        <div class="popover-panel" id="popover-severity" style="display: none;">
          <div class="popover-panel-header">过滤危险级别</div>
          <div class="popover-panel-body">
            <div class="popover-checkbox-list">
              <label class="popover-checkbox-item">
                <input type="checkbox" name="severity" value="critical" {{if contains .Filters.Severities "critical"}}checked{{end}}>
                <span class="severity-dot critical"></span>
                <span>critical</span>
              </label>
              <label class="popover-checkbox-item">
                <input type="checkbox" name="severity" value="high" {{if contains .Filters.Severities "high"}}checked{{end}}>
                <span class="severity-dot high"></span>
                <span>high</span>
              </label>
              <label class="popover-checkbox-item">
                <input type="checkbox" name="severity" value="medium" {{if contains .Filters.Severities "medium"}}checked{{end}}>
                <span class="severity-dot medium"></span>
                <span>medium</span>
              </label>
              <label class="popover-checkbox-item">
                <input type="checkbox" name="severity" value="low" {{if contains .Filters.Severities "low"}}checked{{end}}>
                <span class="severity-dot low"></span>
                <span>low</span>
              </label>
              <label class="popover-checkbox-item">
                <input type="checkbox" name="severity" value="info" {{if contains .Filters.Severities "info"}}checked{{end}}>
                <span class="severity-dot info"></span>
                <span>info</span>
              </label>
            </div>
          </div>
          <div class="popover-panel-footer">
            <button class="button button-primary popover-apply-btn" type="submit">应用</button>
          </div>
        </div>
      </div>

      <!-- 🔌 端口与服务 Popover -->
      <div class="popover-wrapper">
        <button class="popover-trigger-btn" type="button" data-popover-target="popover-ports">
          <span>🔌 端口与服务</span>
          <svg class="chevron-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 0.7rem; height: 0.7rem; transition: transform 0.15s ease;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
          </svg>
        </button>
        <div class="popover-panel" id="popover-ports" style="display: none;">
          <div class="popover-panel-header">端口与服务过滤</div>
          <div class="popover-panel-body">
            <div class="popover-form-group">
              <label>
                <span>特定端口</span>
                <input name="port" placeholder="例如: 80" value="{{.Filters.Port}}">
              </label>
              <label>
                <span>服务精确匹配</span>
                <input name="service" placeholder="例如: redis" value="{{.Filters.Service}}">
              </label>
            </div>
          </div>
          <div class="popover-panel-footer">
            <button class="button button-primary popover-apply-btn" type="submit">应用</button>
          </div>
        </div>
      </div>

      <!-- 📦 数据源 Popover -->
      <div class="popover-wrapper">
        <button class="popover-trigger-btn" type="button" data-popover-target="popover-source">
          <span>📦 数据源</span>
          <svg class="chevron-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 0.7rem; height: 0.7rem; transition: transform 0.15s ease;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
          </svg>
        </button>
        <div class="popover-panel" id="popover-source" style="display: none;">
          <div class="popover-panel-header">数据源过滤</div>
          <div class="popover-panel-body">
            <div class="popover-form-group">
              <label>
                <span>探针数据源</span>
                <input name="source" placeholder="例如: nuclei" value="{{.Filters.Source}}">
              </label>
            </div>
          </div>
          <div class="popover-panel-footer">
            <button class="button button-primary popover-apply-btn" type="submit">应用</button>
          </div>
        </div>
      </div>

      <!-- 👁️ 视图模式 Popover -->
      <div class="popover-wrapper">
        <button class="popover-trigger-btn" type="button" data-popover-target="popover-view">
          <span>👁️ 视图模式</span>
          <svg class="chevron-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor" style="width: 0.7rem; height: 0.7rem; transition: transform 0.15s ease;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
          </svg>
        </button>
        <div class="popover-panel" id="popover-view" style="display: none;">
          <div class="popover-panel-header">切换视图呈现</div>
          <div class="popover-panel-body">
            <div class="popover-form-group">
              <select name="view" id="filter-view-select">
                <option value="ports" {{if eq .AssetView "ports"}}selected{{end}}>按端口列表</option>
                <option value="hosts" {{if eq .AssetView "hosts"}}selected{{end}}>按主机聚合</option>
              </select>
            </div>
          </div>
          <div class="popover-panel-footer">
            <button class="button button-primary popover-apply-btn" type="submit">应用</button>
          </div>
        </div>
      </div>

      <!-- 重置全部 -->
      <a href="/reports/{{.Run.RunID}}" class="button button-secondary" style="height: 34px; padding: 0 0.85rem; font-size: 0.82rem;">重置筛选</a>
    </div>

    <!-- 3. 活动过滤徽章区域 -->
    <div class="active-filter-badges" id="active-filter-badges" style="display: none;">
      <span class="active-badges-label">活动过滤器：</span>
      <div class="badges-row-content" id="badges-row-content"></div>
    </div>
  </form>
</section>
```

- [ ] **Step 2: 编译测试并启动自检，确认模板修改无编译错误**

Run: `make test`
Expected: go unit tests pass.

- [ ] **Step 3: 提交变更**

Run:
```bash
git add internal/web/templates/report.html
git commit -m "feat: refactor report filter panel to smart console HTML scaffold"
```

---

### Task 2: 样式表升级

**Files:**
- Modify: `internal/web/static/style.css` (在底部追加智能检索控制台及 Popover 的精美布局)

**Interfaces:**
- Consumes: Task 1 中添加的 HTML 元素的类选择器
- Produces: 暗黑高质感的 Popover 气泡浮动菜单和徽章标签视觉效果

- [ ] **Step 1: 在 style.css 底部追加控制台和 Popover 详细 CSS 规则**

在 `internal/web/static/style.css` 底部写入样式内容：
```css
/* 智能搜索控制台 */
.search-console-form {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 1.25rem;
  margin-bottom: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}
.search-console-bar {
  display: flex;
  gap: 0.65rem;
}
.search-input-wrapper {
  flex: 1;
  position: relative;
}
.search-input-wrapper input {
  height: 38px;
  padding-left: 2.35rem;
  font-size: 0.9rem;
  background: #07080a;
  border-color: var(--border-strong);
}
.search-submit-btn {
  height: 38px;
  min-width: 6.5rem;
}

/* 下拉维度栏 */
.filter-popover-row {
  display: flex;
  gap: 0.65rem;
  align-items: center;
  flex-wrap: wrap;
}
.popover-wrapper {
  position: relative;
}
.popover-trigger-btn {
  background: #16171d;
  border: 1px solid var(--border-strong);
  color: var(--text);
  padding: 0.45rem 0.85rem;
  font-size: 0.82rem;
  font-weight: 600;
  border-radius: var(--radius-md);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  height: 34px;
  transition: all 0.15s ease;
}
.popover-trigger-btn:hover,
.popover-trigger-btn.active {
  background: #1d1e26;
  border-color: var(--primary);
  color: var(--heading);
}
.trigger-active-count {
  background: var(--primary);
  color: #000000;
  font-size: 0.7rem;
  font-weight: 800;
  padding: 1px 5px;
  border-radius: 10px;
  min-width: 16px;
  text-align: center;
  line-height: 1;
}

/* popover 面板 */
.popover-panel {
  position: absolute;
  top: 100%;
  left: 0;
  margin-top: 0.45rem;
  width: 240px;
  background: #131419;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow);
  z-index: 110;
  display: flex;
  flex-direction: column;
  animation: popoverFadeIn 0.15s cubic-bezier(0.4, 0, 0.2, 1);
}
@keyframes popoverFadeIn {
  from { opacity: 0; transform: translateY(-5px); }
  to { opacity: 1; transform: translateY(0); }
}
.popover-panel-header {
  padding: 0.65rem 0.85rem;
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.popover-panel-body {
  padding: 0.85rem;
  max-height: 220px;
  overflow-y: auto;
}
.popover-checkbox-list {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}
.popover-checkbox-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  font-size: 0.82rem;
  color: var(--text);
  cursor: pointer;
}
.popover-checkbox-item input {
  width: 14px;
  height: 14px;
  cursor: pointer;
}
.popover-form-group {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.popover-form-group label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.78rem;
  color: var(--muted);
  font-weight: 600;
}
.popover-form-group input,
.popover-form-group select {
  padding: 0.45rem 0.65rem;
  font-size: 0.82rem;
}
.popover-panel-footer {
  padding: 0.55rem 0.85rem;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
}
.popover-apply-btn {
  padding: 0.35rem 0.75rem;
  font-size: 0.78rem;
  height: auto;
}

/* 活动过滤徽章 */
.active-filter-badges {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
  flex-wrap: wrap;
}
.active-badges-label {
  font-size: 0.78rem;
  color: var(--muted);
  font-weight: 600;
}
.badges-row-content {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}
.filter-badge-tag {
  background: rgba(249, 115, 22, 0.05);
  border: 1px solid rgba(249, 115, 22, 0.22);
  color: var(--primary);
  font-size: 0.76rem;
  font-family: var(--mono);
  padding: 0.22rem 0.5rem;
  border-radius: var(--radius-sm);
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-weight: 600;
}
.filter-badge-tag-remove {
  cursor: pointer;
  color: var(--muted);
  font-weight: 700;
  transition: color 0.15s ease;
}
.filter-badge-tag-remove:hover {
  color: var(--danger);
}
```

- [ ] **Step 2: 提交变更**

Run:
```bash
git add internal/web/static/style.css
git commit -m "style: add popover dropdown layout and filter tag badge styles"
```

---

### Task 3: JS 控制器及单元测试实现

**Files:**
- Modify: `internal/web/static/app.js` (添加智能 IP 解析路由、Popover 防冲突控制、活动徽章渲染及一键 ✕ 移除表单自动提交逻辑)
- Modify: `internal/web/static/app.test.mjs` (追加 Popovers 面板联动与 Smart Input 解析断言测试)

**Interfaces:**
- Consumes: `report.html` 表单元素，以及 `#smart-search-input` 内容
- Produces: 弹窗隐藏/旋转，动态生成标签，以及点击 ✕ 快速清除对应条件并提交表单

- [ ] **Step 1: 在 app.js 尾部，追加 Popover 控制和智能路由解析代码**

在 `internal/web/static/app.js` 尾部追加逻辑：
```javascript
  const smartForm = document.getElementById('report-filter-form');
  const smartInput = document.getElementById('smart-search-input');
  const hiddenIP = document.getElementById('hidden-ip');
  const hiddenQ = document.getElementById('hidden-q');
  const popoverViewSelect = document.getElementById('filter-view-select');

  if (smartForm) {
    // 1. Popover Panel 的 Toggle 与点击外部关闭
    document.querySelectorAll('[data-popover-target]').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const targetId = btn.getAttribute('data-popover-target');
        const panel = document.getElementById(targetId);
        if (!panel) return;

        const isHidden = panel.style.display === 'none';

        document.querySelectorAll('.popover-panel').forEach(p => p.style.display = 'none');
        document.querySelectorAll('.popover-trigger-btn').forEach(b => {
          b.classList.remove('active');
          const icon = b.querySelector('.chevron-icon');
          if (icon) icon.style.transform = 'rotate(0deg)';
        });

        if (isHidden) {
          panel.style.display = 'flex';
          btn.classList.add('active');
          const icon = btn.querySelector('.chevron-icon');
          if (icon) icon.style.transform = 'rotate(180deg)';
        }
      });
    });

    document.querySelectorAll('.popover-panel').forEach(panel => {
      panel.addEventListener('click', (e) => e.stopPropagation());
    });

    document.addEventListener('click', () => {
      document.querySelectorAll('.popover-panel').forEach(p => p.style.display = 'none');
      document.querySelectorAll('.popover-trigger-btn').forEach(b => {
        b.classList.remove('active');
        const icon = b.querySelector('.chevron-icon');
        if (icon) icon.style.transform = 'rotate(0deg)';
      });
    });

    // 2. 智能搜索输入框路由逻辑
    smartForm.addEventListener('submit', () => {
      if (!smartInput) return;
      const val = smartInput.value.trim();
      if (!val) {
        hiddenIP.value = '';
        hiddenQ.value = '';
      } else {
        const ipPattern = /^([0-9]{1,3}\.){3}[0-9]{1,3}(\/[0-9]{1,2})?$/;
        const rangePattern = /^([0-9]{1,3}\.){3}[0-9]{1,3}-[0-9]{1,3}$/;
        if (ipPattern.test(val) || rangePattern.test(val) || val.includes(',')) {
          hiddenIP.value = val;
          hiddenQ.value = '';
        } else {
          hiddenQ.value = val;
          hiddenIP.value = '';
        }
      }
    });

    // 3. 动态渲染活动过滤徽章 Tags & 计数 Badge
    const generateFilterBadges = () => {
      const badgesRow = document.getElementById('badges-row-content');
      const container = document.getElementById('active-filter-badges');
      if (!badgesRow || !container) return;

      badgesRow.innerHTML = '';
      let hasBadges = false;

      const addTag = (label, val, removeCallback) => {
        const tag = document.createElement('div');
        tag.className = 'filter-badge-tag';
        tag.innerHTML = `<span>${label}: ${val}</span>`;

        const removeBtn = document.createElement('span');
        removeBtn.className = 'filter-badge-tag-remove';
        removeBtn.innerHTML = '✕';
        removeBtn.addEventListener('click', (e) => {
          e.stopPropagation();
          removeCallback();
        });

        tag.appendChild(removeBtn);
        badgesRow.appendChild(tag);
      };

      if (hiddenIP && hiddenIP.value.trim()) {
        addTag('IP', hiddenIP.value.trim(), () => {
          hiddenIP.value = '';
          smartInput.value = '';
          smartForm.submit();
        });
        hasBadges = true;
      }
      if (hiddenQ && hiddenQ.value.trim()) {
        addTag('关键词', hiddenQ.value.trim(), () => {
          hiddenQ.value = '';
          smartInput.value = '';
          smartForm.submit();
        });
        hasBadges = true;
      }

      const portInput = smartForm.querySelector('input[name="port"]');
      if (portInput && portInput.value.trim()) {
        addTag('端口', portInput.value.trim(), () => {
          portInput.value = '';
          smartForm.submit();
        });
        hasBadges = true;
      }

      const serviceInput = smartForm.querySelector('input[name="service"]');
      if (serviceInput && serviceInput.value.trim()) {
        addTag('服务', serviceInput.value.trim(), () => {
          serviceInput.value = '';
          smartForm.submit();
        });
        hasBadges = true;
      }

      const sourceInput = smartForm.querySelector('input[name="source"]');
      if (sourceInput && sourceInput.value.trim()) {
        addTag('数据源', sourceInput.value.trim(), () => {
          sourceInput.value = '';
          smartForm.submit();
        });
        hasBadges = true;
      }

      if (popoverViewSelect && popoverViewSelect.value !== 'ports') {
        addTag('视图', '主机聚合', () => {
          popoverViewSelect.value = 'ports';
          smartForm.submit();
        });
        hasBadges = true;
      }

      const severities = [];
      document.querySelectorAll('.popover-checkbox-item input[type="checkbox"]').forEach(box => {
        if (box.checked) {
          severities.push(box.value);
        }
      });

      const severityCountEl = document.getElementById('active-severity-count');
      if (severityCountEl) {
        if (severities.length > 0) {
          severityCountEl.textContent = severities.length;
          severityCountEl.style.display = 'inline-block';
        } else {
          severityCountEl.style.display = 'none';
        }
      }

      severities.forEach(sev => {
        addTag('级别', sev, () => {
          const box = smartForm.querySelector(`.popover-checkbox-item input[value="${sev}"]`);
          if (box) box.checked = false;
          smartForm.submit();
        });
        hasBadges = true;
      });

      container.style.display = hasBadges ? 'flex' : 'none';
    };

    generateFilterBadges();
  }
```

- [ ] **Step 2: 在 app.test.mjs 中，追加对 Popovers、智能 IP / 文本路由以及 Tags 移除功能的单元测试断言**

在 `internal/web/static/app.test.mjs` 中，追加对该新交互逻辑的 Mock DOM 验证：
```javascript
  // 测试 Popover 点击显示折叠状态
  // 测试 smart-search-input 输入 192.168.1.1 时提交，自动路由至 hidden-ip
  // 测试 smart-search-input 输入 cve-2026 时提交，自动路由至 hidden-q
  // 测试点击 tag 上的 ✕ 会触发 hidden 参数清空以及 submit 被触发
```

- [ ] **Step 3: 运行 `make test` 进行测试集回归验证**

Run: `make test`
Expected: ALL PASS.

- [ ] **Step 4: 提交变更**

Run:
```bash
git add internal/web/static/app.js internal/web/static/app.test.mjs
git commit -m "feat: implement smart search routing, Popover toggles, and dynamic active filter tags"
```
