import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

for (const name of fs.readdirSync(new URL('../templates/', import.meta.url))) {
  const template = fs.readFileSync(new URL(`../templates/${name}`, import.meta.url), 'utf8');
  assert.equal(template.includes('/static/app.js'), name === 'base.html', `${name} must not load app.js`);
}

const source = fs.readFileSync(new URL('./app.js', import.meta.url), 'utf8');
const runStatusSource = fs.readFileSync(new URL('./run-status.js', import.meta.url), 'utf8');
const toolFormSource = fs.readFileSync(new URL('./tool-form.js', import.meta.url), 'utf8');
const reportUISource = fs.readFileSync(new URL('./report-ui.js', import.meta.url), 'utf8');
const domContentLoadedCallbacks = [];
const context = {
  window: {
    getComputedStyle: (el) => el.style || {}
  },
  document: {
    getElementById: () => null,
    querySelector: () => null,
    addEventListener: (event, callback) => {
      if (event === 'DOMContentLoaded') domContentLoadedCallbacks.push(callback);
    },
  },
  setInterval: () => {},
};
vm.createContext(context);
vm.runInContext(source, context);
vm.runInContext(runStatusSource, context);
vm.runInContext(reportUISource, context);
assert.equal(domContentLoadedCallbacks.length, 3);

const navItems = [
  { id: 'nav-home', classList: { active: false, toggle(_, value) { this.active = value; } } },
  { id: 'nav-projects', classList: { active: false, toggle(_, value) { this.active = value; } } },
  { id: 'nav-runs', classList: { active: false, toggle(_, value) { this.active = value; } } },
];
context.setActiveNavigation('/projects/demo', navItems);
assert.equal(navItems[0].classList.active, false);
assert.equal(navItems[1].classList.active, true);
assert.equal(navItems[2].classList.active, false);

assert.equal(
  context.formatEventTime('2026-07-09T03:16:55.614Z'),
  '2026-07-09 11:16:55',
);

const mockEventBox = { innerHTML: '', scrollTop: 0, scrollHeight: 100, clientHeight: 100 };
context.document.getElementById = (id) => id === 'events' ? mockEventBox : null;
context.window.anchorRunID = 'run-test';
context.fetch = async () => ({
  ok: true,
  json: async () => [{ time: '2026-07-09T03:16:55.614Z', stage: '<img>', message: 'ok' }],
});
await context.refreshEvents();
assert.ok(mockEventBox.innerHTML.includes('[&lt;img&gt;]'));

let toolSubmitHandler;
const toolContext = {
  window: {},
  document: {
    getElementById: () => null,
    querySelector: () => null,
    addEventListener: () => {},
  },
  setInterval: () => {},
};
vm.createContext(toolContext);
vm.runInContext(source, toolContext);
toolContext.document.getElementById = (id) => id === 'tool-output' ? { textContent: '', scrollTop: 0, scrollHeight: 0 } : null;
toolContext.document.querySelector = (selector) => selector === '[data-tool-form]' ? {
  querySelector: () => null,
  addEventListener(event, handler) {
    if (event === 'submit') toolSubmitHandler = handler;
  },
} : null;
vm.runInContext(toolFormSource, toolContext);
assert.equal(typeof toolContext.setupToolForm, 'function');
assert.equal(typeof toolSubmitHandler, 'function');

// Unit test for renderVulnDistribution
let containerDisplay = 'none';
let barHTML = '';
let legendHTML = '';

const mockBadges = [
  { textContent: 'critical' },
  { textContent: 'high' },
  { textContent: 'high' },
  { textContent: 'medium' },
  { textContent: 'low' },
  { textContent: 'info' }
];

const mockContainer = {
  style: {
    get display() { return containerDisplay; },
    set display(val) { containerDisplay = val; }
  }
};
const mockBar = {
  set innerHTML(val) { barHTML = val; }
};
const mockLegend = {
  set innerHTML(val) { legendHTML = val; }
};

