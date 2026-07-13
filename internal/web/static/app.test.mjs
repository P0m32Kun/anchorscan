import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('./app.js', import.meta.url), 'utf8');
const runStatusSource = fs.readFileSync(new URL('./run-status.js', import.meta.url), 'utf8');
const context = {
  window: {
    getComputedStyle: (el) => el.style || {}
  },
  document: {
    getElementById: () => null,
    querySelector: () => null,
    addEventListener: () => {},
  },
  setInterval: () => {},
};
vm.createContext(context);
vm.runInContext(source, context);
vm.runInContext(runStatusSource, context);

assert.equal(
  context.formatEventTime('2026-07-09T03:16:55.614Z'),
  '2026-07-09 11:16:55',
);

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

// run renderVulnDistribution
context.renderVulnDistribution();

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

// Test suite for updateStepper
const mockSteps = {
  'step-discover': { className: '', icon: { innerHTML: '' }, querySelector(sel) { return sel === '.step-icon' ? this.icon : null; } },
  'step-portscan': { className: '', icon: { innerHTML: '' }, querySelector(sel) { return sel === '.step-icon' ? this.icon : null; } },
  'step-fingerprint': { className: '', icon: { innerHTML: '' }, querySelector(sel) { return sel === '.step-icon' ? this.icon : null; } },
  'step-vuln': { className: '', icon: { innerHTML: '' }, querySelector(sel) { return sel === '.step-icon' ? this.icon : null; } },
  'step-report': { className: '', icon: { innerHTML: '' }, querySelector(sel) { return sel === '.step-icon' ? this.icon : null; } }
};

const mockLines = [
  { className: '' },
  { className: '' },
  { className: '' },
  { className: '' }
];

function resetMockStepper() {
  Object.keys(mockSteps).forEach(k => {
    mockSteps[k].className = '';
    mockSteps[k].icon.innerHTML = '';
  });
  mockLines.forEach(l => {
    l.className = '';
  });
}

context.document.getElementById = (id) => {
  if (mockSteps[id]) return mockSteps[id];
  return null;
};
context.document.querySelectorAll = (selector) => {
  if (selector === '.step-line') return mockLines;
  return [];
};

// Test Case 1: completed status
resetMockStepper();
context.updateStepper([], 'completed');
assert.equal(mockSteps['step-discover'].className, 'step completed');
assert.equal(mockSteps['step-discover'].icon.innerHTML, '✓');
assert.equal(mockSteps['step-report'].className, 'step completed');
assert.equal(mockSteps['step-report'].icon.innerHTML, '✓');
assert.equal(mockLines[0].className, 'step-line completed');
assert.equal(mockLines[3].className, 'step-line completed');

// Test Case 2: discover stage (nmap + alive)
resetMockStepper();
context.updateStepper([
  { stage: 'nmap', message: 'Host is alive' }
], 'running');
assert.equal(mockSteps['step-discover'].className, 'step active');
assert.equal(mockSteps['step-discover'].icon.innerHTML, 1);
assert.equal(mockSteps['step-portscan'].className, 'step');
assert.equal(mockSteps['step-portscan'].icon.innerHTML, 2);
assert.equal(mockLines[0].className, 'step-line');

// Test Case 3: portscan stage (rustscan)
resetMockStepper();
context.updateStepper([
  { stage: 'nmap', message: 'Host is alive' },
  { stage: 'rustscan', message: 'Scanning ports' }
], 'running');
assert.equal(mockSteps['step-discover'].className, 'step completed');
assert.equal(mockSteps['step-discover'].icon.innerHTML, '✓');
assert.equal(mockSteps['step-portscan'].className, 'step active');
assert.equal(mockSteps['step-portscan'].icon.innerHTML, 2);
assert.equal(mockSteps['step-fingerprint'].className, 'step');
assert.equal(mockLines[0].className, 'step-line completed');
assert.equal(mockLines[1].className, 'step-line');

// Test Case 4: fingerprint stage (nmap but not alive)
resetMockStepper();
context.updateStepper([
  { stage: 'nmap', message: 'Host is alive' },
  { stage: 'rustscan', message: 'Scanning ports' },
  { stage: 'nmap', message: 'Scanning service details' }
], 'running');
assert.equal(mockSteps['step-discover'].className, 'step completed');
assert.equal(mockSteps['step-portscan'].className, 'step completed');
assert.equal(mockSteps['step-fingerprint'].className, 'step active');
assert.equal(mockSteps['step-fingerprint'].icon.innerHTML, 3);
assert.equal(mockSteps['step-vuln'].className, 'step');
assert.equal(mockLines[0].className, 'step-line completed');
assert.equal(mockLines[1].className, 'step-line completed');
assert.equal(mockLines[2].className, 'step-line');

// Test Case 5: vuln stage (nuclei / httpx / nse)
resetMockStepper();
context.updateStepper([
  { stage: 'nmap', message: 'Host is alive' },
  { stage: 'rustscan', message: 'Scanning ports' },
  { stage: 'nmap', message: 'Scanning service details' },
  { stage: 'nuclei', message: 'Checking templates' }
], 'running');
assert.equal(mockSteps['step-fingerprint'].className, 'step completed');
assert.equal(mockSteps['step-vuln'].className, 'step active');
assert.equal(mockSteps['step-vuln'].icon.innerHTML, 4);
assert.equal(mockSteps['step-report'].className, 'step');
assert.equal(mockLines[2].className, 'step-line completed');
assert.equal(mockLines[3].className, 'step-line');

// Test Case 6: report stage (report)
resetMockStepper();
context.updateStepper([
  { stage: 'nmap', message: 'Host is alive' },
  { stage: 'rustscan', message: 'Scanning ports' },
  { stage: 'nmap', message: 'Scanning service details' },
  { stage: 'nuclei', message: 'Checking templates' },
  { stage: 'report', message: 'Generating report' }
], 'running');
assert.equal(mockSteps['step-vuln'].className, 'step completed');
assert.equal(mockSteps['step-report'].className, 'step active');
assert.equal(mockSteps['step-report'].icon.innerHTML, 5);
assert.equal(mockLines[3].className, 'step-line completed');

// Test Case 7: updateStepper defensive check for null/missing steps
{
  resetMockStepper();
  const originalReport = mockSteps['step-report'];
  mockSteps['step-report'] = null;

  // This should not crash when completing
  context.updateStepper([], 'completed');
  assert.equal(mockSteps['step-discover'].className, 'step completed');
  assert.equal(mockSteps['step-discover'].icon.innerHTML, '✓');

  // Restore
  mockSteps['step-report'] = originalReport;
}

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



