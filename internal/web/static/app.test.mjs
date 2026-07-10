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


