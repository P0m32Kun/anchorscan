import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('./app.js', import.meta.url), 'utf8');
const context = {
  window: {},
  document: {
    getElementById: () => null,
    querySelector: () => null,
    addEventListener: () => {},
  },
  setInterval: () => {},
};
vm.createContext(context);
vm.runInContext(source, context);

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
    window: {},
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

  // 1. Verify initial active class restoration (mockCheckbox is checked, so parent gets active)
  assert.ok(mockClassList.classes.has('active'));

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

  // 3. Verify view select immediate submit
  const viewChangeHandler = mockViewSelect.listeners['change'];
  assert.ok(viewChangeHandler);
  viewChangeHandler();
  assert.equal(mockForm.submitCalledCount, 1);

  // 4. Verify checkbox change event triggers submit and active class toggle
  const checkboxChangeHandler = mockCheckbox.listeners['change'];
  assert.ok(checkboxChangeHandler);

  mockCheckbox.checked = false;
  checkboxChangeHandler.call(mockCheckbox);
  assert.ok(!mockClassList.classes.has('active'));
  assert.equal(mockForm.submitCalledCount, 2);
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
      isSecureContext: true
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
      isSecureContext: true
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



