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

setupToolForm();
