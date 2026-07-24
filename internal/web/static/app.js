function setActiveNavigation(path, items = document.querySelectorAll('.nav-item')) {
  const activeID = path === '/' || path === '' ? 'nav-home'
    : path.startsWith('/projects') ? 'nav-projects'
    : path.startsWith('/runs') || path.startsWith('/scan') || path.startsWith('/reports') ? 'nav-runs'
    : path.startsWith('/import') ? 'nav-import'
    : path.startsWith('/tools') ? 'nav-tools'
    : path.startsWith('/kb') ? 'nav-kb'
    : path.startsWith('/config') ? 'nav-config'
    : '';
  items.forEach(item => item.classList.toggle('active', item.id === activeID));
}

document.addEventListener('DOMContentLoaded', () => setActiveNavigation(window.location.pathname));

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
  if(!button || document.querySelector('[data-report-interactions][data-mounted="true"]')) return;
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
