const beijingTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
  hourCycle: 'h23',
});

function formatEventTime(value){
  const date = new Date(value);
  if(Number.isNaN(date.getTime())) return value || '';
  const parts = Object.fromEntries(beijingTimeFormatter.formatToParts(date).map(part => [part.type, part.value]));
  return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`;
}

async function refreshEvents(){
  if(!window.anchorRunID) return;
  const res = await fetch('/api/runs/' + window.anchorRunID + '/events');
  if(!res.ok) return;
  const events = await res.json();
  const box = document.getElementById('events');
  if(box){
    const lines = events.map(e => {
      let cls = 'log-info';
      const msg = e.message.toLowerCase();
      const stage = e.stage.toLowerCase();
      
      if (msg.includes('error') || msg.includes('failed') || msg.includes('critical') || msg.includes('fatal')) {
        cls = 'log-error';
      } else if (msg.includes('warn') || msg.includes('warning') || msg.includes('timeout') || msg.includes('alert')) {
        cls = 'log-warn';
      } else if (stage === 'system' || stage === 'init' || msg.includes('start') || msg.includes('begin')) {
        cls = 'log-system';
      }
      
      // Escape HTML entities to prevent rendering issues or XSS
      const safeMsg = e.message
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
        
      return `<span style="color: #64748b;">${formatEventTime(e.time)}</span> <span style="color: #60a5fa; font-weight: 600;">[${e.stage}]</span> <span class="${cls}">${safeMsg}</span>`;
    });
    box.innerHTML = lines.join('\n') + '\n<span class="terminal-cursor">█</span>';
    
    // 触发 Stepper 更新
    const statusText = document.getElementById('run-status')?.textContent || window.anchorRunStatus || '';
    updateStepper(events, statusText);
    
    // Auto scroll if user is already near bottom
    const threshold = 50;
    const isNearBottom = box.scrollHeight - box.clientHeight - box.scrollTop < threshold;
    if (isNearBottom || box.scrollTop === 0) {
      box.scrollTop = box.scrollHeight;
    }
  }
}
setInterval(refreshEvents, 1000);
refreshEvents();

async function refreshRunStatus(){
  if(!window.anchorRunID) return;
  const current = (window.anchorRunStatus || document.getElementById('run-status')?.textContent || '').trim().toLowerCase();
  if(current !== 'running') return;
  const res = await fetch('/api/runs/' + window.anchorRunID + '/status');
  if(!res.ok) return;
  const data = await res.json();
  if((data.status || '').toLowerCase() !== 'running') location.reload();
}
setInterval(refreshRunStatus, 1500);
refreshRunStatus();

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
      if (!s) return;
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