context.document.getElementById = (id) => {
  if (id === 'distribution-container') return mockContainer;
  if (id === 'distribution-bar') return mockBar;
  if (id === 'distribution-legend') return mockLegend;
  return null;
};

context.document.querySelectorAll = (selector) => {
  if (selector === '.severity-badge') return mockBadges;
  return [];
};

// Run the report page initializer registered by report-ui.js.
domContentLoadedCallbacks.at(-1)();

assert.equal(containerDisplay, 'block');
assert.ok(barHTML.includes('vuln-bar-segment critical'));
assert.ok(barHTML.includes('width: 16.7%'));
assert.ok(barHTML.includes('width: 33.3%'));
assert.ok(legendHTML.includes('严重 (Critical)'));
assert.ok(legendHTML.includes('legend-count">1</span>'));
assert.ok(legendHTML.includes('legend-count">2</span>'));

// Test case when badges are empty
containerDisplay = 'block';
context.document.querySelectorAll = (selector) => [];
context.renderVulnDistribution();
assert.equal(containerDisplay, 'none');

// Test suite for updateProgress (IP-dimension progress bar)
const mockProgressBar = { style: { width: '0%' } };
const mockProgressCount = { textContent: '' };
const mockProgressDetail = { textContent: '' };
const mockRunStatus = { textContent: 'running' };

function resetMockProgress() {
  mockProgressBar.style.width = '0%';
  mockProgressCount.textContent = '';
  mockProgressDetail.textContent = '';
}

context.document.getElementById = (id) => {
  if (id === 'scan-progress-bar') return mockProgressBar;
  if (id === 'scan-progress-count') return mockProgressCount;
  if (id === 'scan-progress-detail') return mockProgressDetail;
  if (id === 'run-status') return mockRunStatus;
  return null;
};

// Test Case 1: no progress events yet → shows waiting state
resetMockProgress();
context.updateProgress([]);
assert.equal(mockProgressCount.textContent, '等待存活探测…');
assert.equal(mockProgressBar.style.width, '0%');

// Test Case 2: initial total announced, none done
resetMockProgress();
context.updateProgress([
  { stage: 'progress', message: 'progress 0/10 done=0 failed=0' }
]);
assert.ok(mockProgressCount.textContent.includes('0 / 10'));
assert.equal(mockProgressBar.style.width, '0%');

// Test Case 3: 5 of 10 done → 50%
resetMockProgress();
context.updateProgress([
  { stage: 'progress', message: 'progress 0/10 done=0 failed=0' },
  { stage: 'progress', message: 'progress 5/10 done=5 failed=0 current=192.168.1.5' }
]);
assert.ok(mockProgressCount.textContent.includes('5 / 10'));
assert.equal(mockProgressBar.style.width, '50%');
assert.ok(mockProgressDetail.textContent.includes('192.168.1.5'));

// Test Case 4: failures are surfaced in the count
resetMockProgress();
context.updateProgress([
  { stage: 'progress', message: 'progress 3/10 done=3 failed=1 current=192.168.1.3' }
]);
assert.ok(mockProgressCount.textContent.includes('失败 1'));

// Test Case 5: completed status fills the bar
resetMockProgress();
mockRunStatus.textContent = 'completed';
context.updateProgress([
  { stage: 'progress', message: 'progress 10/10 done=10 failed=0' }
]);
assert.equal(mockProgressBar.style.width, '100%');
assert.ok(mockProgressDetail.textContent.includes('扫描完成'));
mockRunStatus.textContent = 'running';

// Test Case 6: only the latest progress event is used
resetMockProgress();
context.updateProgress([
  { stage: 'progress', message: 'progress 2/10 done=2 failed=0' },
  { stage: 'progress', message: 'progress 8/10 done=8 failed=0 current=10.0.0.8' },
  { stage: 'nmap', message: 'nmap alive hosts=[10.0.0.8]' }
]);
assert.ok(mockProgressCount.textContent.includes('8 / 10'));
assert.equal(mockProgressBar.style.width, '80%');

