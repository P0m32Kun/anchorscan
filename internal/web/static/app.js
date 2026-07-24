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

// Scroll-spy: highlight the anchor link matching the section in view.
// Used by the settings anchor nav and the report floating outline.
function initScrollSpy(nav) {
  const links = Array.from(nav.querySelectorAll('a[href^="#"]'));
  if (links.length === 0 || !('IntersectionObserver' in window)) return;
  const pairs = links
    .map(link => ({ link, section: document.getElementById(link.hash.slice(1)) }))
    .filter(pair => pair.section);
  if (pairs.length === 0) return;
  const visible = new Set();
  const activate = () => {
    let current = pairs[0];
    const visiblePairs = pairs.filter(p => visible.has(p.section.id));
    if (visiblePairs.length > 0) {
      current = visiblePairs[0];
      for (let i = 1; i < visiblePairs.length; i++) {
        const candidate = visiblePairs[i];
        if (current.section.contains(candidate.section)) {
          current = candidate;
        }
      }
    } else {
      for (const pair of pairs) {
        if (pair.section.getBoundingClientRect().top < window.innerHeight * 0.5) current = pair;
      }
    }
    pairs.forEach(pair => pair.link.classList.toggle('active', pair === current));
  };
  const observer = new IntersectionObserver(entries => {
    entries.forEach(entry => {
      if (entry.isIntersecting) visible.add(entry.target.id);
      else visible.delete(entry.target.id);
    });
    activate();
  }, { rootMargin: '-20% 0px -60% 0px' });
  pairs.forEach(pair => observer.observe(pair.section));
  activate();
}

// Zone Tabs: filter project run tables by zone. Toggle buttons (aria-pressed),
// default "all" keeps every table visible.
function initZoneTabs() {
  const tabbar = document.querySelector('[data-zone-tabs]');
  if (!tabbar) return;
  const buttons = Array.from(tabbar.querySelectorAll('[data-zone-target]'));
  const groups = Array.from(document.querySelectorAll('.project-zone-runs[data-zone]'));
  if (buttons.length === 0 || groups.length === 0) return;
  tabbar.addEventListener('click', event => {
    const button = event.target.closest('[data-zone-target]');
    if (!button) return;
    const target = button.dataset.zoneTarget;
    buttons.forEach(btn => btn.setAttribute('aria-pressed', String(btn === button)));
    groups.forEach(group => {
      group.hidden = target !== 'all' && group.dataset.zone !== target;
    });
  });
}

function initAnchorNavs() {
  document.querySelectorAll('[data-scroll-spy]').forEach(initScrollSpy);
  initZoneTabs();
}

document.addEventListener('DOMContentLoaded', () => {
  setActiveNavigation(window.location.pathname);
  initAnchorNavs();
});

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
