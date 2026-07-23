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
    updateProgress(events);
    const selection = window.getSelection?.();
    if(selection && !selection.isCollapsed && (box.contains(selection.anchorNode) || box.contains(selection.focusNode))) return;
    const threshold = 50;
    const isNearBottom = box.scrollHeight - box.clientHeight - box.scrollTop < threshold;
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
      
      const safeMsg = ansiHTML(e.message);
       
      const safeStage = String(e.stage || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
      return `<span class="event-time">${formatEventTime(e.time)}</span> <span class="event-stage">[${safeStage}]</span> <span class="${cls}">${safeMsg}</span>`;
    });
    box.innerHTML = lines.join('\n') + '\n<span class="terminal-cursor">█</span>';
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
  const res = await fetch('/api/runs/' + window.anchorRunID + '/status');
  if(!res.ok) return;
  const data = await res.json();
  const summary = document.getElementById('detection-check-summary');
  if(summary && data.detection_checks){
    const checks = data.detection_checks;
    summary.textContent = `检测检查：运行 ${checks.running || 0} · 完成 ${checks.completed || 0} · 跳过 ${checks.skipped || 0} · 失败 ${checks.failed || 0} · 已取消 ${checks.canceled || 0} · 已中断 ${checks.interrupted || 0}`;
  }
  if(current === 'running' && (data.status || '').toLowerCase() !== 'running') location.reload();
}
setInterval(refreshRunStatus, 1500);
refreshRunStatus();

// updateProgress parses the per-target "progress" events emitted by the scan
// engine and renders an IP-dimension progress bar: how many hosts are done out
// of the total alive set. This replaces the old fixed 5-stage cascade stepper,
// which was meaningless because every IP walks the full pipeline sequentially —
// only the final report is generated once for all hosts.
function updateProgress(events) {
  const bar = document.getElementById('scan-progress-bar');
  const count = document.getElementById('scan-progress-count');
  const detail = document.getElementById('scan-progress-detail');
  if (!bar || !count) return;

  // Find the last progress event to get the latest done/total counts.
  let latest = null;
  for (const e of events) {
    if ((e.stage || '').toLowerCase() === 'progress') {
      latest = e;
    }
  }

  const statusText = (document.getElementById('run-status')?.textContent || window.anchorRunStatus || '').trim().toLowerCase();

  if (!latest) {
    count.textContent = '等待存活探测…';
    bar.style.width = '0%';
    if (detail) detail.textContent = '';
    return;
  }

  const msg = latest.message || '';
  const totalMatch = msg.match(/(\d+)\/(\d+)/);
  const doneMatch = msg.match(/done=(\d+)/);
  const failedMatch = msg.match(/failed=(\d+)/);
  const currentMatch = msg.match(/current=(\S+)/);

  const done = doneMatch ? parseInt(doneMatch[1], 10) : 0;
  const total = totalMatch ? parseInt(totalMatch[2], 10) : 0;
  const failed = failedMatch ? parseInt(failedMatch[1], 10) : 0;
  const current = currentMatch ? currentMatch[1] : '';

  if (total > 0) {
    const pct = Math.min(100, Math.round((done / total) * 100));
    bar.style.width = pct + '%';
    count.textContent = `已完成 ${done} / ${total} 个主机` + (failed > 0 ? `（失败 ${failed}）` : '');
    if (detail) {
      if (statusText === 'completed') {
        detail.textContent = failed > 0 ? `扫描结束，${failed} 个主机失败` : '扫描完成';
      } else if (statusText === 'failed' || statusText === 'canceled') {
        detail.textContent = statusText === 'canceled' ? '扫描已取消' : '扫描失败';
      } else if (current) {
        detail.textContent = `正在扫描 ${current}`;
      } else {
        detail.textContent = '';
      }
    }
  }
}