// Test DOMContentLoaded interactions (collapsible panel & auto-submit)
{
  let domContentLoadedCallback = null;
  const mockForm = {
    submitCalledCount: 0,
    requestSubmit() {
      this.submit();
    },
    submit() {
      this.submitCalledCount++;
    }
  };

  const mockChevron = {
    style: { transform: '' }
  };

  const mockToggleBtn = {
    listeners: {},
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    },
    querySelector(sel) {
      if (sel === '.chevron-icon') return mockChevron;
      return null;
    }
  };

  const mockAdvPanel = {
    style: { display: 'none' }
  };

  const mockViewSelect = {
    listeners: {},
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    },
    closest(sel) {
      if (sel === 'form') return mockForm;
      return null;
    }
  };

  const mockClassList = {
    classes: new Set(),
    toggle(cls, state) {
      if (state) {
        this.classes.add(cls);
      } else {
        this.classes.delete(cls);
      }
    }
  };

  const mockCheckbox = {
    checked: true,
    listeners: {},
    parentElement: {
      classList: mockClassList
    },
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    },
    closest(sel) {
      if (sel === 'form') return mockForm;
      return null;
    }
  };

  const interactiveContext = {
    window: {
      getComputedStyle: (el) => el.style || {}
    },
    document: {
      addEventListener(event, handler) {
        if (event === 'DOMContentLoaded') {
          domContentLoadedCallback = handler;
        }
      },
      getElementById(id) {
        if (id === 'btn-toggle-advanced') return mockToggleBtn;
        if (id === 'advanced-filter-panel') return mockAdvPanel;
        if (id === 'filter-view-select') return mockViewSelect;
        return null;
      },
      querySelectorAll(sel) {
        if (sel === '.severity-filter-chip input[type="checkbox"]') {
          return [mockCheckbox];
        }
        return [];
      },
      querySelector() {
        return null;
      }
    },
    setInterval() {},
  };

  vm.createContext(interactiveContext);
  vm.runInContext(source, interactiveContext);

  // Trigger DOMContentLoaded
  assert.ok(domContentLoadedCallback);
  domContentLoadedCallback();

  // 2. Verify collapsible panel toggle behavior
  const clickHandler = mockToggleBtn.listeners['click'];
  assert.ok(clickHandler);

  // Call click handler
  let preventDefaultCalled = false;
  clickHandler({
    preventDefault() {
      preventDefaultCalled = true;
    }
  });
  assert.ok(preventDefaultCalled);
  assert.equal(mockAdvPanel.style.display, 'block');
  assert.equal(mockChevron.style.transform, 'rotate(180deg)');

  // Toggle again
  clickHandler({ preventDefault() {} });
  assert.equal(mockAdvPanel.style.display, 'none');
  assert.equal(mockChevron.style.transform, 'rotate(0deg)');
}

