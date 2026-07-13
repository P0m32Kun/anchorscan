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
