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