// Test DOMContentLoaded - Evidence verification Copy Button (Success path)
{
  let domContentLoadedCallback = null;
  let writtenText = '';
  let writeClipboardCalled = false;

  const mockTarget = {
    innerText: 'evidence details content'
  };

  const mockBtn = {
    listeners: {},
    disabled: false,
    innerHTML: '<span>复制数据</span>',
    getAttribute(attr) {
      if (attr === 'data-copy-target-id') return 'evidence-pre-0';
      return null;
    },
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    }
  };

  let setTimeoutCallback = null;
  let setTimeoutDelay = 0;

  const copyContext = {
    window: {
      isSecureContext: true,
      getComputedStyle: (el) => el.style || {}
    },
    navigator: {
      clipboard: {
        writeText: async (text) => {
          writeClipboardCalled = true;
          writtenText = text;
        }
      }
    },
    document: {
      addEventListener(event, handler) {
        if (event === 'DOMContentLoaded') {
          domContentLoadedCallback = handler;
        }
      },
      getElementById(id) {
        if (id === 'evidence-pre-0') return mockTarget;
        return null;
      },
      querySelectorAll(sel) {
        if (sel === '[data-copy-target-id]') {
          return [mockBtn];
        }
        return [];
      },
      querySelector() {
        return null;
      }
    },
    setTimeout(handler, delay) {
      setTimeoutCallback = handler;
      setTimeoutDelay = delay;
    },
    setInterval: () => {}
  };

  vm.createContext(copyContext);
  vm.runInContext(source, copyContext);

  assert.ok(domContentLoadedCallback);
  domContentLoadedCallback();

  const clickHandler = mockBtn.listeners['click'];
  assert.ok(clickHandler);

  let preventDefaultCalled = false;
  const clickPromise = clickHandler({
    preventDefault() {
      preventDefaultCalled = true;
    }
  });

  await clickPromise;

  assert.ok(preventDefaultCalled);
  assert.ok(writeClipboardCalled);
  assert.equal(writtenText, 'evidence details content');
  assert.ok(mockBtn.disabled);
  assert.ok(mockBtn.innerHTML.includes('已复制!'));

  assert.ok(setTimeoutCallback);
  assert.equal(setTimeoutDelay, 1200);
  setTimeoutCallback();

  assert.ok(!mockBtn.disabled);
  assert.equal(mockBtn.innerHTML, '<span>复制数据</span>');
}

// Test DOMContentLoaded - Evidence verification Copy Button (Failure path)
{
  let domContentLoadedCallback = null;

  const mockTarget = {
    innerText: 'evidence details content'
  };

  const mockBtn = {
    listeners: {},
    disabled: false,
    innerHTML: '<span>复制数据</span>',
    getAttribute(attr) {
      if (attr === 'data-copy-target-id') return 'evidence-pre-0';
      return null;
    },
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    }
  };

  let setTimeoutCallback = null;

  const copyContext = {
    window: {
      isSecureContext: true,
      getComputedStyle: (el) => el.style || {}
    },
    navigator: {
      clipboard: {
        writeText: async (text) => {
          throw new Error('clipboard error');
        }
      }
    },
    document: {
      addEventListener(event, handler) {
        if (event === 'DOMContentLoaded') {
          domContentLoadedCallback = handler;
        }
      },
      getElementById(id) {
        if (id === 'evidence-pre-0') return mockTarget;
        return null;
      },
      querySelectorAll(sel) {
        if (sel === '[data-copy-target-id]') {
          return [mockBtn];
        }
        return [];
      },
      querySelector() {
        return null;
      }
    },
    setTimeout(handler, delay) {
      setTimeoutCallback = handler;
    },
    setInterval: () => {}
  };

  vm.createContext(copyContext);
  vm.runInContext(source, copyContext);

  domContentLoadedCallback();

  const clickHandler = mockBtn.listeners['click'];
  await clickHandler({
    preventDefault() {}
  });

  assert.ok(mockBtn.innerHTML.includes('复制失败'));

  setTimeoutCallback();
  assert.equal(mockBtn.innerHTML, '<span>复制数据</span>');
}

