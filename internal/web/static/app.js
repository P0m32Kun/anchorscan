function setupToolForm(){
  const form = document.querySelector('[data-tool-form]');
  const terminal = document.getElementById('tool-output');
  if(!form || !terminal) return;
  const button = form.querySelector('button[type="submit"]');

  const write = (text) => {
    terminal.textContent = text;
    terminal.scrollTop = terminal.scrollHeight;
  };

  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    if(button) button.disabled = true;
    const raw = (form.elements.raw_args?.value || '').trim();
    write('$ ' + form.dataset.tool + (raw ? ' ' + raw : '') + '\n启动中...\n');
    try {
      const res = await fetch(form.action, {
        method: 'POST',
        body: new URLSearchParams(new FormData(form)),
        headers: {'X-Requested-With': 'fetch'},
      });
      if(!res.ok) throw new Error((await res.text()).trim() || '启动失败');
      const data = await res.json();
      await pollToolOutput(data.run_id, write);
    } catch (err) {
      write(terminal.textContent + (err.message || String(err)) + '\n');
    } finally {
      if(button) button.disabled = false;
    }
  });
}

async function pollToolOutput(runID, write){
  for(;;){
    const eventsRes = await fetch('/api/runs/' + runID + '/events');
    if(eventsRes.ok){
      const events = await eventsRes.json();
      const lines = events.filter(e => e.stage !== 'report').map(e => e.message);
      if(lines.length) write(lines.join('\n') + '\n');
    }
    const statusRes = await fetch('/api/runs/' + runID + '/status');
    if(statusRes.ok){
      const data = await statusRes.json();
      if((data.status || '').toLowerCase() !== 'running') return;
    }
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
}

async function copyReportData(button){
  let text = button.dataset.copyText || '';
  if(button.dataset.copyUrl){
    const res = await fetch(button.dataset.copyUrl);
    if(!res.ok) throw new Error('copy fetch failed');
    text = await res.text();
  }
  await writeClipboard(text.trimEnd());
}

async function writeClipboard(text){
  if(navigator.clipboard && window.isSecureContext){
    await navigator.clipboard.writeText(text);
    return;
  }
  const box = document.createElement('textarea');
  box.value = text;
  box.style.position = 'fixed';
  box.style.left = '-9999px';
  document.body.appendChild(box);
  box.focus();
  box.select();
  const ok = document.execCommand('copy');
  document.body.removeChild(box);
  if(!ok) throw new Error('copy failed');
}

document.addEventListener('click', async (event) => {
  const preset = event.target.closest('.preset-chip');
  if(preset){
    const form = document.querySelector('[data-tool-form]');
    if(!form) return;
    if(form.elements.raw_args) form.elements.raw_args.value = preset.dataset.setRawArgs || '';
    form.dispatchEvent(new Event('change', {bubbles: true}));
    return;
  }

  const insertBtn = event.target.closest('[data-insert-ports]');
  if(insertBtn){
    const targetName = insertBtn.dataset.insertTarget;
    const input = document.querySelector(`[name="${targetName}"]`);
    if(input){
      const value = insertBtn.dataset.insertPorts || '';
      if(insertBtn.dataset.insertMode === 'append'){
        const current = input.value.trim();
        input.value = current ? current + ' ' + value : value;
      } else {
        input.value = value;
      }
      input.dispatchEvent(new Event('change', {bubbles: true}));
    }
    return;
  }

  const button = event.target.closest('[data-copy-url],[data-copy-text]');
  if(!button) return;
  const original = button.textContent;
  button.disabled = true;
  try {
    await copyReportData(button);
    button.textContent = '已复制';
  } catch (err) {
    button.textContent = '复制失败';
  }
  setTimeout(() => {
    button.disabled = false;
    button.textContent = original;
  }, 1200);
});

setupToolForm();

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

  // 1. 高级过滤选项折叠/展开
  const toggleAdvBtn = document.getElementById('btn-toggle-advanced');
  const advPanel = document.getElementById('advanced-filter-panel');
  if (toggleAdvBtn && advPanel) {
    toggleAdvBtn.addEventListener('click', (e) => {
      e.preventDefault();
      const isHidden = advPanel.style.display === 'none';
      advPanel.style.display = isHidden ? 'block' : 'none';
      const chevron = toggleAdvBtn.querySelector('.chevron-icon');
      if (chevron) {
        chevron.style.transform = isHidden ? 'rotate(180deg)' : 'rotate(0deg)';
      }
    });
  }

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
});

// Popover 控制和智能路由解析代码
(() => {
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
        
        const isHidden = window.getComputedStyle(panel).display === 'none';
        
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
        if (hiddenIP) hiddenIP.value = '';
        if (hiddenQ) hiddenQ.value = '';
      } else {
        const ipPattern = /^([0-9]{1,3}\.){3}[0-9]{1,3}(\/[0-9]{1,2})?$/;
        const rangePattern = /^([0-9]{1,3}\.){3}[0-9]{1,3}-[0-9]{1,3}$/;
        if (ipPattern.test(val) || rangePattern.test(val) || val.includes(',')) {
          if (hiddenIP) hiddenIP.value = val;
          if (hiddenQ) hiddenQ.value = '';
        } else {
          if (hiddenQ) hiddenQ.value = val;
          if (hiddenIP) hiddenIP.value = '';
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
        const textSpan = document.createElement('span');
        textSpan.textContent = `${label}: ${val}`;
        tag.appendChild(textSpan);
        
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
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
        });
        hasBadges = true;
      }
      if (hiddenQ && hiddenQ.value.trim()) {
        addTag('关键词', hiddenQ.value.trim(), () => {
          hiddenQ.value = '';
          smartInput.value = '';
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
        });
        hasBadges = true;
      }

      const portInput = smartForm.querySelector('input[name="port"]');
      if (portInput && portInput.value.trim()) {
        addTag('端口', portInput.value.trim(), () => {
          portInput.value = '';
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
        });
        hasBadges = true;
      }

      const serviceInput = smartForm.querySelector('input[name="service"]');
      if (serviceInput && serviceInput.value.trim()) {
        addTag('服务', serviceInput.value.trim(), () => {
          serviceInput.value = '';
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
        });
        hasBadges = true;
      }

      const sourceInput = smartForm.querySelector('input[name="source"]');
      if (sourceInput && sourceInput.value.trim()) {
        addTag('数据源', sourceInput.value.trim(), () => {
          sourceInput.value = '';
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
        });
        hasBadges = true;
      }

      if (popoverViewSelect && popoverViewSelect.value !== 'ports') {
        addTag('视图', '主机聚合', () => {
          popoverViewSelect.value = 'ports';
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
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
          if (typeof smartForm.requestSubmit === 'function') { smartForm.requestSubmit(); } else { smartForm.submit(); }
        });
        hasBadges = true;
      });

      container.style.display = hasBadges ? 'flex' : 'none';
    };

    generateFilterBadges();
  }
})();
