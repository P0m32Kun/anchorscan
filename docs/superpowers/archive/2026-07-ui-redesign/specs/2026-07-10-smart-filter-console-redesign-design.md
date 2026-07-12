# 设计规格说明书: 智能搜索控制台重设计 (2026-07-10)

本设计规格说明书针对 AnchorScan 漏洞与资产检索模块进行二阶段深度体验重设计。主要替换上一阶段中散乱且频繁刷新页面的过滤表单，引入现代安全 SaaS 控制台风格的**智能搜索控制台（Smart Search Console）**，实现组件美感一致、智能 IP 识别和 Popovers 防刷新多维筛选。

---

## 1. 重设计痛点与目标

1. **交互逻辑混乱**：原设计中，部分元素（如 Severity Checkbox、View Select）在点击/改变时会自动刷新页面，而输入框需要点击“开始过滤”。这导致用户体验割裂。
2. **布局杂乱无一致性**：多种布局（Flex 与 Grid）在同一个过滤卡片中混排，在视觉层面没有统一的对齐逻辑。
3. **优化方案 (方案 B)**：
   * **单一智能检索栏**：主搜索框根据输入值格式自动判定是 IP/网段查找还是漏洞/关键字文本查找，自动分流。
   * **Popover 维度过滤器组**：将危险等级、端口服务、数据源、视图模式收纳至浮动的 Popover 面板中。仅在点击应用或点击外部关闭时触发表单操作，彻底消除散乱感。
   * **活动过滤徽章 (Filter Badges)**：在搜索框下方动态绘制正在起效的筛选标签，点击标签上的 `✕` 可一键清除并重载。

---

## 2. 影响范围

修改仅涉及 3 个前端文件（均位于 `internal/web/` 目录下）：
1. [internal/web/static/style.css](file:///Users/kun/DEV/new-Anchor/internal/web/static/style.css) (添加 Popover、Filter Tag、智能搜索框的布局与过渡样式)
2. [internal/web/static/app.js](file:///Users/kun/DEV/new-Anchor/internal/web/static/app.js) (Popover Toggle 监听、智能 IP/Q 分配路由、Filter Badges 提取与自动清除逻辑)
3. [internal/web/templates/report.html](file:///Users/kun/DEV/new-Anchor/internal/web/templates/report.html) (完全重构过滤器表单结构)

---

## 3. 具体实施规格与代码细节

### 3.1 报告页过滤表单重构 (`report.html`)
完全替换 `report.html` 中的数据过滤表单容器：

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

---

### 3.2 样式扩展 (`style.css`)
在 CSS 文件尾部追加新的控制台与浮窗样式，同时**保留原有表单的基本选择器兼容**，防止产生破位冲突：

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

/* 过滤标记 */
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

---

### 3.3 交互逻辑扩展 (`app.js`)
在 `internal/web/static/app.js` 的 `DOMContentLoaded` 中追加 Popovers 触发控制、智能搜索路由、徽章渲染等逻辑：

```javascript
  // 智能搜索表单元素
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

        // 关闭所有已打开的面板
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

    // 面板内防止冒泡
    document.querySelectorAll('.popover-panel').forEach(panel => {
      panel.addEventListener('click', (e) => e.stopPropagation());
    });

    // 全局点击关闭 Popover
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

      // A. IP 与 关键词
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

      // B. 端口与服务
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

      // C. 数据源
      const sourceInput = smartForm.querySelector('input[name="source"]');
      if (sourceInput && sourceInput.value.trim()) {
        addTag('数据源', sourceInput.value.trim(), () => {
          sourceInput.value = '';
          smartForm.submit();
        });
        hasBadges = true;
      }

      // D. 视图模式
      if (popoverViewSelect && popoverViewSelect.value !== 'ports') {
        addTag('视图', '主机聚合', () => {
          popoverViewSelect.value = 'ports';
          smartForm.submit();
        });
        hasBadges = true;
      }

      // E. 危险等级计数
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

---

## 4. 验证与单元测试扩展
* **前端 DOM 状态校验**：
  在 `app.test.mjs` 中追加对智能检测（IPv4/关键词划分规则）的测试断言，以及对 Popover 面板显示隐藏与 Badge 清除回调行为的单元测试验证，确保功能闭环。