// Test suite for Popovers, smart search input routing, tags rendering & removal, and apply buttons
{
  let submitCount = 0;
  const mockForm = {
    id: 'report-filter-form',
    submitCount: 0,
    requestSubmit() {
      this.submit();
    },
    submit() {
      this.submitCount++;
    },
    addEventListener(event, handler) {
      if (event === 'submit') {
        this.submitHandler = handler;
      }
    },
    submitHandler: null,
    querySelector(selector) {
      if (selector === 'input[name="port"]') return mockPortInput;
      if (selector === 'input[name="service"]') return mockServiceInput;
      if (selector === 'input[name="source"]') return mockSourceInput;
      if (selector.startsWith('.popover-checkbox-item input[value=')) {
        const val = selector.match(/value="([^"]+)"/)[1];
        return mockCheckboxes.find(c => c.value === val);
      }
      return null;
    }
  };

  const mockSmartInput = { id: 'smart-search-input', value: '' };
  const mockHiddenIP = { id: 'hidden-ip', value: '' };
  const mockHiddenQ = { id: 'hidden-q', value: '' };
  const mockViewSelect = { id: 'filter-view-select', value: 'ports' };
  
  const mockPortInput = { value: '' };
  const mockServiceInput = { value: '' };
  const mockSourceInput = { value: '' };

  const mockCheckboxes = [
    { checked: false, value: 'high' },
    { checked: false, value: 'medium' }
  ];

  const mockBadgesRow = {
    id: 'badges-row-content',
    innerHTML: '',
    children: [],
    appendChild(child) {
      this.children.push(child);
    }
  };
  const mockBadgesContainer = {
    id: 'active-filter-badges',
    style: { display: 'none' }
  };
  const mockSeverityCount = {
    id: 'active-severity-count',
    textContent: '',
    style: { display: 'none' }
  };

  const mockChevron = {
    style: { transform: '' }
  };
  const mockTriggerBtn = {
    listeners: {},
    classList: {
      classes: new Set(['popover-trigger-btn']),
      add(cls) { this.classes.add(cls); },
      remove(cls) { this.classes.delete(cls); },
      contains(cls) { return this.classes.has(cls); }
    },
    getAttribute(name) {
      if (name === 'data-popover-target') return 'popover-panel-1';
      return null;
    },
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    },
    querySelector(sel) {
      if (sel === '.chevron-icon') return mockChevron;
      return null;
    }
  };

  const mockPanel = {
    id: 'popover-panel-1',
    style: { display: 'none' },
    listeners: {},
    addEventListener(event, handler) {
      this.listeners[event] = handler;
    }
  };

  const elements = {
    'report-filter-form': mockForm,
    'smart-search-input': mockSmartInput,
    'hidden-ip': mockHiddenIP,
    'hidden-q': mockHiddenQ,
    'filter-view-select': mockViewSelect,
    'badges-row-content': mockBadgesRow,
    'active-filter-badges': mockBadgesContainer,
    'active-severity-count': mockSeverityCount,
    'popover-panel-1': mockPanel
  };

  let docClickHandlers = [];
  const testContext = {
    window: {
      getComputedStyle: (el) => el.style || {}
    },
    document: {
      getElementById(id) {
        return elements[id] || null;
      },
      querySelectorAll(selector) {
        if (selector === '[data-popover-target]') {
          return [mockTriggerBtn];
        }
        if (selector === '.popover-panel') {
          return [mockPanel];
        }
        if (selector === '.popover-trigger-btn') {
          return [mockTriggerBtn];
        }
        if (selector === '.popover-checkbox-item input[type="checkbox"]') {
          return mockCheckboxes;
        }
        return [];
      },
      createElement(tag) {
        return {
          tagName: tag.toUpperCase(),
          className: '',
          _innerHTML: '',
          textContent: '',
          children: [],
          listeners: {},
          get innerHTML() {
            if (this._innerHTML) return this._innerHTML;
            return this.children.map(c => {
              const tagLower = c.tagName.toLowerCase();
              return `<${tagLower} class="${c.className}">${c.textContent || c.innerHTML}</${tagLower}>`;
            }).join('');
          },
          set innerHTML(val) {
            this._innerHTML = val;
          },
          addEventListener(event, handler) {
            this.listeners[event] = handler;
          },
          appendChild(child) {
            this.children.push(child);
          }
        };
      },
      querySelector(selector) {
        return null;
      },
      addEventListener(event, handler) {
        if (event === 'click') {
          docClickHandlers.push(handler);
        }
      }
    },
    setInterval() {}
  };

  vm.createContext(testContext);
  vm.runInContext(source, testContext);

  // 1. Popover clicks toggle display, chevron rotate, active classes
  const clickHandler = mockTriggerBtn.listeners['click'];
  assert.ok(clickHandler, 'Popover click handler should be registered');

  // Click trigger (it was display 'none') -> should show ('flex'), rotate chevron, add active class
  let triggerStopProp = false;
  clickHandler({
    stopPropagation() {
      triggerStopProp = true;
    }
  });
  assert.ok(triggerStopProp);
  assert.equal(mockPanel.style.display, 'flex');
  assert.ok(mockTriggerBtn.classList.contains('active'));
  assert.equal(mockChevron.style.transform, 'rotate(180deg)');

  // Click again (it is now 'flex', i.e. not 'none') -> should hide ('none'), reset chevron, remove active
  triggerStopProp = false;
  clickHandler({ stopPropagation() { triggerStopProp = true; } });
  assert.ok(triggerStopProp);
  assert.equal(mockPanel.style.display, 'none');
  assert.ok(!mockTriggerBtn.classList.contains('active'));
  assert.equal(mockChevron.style.transform, 'rotate(0deg)');

  // Click panel directly -> should NOT hide (stopPropagation is called)
  const panelClickHandler = mockPanel.listeners['click'];
  assert.ok(panelClickHandler);
  let panelStopProp = false;
  panelClickHandler({ stopPropagation() { panelStopProp = true; } });
  assert.ok(panelStopProp);

  // Click document body -> should close popovers
  // First make panel visible
  mockPanel.style.display = 'flex';
  mockTriggerBtn.classList.add('active');
  mockChevron.style.transform = 'rotate(180deg)';
  
  assert.ok(docClickHandlers.length > 0);
  docClickHandlers.forEach(h => h({ target: { closest: () => null } }));
  assert.equal(mockPanel.style.display, 'none');
  assert.ok(!mockTriggerBtn.classList.contains('active'));
  assert.equal(mockChevron.style.transform, 'rotate(0deg)');

  // 2. Smart search input routing tests
  const submitHandler = mockForm.submitHandler;
  assert.ok(submitHandler, 'Submit handler should be registered');

  // Case 2.1: Input IP
  mockSmartInput.value = '192.168.1.1';
  submitHandler();
  assert.equal(mockHiddenIP.value, '192.168.1.1');
  assert.equal(mockHiddenQ.value, '');

  // Case 2.2: Input IP CIDR
  mockSmartInput.value = '10.0.0.0/24';
  submitHandler();
  assert.equal(mockHiddenIP.value, '10.0.0.0/24');
  assert.equal(mockHiddenQ.value, '');

  // Case 2.3: Input IP range
  mockSmartInput.value = '192.168.1.1-254';
  submitHandler();
  assert.equal(mockHiddenIP.value, '192.168.1.1-254');
  assert.equal(mockHiddenQ.value, '');

  // Case 2.4: Input comma-separated IPs
  mockSmartInput.value = '1.1.1.1,8.8.8.8';
  submitHandler();
  assert.equal(mockHiddenIP.value, '1.1.1.1,8.8.8.8');
  assert.equal(mockHiddenQ.value, '');

  // Case 2.5: Input non-IP text query
  mockSmartInput.value = 'cve-2026';
  submitHandler();
  assert.equal(mockHiddenIP.value, '');
  assert.equal(mockHiddenQ.value, 'cve-2026');

  // Case 2.6: Input empty value
  mockSmartInput.value = '  ';
  submitHandler();
  assert.equal(mockHiddenIP.value, '');
  assert.equal(mockHiddenQ.value, '');

  // 3. Test Popover-Apply Submit Button simulation
  mockForm.submitCount = 0;
  const mockApplyBtn = {
    tagName: 'BUTTON',
    type: 'submit',
    click() {
      if (mockForm.submitHandler) {
        mockForm.submitHandler();
      }
    }
  };
  mockSmartInput.value = '192.168.2.2';
  mockApplyBtn.click();
  assert.equal(mockHiddenIP.value, '192.168.2.2');
}

