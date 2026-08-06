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
//
// 激活判定完全基于 rect 几何，分两段（不再使用 IntersectionObserver 的可见集合
// 顺序，避免 1px 脆弱性）：
//   1) 优先：完整位于视口内的 section 中，选最靠上的（top 最小）。这覆盖页面
//      顶部（大纲首项默认 active——刚打开页面看到的就是顶部区块）以及目标
//      section 被完整滚入视口的情形（scrollIntoView({block:'center'}) 之后，
//      若目标比视口矮则必然唯一完整在视口内，从而被选中）。
//   2) 兜底：所有相交 section 都跨越视口边界（典型：长表格/大面板）时，高亮
//      "垂直中心最接近视口中心"的 section（用户正在看的区块）；距离并列
//      （容差 1px）时优先更具体（高度更小）的，以纯几何方式覆盖
//      #config-timeouts 嵌套在 #config-engine 内的场景，不依赖 DOM 顺序或 contains。
//
// 旧实现用 IntersectionObserver 的可见集合顺序 + 固定观察带（rootMargin
// '-20% 0px -60% 0px'，仅覆盖视口 20%–40%），与 scrollIntoView(center) 把目标
// 滚到视口 50% 处不匹配；观察带的二值相交判定 + visiblePairs[0] 使结果对 1px
// 布局差异极度敏感（CI quality-gate 失败）。改用上述几何判定彻底消除该脆弱性。
function initScrollSpy(nav) {
  const links = Array.from(nav.querySelectorAll('a[href^="#"]'));
  if (links.length === 0) return;
  const pairs = links
    .map(link => ({ link, section: document.getElementById(link.hash.slice(1)) }))
    .filter(pair => pair.section);
  if (pairs.length === 0) return;

  const activate = () => {
    const viewportHeight = window.innerHeight;
    const viewportCenter = viewportHeight / 2;
    // 1) 完整位于视口内的 section：选最靠上的（top 最小）。
    let best = null;
    let bestTop = Infinity;
    for (const pair of pairs) {
      const rect = pair.section.getBoundingClientRect();
      if (rect.top >= 0 && rect.bottom <= viewportHeight && rect.top < bestTop) {
        best = pair;
        bestTop = rect.top;
      }
    }
    // 2) 兜底：所有相交 section 都跨越视口边界，选中心最接近视口中心的
    //    （并列时取更矮/更具体者）。
    if (!best) {
      let bestDist = Infinity;
      let bestHeight = Infinity;
      for (const pair of pairs) {
        const rect = pair.section.getBoundingClientRect();
        const center = (rect.top + rect.bottom) / 2;
        const dist = Math.abs(center - viewportCenter);
        const height = rect.height || 1;
        const clearlyCloser = dist < bestDist - 1;
        const tied = Math.abs(dist - bestDist) <= 1;
        const moreSpecific = tied && height < bestHeight;
        if (!best || clearlyCloser || moreSpecific) {
          best = pair;
          bestDist = dist;
          bestHeight = height;
        }
      }
    }
    pairs.forEach(pair => pair.link.classList.toggle('active', pair === best));
  };

  let scheduled = false;
  const update = () => {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(() => {
      scheduled = false;
      activate();
    });
  };

  activate();
  window.addEventListener('scroll', update, { passive: true });
  window.addEventListener('resize', update, { passive: true });
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
