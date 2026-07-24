function imagesFromClipboardData(items){
  const files = [];
  if(!items) return files;
  for(const item of items){
    if(item.type?.startsWith('image/')){
      const file = item.getAsFile();
      if(file) files.push(file);
    }
  }
  return files;
}

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

function queueNameFromHash(hash){
  const name = String(hash || '').replace(/^#/, '');
  return ['positive', 'negative', 'incomplete'].includes(name) ? name : 'positive';
}

const ansiColors = [
  '#000000', '#cd3131', '#0dbc79', '#e5e510', '#2472c8', '#bc3fbc', '#11a8cd', '#e5e5e5',
  '#666666', '#f14c4c', '#23d18b', '#f5f543', '#3b8eea', '#d670d6', '#29b8db', '#ffffff',
];

function ansi256Color(value){
  const n = Math.max(0, Math.min(255, Number(value) || 0));
  if(n < 16) return ansiColors[n];
  if(n < 232){
    const cube = n - 16;
    const levels = [0, 95, 135, 175, 215, 255];
    return `rgb(${levels[Math.floor(cube / 36)]}, ${levels[Math.floor(cube / 6) % 6]}, ${levels[cube % 6]})`;
  }
  const gray = 8 + (n - 232) * 10;
  return `rgb(${gray}, ${gray}, ${gray})`;
}

function ansiSegments(text){
  const segments = [];
  const state = {color: '', background: '', bold: false, italic: false, underline: false};
  const pattern = /\x1b\[([0-9;]*)m|\[([0-9;]+)m/g;
  let cursor = 0;
  let match;
  const push = value => {
    if(value) segments.push({text: value, ...state});
  };
  while((match = pattern.exec(String(text))) !== null){
    push(String(text).slice(cursor, match.index));
    const codes = (match[1] ?? match[2] ?? '0').split(';').map(Number);
    for(let i = 0; i < codes.length; i++){
      const code = codes[i];
      if(code === 0) Object.assign(state, {color: '', background: '', bold: false, italic: false, underline: false});
      else if(code === 1) state.bold = true;
      else if(code === 3) state.italic = true;
      else if(code === 4) state.underline = true;
      else if(code === 22) state.bold = false;
      else if(code === 23) state.italic = false;
      else if(code === 24) state.underline = false;
      else if(code >= 30 && code <= 37) state.color = ansiColors[code - 30];
      else if(code >= 90 && code <= 97) state.color = ansiColors[code - 90 + 8];
      else if(code === 39) state.color = '';
      else if(code >= 40 && code <= 47) state.background = ansiColors[code - 40];
      else if(code >= 100 && code <= 107) state.background = ansiColors[code - 100 + 8];
      else if(code === 49) state.background = '';
      else if((code === 38 || code === 48) && codes[i + 1] === 5){
        state[code === 38 ? 'color' : 'background'] = ansi256Color(codes[i + 2]);
        i += 2;
      } else if((code === 38 || code === 48) && codes[i + 1] === 2){
        state[code === 38 ? 'color' : 'background'] = `rgb(${codes[i + 2]}, ${codes[i + 3]}, ${codes[i + 4]})`;
        i += 4;
      }
    }
    cursor = pattern.lastIndex;
  }
  push(String(text).slice(cursor));
  return segments;
}

function renderANSI(element, text){
  const fragment = document.createDocumentFragment();
  ansiSegments(text).forEach(segment => {
    const span = document.createElement('span');
    span.textContent = segment.text;
    if(segment.color) span.style.color = segment.color;
    if(segment.background) span.style.backgroundColor = segment.background;
    if(segment.bold) span.style.fontWeight = '700';
    if(segment.italic) span.style.fontStyle = 'italic';
    if(segment.underline) span.style.textDecoration = 'underline';
    fragment.appendChild(span);
  });
  element.replaceChildren(fragment);
}

function ansiHTML(text){
  return ansiSegments(text).map(segment => {
    const safe = segment.text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
    const styles = [];
    if(segment.color) styles.push('color:' + segment.color);
    if(segment.background) styles.push('background-color:' + segment.background);
    if(segment.bold) styles.push('font-weight:700');
    if(segment.italic) styles.push('font-style:italic');
    if(segment.underline) styles.push('text-decoration:underline');
    return styles.length ? `<span style="${styles.join(';')}">${safe}</span>` : safe;
  }).join('');
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