// Test suite for dynamic tags rendering and badge removal triggers submit
{
  let submitCount = 0;
  const mockForm = {
    id: 'report-filter-form',
    requestSubmit() {
      this.submit();
    },
    submit() {
      submitCount++;
    },
    addEventListener(event, handler) {},
    querySelector(selector) {
      if (selector === 'input[name="port"]') return mockPortInput;
      if (selector === 'input[name="service"]') return mockServiceInput;
      if (selector === 'input[name="source"]') return mockSourceInput;
      if (selector.startsWith('.popover-checkbox-item input[value=')) {
        const val = selector.match(/value="([^"]+)"/)[1];
        return mockCheckboxes.find(c => c.value === val);
      }
      return null;
    }
  };

  const mockSmartInput = { id: 'smart-search-input', value: '192.168.1.1' };
  const mockHiddenIP = { id: 'hidden-ip', value: '192.168.1.1' };
  const mockHiddenQ = { id: 'hidden-q', value: 'cve-2026' };
  const mockViewSelect = { id: 'filter-view-select', value: 'hosts' }; // not 'ports'
  
  const mockPortInput = { value: '80' };
  const mockServiceInput = { value: 'http' };
  const mockSourceInput = { value: 'nmap' };

  const mockCheckboxes = [
    { checked: true, value: 'high' },
    { checked: false, value: 'medium' }
  ];

  const mockBadgesRow = {
    id: 'badges-row-content',
    innerHTML: '',
    children: [],
    appendChild(child) {
      this.children.push(child);
    }
  };
  const mockBadgesContainer = {
    id: 'active-filter-badges',
    style: { display: 'none' }
  };
  const mockSeverityCount = {
    id: 'active-severity-count',
    textContent: '',
    style: { display: 'none' }
  };

  const elements = {
    'report-filter-form': mockForm,
    'smart-search-input': mockSmartInput,
    'hidden-ip': mockHiddenIP,
    'hidden-q': mockHiddenQ,
    'filter-view-select': mockViewSelect,
    'badges-row-content': mockBadgesRow,
    'active-filter-badges': mockBadgesContainer,
    'active-severity-count': mockSeverityCount,
  };

  const tagContext = {
    window: {
      getComputedStyle: (el) => el.style || {}
    },
    document: {
      getElementById(id) {
        return elements[id] || null;
      },
      querySelectorAll(selector) {
        if (selector === '.popover-checkbox-item input[type="checkbox"]') {
          return mockCheckboxes;
        }
        return [];
      },
      createElement(tag) {
        return {
          tagName: tag.toUpperCase(),
          className: '',
          _innerHTML: '',
          textContent: '',
          children: [],
          listeners: {},
          get innerHTML() {
            if (this._innerHTML) return this._innerHTML;
            return this.children.map(c => {
              const tagLower = c.tagName.toLowerCase();
              return `<${tagLower} class="${c.className}">${c.textContent || c.innerHTML}</${tagLower}>`;
            }).join('');
          },
          set innerHTML(val) {
            this._innerHTML = val;
          },
          addEventListener(event, handler) {
            this.listeners[event] = handler;
          },
          appendChild(child) {
            this.children.push(child);
          }
        };
      },
      querySelector(selector) {
        return null;
      },
      addEventListener(event, handler) {}
    },
    setInterval() {}
  };

  vm.createContext(tagContext);
  vm.runInContext(source, tagContext);

  // Assertions on tags generated
  assert.equal(mockBadgesContainer.style.display, 'flex');
  assert.equal(mockBadgesRow.children.length, 7); // IP, Q, port, service, source, view, severity (high)
  assert.equal(mockSeverityCount.textContent, 1);
  assert.equal(mockSeverityCount.style.display, 'inline-block');

  // Find IP tag and click ✕
  const ipTag = mockBadgesRow.children.find(t => t.innerHTML.includes('IP'));
  assert.ok(ipTag);
  const ipRemoveBtn = ipTag.children.find(c => c.className === 'filter-badge-tag-remove');
  assert.ok(ipRemoveBtn);
  assert.equal(ipRemoveBtn.innerHTML, '✕');

  let stopPropagationCalled = false;
  ipRemoveBtn.listeners['click']({
    stopPropagation() {
      stopPropagationCalled = true;
    }
  });
  assert.ok(stopPropagationCalled);
  assert.equal(mockHiddenIP.value, '');
  assert.equal(mockSmartInput.value, '');
  assert.equal(submitCount, 1);

  // Find severity tag and click ✕
  const lvlTag = mockBadgesRow.children.find(t => t.innerHTML.includes('级别'));
  assert.ok(lvlTag);
  const lvlRemoveBtn = lvlTag.children.find(c => c.className === 'filter-badge-tag-remove');
  assert.ok(lvlRemoveBtn);

  submitCount = 0;
  lvlRemoveBtn.listeners['click']({ stopPropagation() {} });
  assert.equal(mockCheckboxes[0].checked, false);
  assert.equal(submitCount, 1);
}

