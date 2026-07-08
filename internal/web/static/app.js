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
        
      return `<span style="color: #64748b;">${e.time}</span> <span style="color: #60a5fa; font-weight: 600;">[${e.stage}]</span> <span class="${cls}">${safeMsg}</span>`;
    });
    box.innerHTML = lines.join('\n') + '\n<span class="terminal-cursor">█</span>';
    
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
