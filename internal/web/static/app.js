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

async function copyReportData(button){
  let text = button.dataset.copyText || '';
  if(button.dataset.copyUrl){
    const res = await fetch(button.dataset.copyUrl);
    if(!res.ok) throw new Error('copy fetch failed');
    text = await res.text();
  }
  await navigator.clipboard.writeText(text.trimEnd());
}

document.addEventListener('click', async (event) => {
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