// Test suite for XSS prevention when rendering tags with HTML characters
{
  const mockForm = {
    id: 'report-filter-form',
    requestSubmit() {},
    submit() {},
    addEventListener(event, handler) {},
    querySelector(selector) {
      return null;
    }
  };

  const mockSmartInput = { id: 'smart-search-input', value: '' };
  const mockHiddenIP = { id: 'hidden-ip', value: '' };
  const mockHiddenQ = { id: 'hidden-q', value: '<img src=x onerror=alert(1)>' };
  const mockViewSelect = { id: 'filter-view-select', value: 'ports' };

  const mockBadgesRow = {
    id: 'badges-row-content',
    innerHTML: '',
    children: [],
    appendChild(child) {
      this.children.push(child);
    }
  };
  const mockBadgesContainer = {
    id: 'active-filter-badges',
    style: { display: 'none' }
  };
  const mockSeverityCount = {
    id: 'active-severity-count',
    textContent: '',
    style: { display: 'none' }
  };

  const elements = {
    'report-filter-form': mockForm,
    'smart-search-input': mockSmartInput,
    'hidden-ip': mockHiddenIP,
    'hidden-q': mockHiddenQ,
    'filter-view-select': mockViewSelect,
    'badges-row-content': mockBadgesRow,
    'active-filter-badges': mockBadgesContainer,
    'active-severity-count': mockSeverityCount,
  };

  const xssContext = {
    window: {
      getComputedStyle: (el) => el.style || {}
    },
    document: {
      getElementById(id) {
        return elements[id] || null;
      },
      querySelectorAll(selector) {
        return [];
      },
      createElement(tag) {
        return {
          tagName: tag.toUpperCase(),
          className: '',
          _innerHTML: '',
          textContent: '',
          children: [],
          listeners: {},
          get innerHTML() {
            if (this._innerHTML) return this._innerHTML;
            return this.children.map(c => {
              const tagLower = c.tagName.toLowerCase();
              return `<${tagLower} class="${c.className}">${c.textContent || c.innerHTML}</${tagLower}>`;
            }).join('');
          },
          set innerHTML(val) {
            this._innerHTML = val;
          },
          addEventListener(event, handler) {
            this.listeners[event] = handler;
          },
          appendChild(child) {
            this.children.push(child);
          }
        };
      },
      querySelector(selector) {
        return null;
      },
      addEventListener(event, handler) {}
    },
    setInterval() {}
  };

  vm.createContext(xssContext);
  vm.runInContext(source, xssContext);

  // Assertions
  assert.equal(mockBadgesRow.children.length, 1);
  const keywordTag = mockBadgesRow.children[0];
  
  // The keyword tag should have two children: the text span and the remove button span
  assert.equal(keywordTag.children.length, 2);
  const textSpan = keywordTag.children[0];
  assert.equal(textSpan.tagName, 'SPAN');
  
  // Verify that the HTML payload is safely stored in textContent and has NOT been parsed/rendered as HTML elements
  assert.equal(textSpan.textContent, '关键词: <img src=x onerror=alert(1)>');
  assert.equal(textSpan._innerHTML, ''); // should not set raw innerHTML of the text span!
}
