import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

for (const name of fs.readdirSync(new URL('../templates/', import.meta.url))) {
  const template = fs.readFileSync(new URL(`../templates/${name}`, import.meta.url), 'utf8');
  assert.equal(template.includes('/static/app.js'), name === 'base.html', `${name} must not load app.js`);
}

const source = fs.readFileSync(new URL('./app.js', import.meta.url), 'utf8');
const callbacks = [];
const context = {
  window: { getComputedStyle: (element) => element.style || {} },
  document: { addEventListener: (event, callback) => { if (event === 'DOMContentLoaded') callbacks.push(callback); }, querySelector: () => null },
  setTimeout: () => {},
};
vm.createContext(context);
vm.runInContext(source, context);
assert.equal(callbacks.length, 1);

const navItems = [
  { id: 'nav-home', classList: { active: false, toggle(_, value) { this.active = value; } } },
  { id: 'nav-projects', classList: { active: false, toggle(_, value) { this.active = value; } } },
  { id: 'nav-runs', classList: { active: false, toggle(_, value) { this.active = value; } } },
];
context.setActiveNavigation('/projects/demo', navItems);
assert.equal(navItems[1].classList.active, true);
assert.equal(navItems[2].classList.active, false);
assert.doesNotMatch(source, /imagesFromClipboardData|queueNameFromHash|ansiSegments|renderANSI|ansiHTML/);

for (const name of ['RunDetail.vue', 'ToolRunFeedback.vue']) {
  const component = fs.readFileSync(new URL(`../frontend/${name}`, import.meta.url), 'utf8');
  assert.match(component, /contains\(selection\.anchorNode\) \|\| .*contains\(selection\.focusNode\)/, `${name} must preserve reverse selections while polling`);
}

const workbench = fs.readFileSync(new URL('../frontend/Workbench.vue', import.meta.url), 'utf8');
assert.doesNotMatch(workbench, /\bconfirm\s*\(/, 'Workbench destructive actions must use the shared confirmation dialog');
assert.match(workbench, /anchorscan:confirm/, 'Workbench must request the shared confirmation dialog');
assert.match(workbench, /function verificationRequestPayload\(/, 'Workbench must own one snake_case verification request projection');
assert.match(workbench, /function verificationUpdateRequestPayload\(/, 'verification updates must exclude create-only associations');
assert.match(workbench, /zone_id: verifyZoneId\.value/, 'positive verification requests must send snake_case zone_id');
assert.match(workbench, /body: JSON\.stringify\(payload\)/, 'create must serialize the shared request projection');
assert.match(workbench, /body: JSON\.stringify\(verificationUpdateRequestPayload\(c\)\)/, 'update must serialize its supported request projection');
assert.match(workbench, /function verificationKey\(zoneID: string, vulnerabilityKey: string\)/, 'verification lookup must include the zone boundary');
assert.match(workbench, /fetchCommand\(c: Candidate, tool: string, asset: string, verificationID: string\)/, 'tool commands must retain their candidate zone');
assert.match(workbench, /body\.set\('zone_id', c\.ZoneID\)/, 'tool command requests must send the candidate zone');

function renderDistribution(badges) {
  const callbacks = [];
  const container = { style: { display: 'unset' } };
  const bar = { innerHTML: '' };
  const legend = { innerHTML: '' };
  const reportContext = {
    document: {
      addEventListener: (event, callback) => { if (event === 'DOMContentLoaded') callbacks.push(callback); },
      getElementById: (id) => ({ 'distribution-container': container, 'distribution-bar': bar, 'distribution-legend': legend })[id],
      querySelectorAll: (selector) => selector === '.severity-badge' ? badges : [],
    },
  };
  vm.createContext(reportContext);
  vm.runInContext(fs.readFileSync(new URL('./report-ui.js', import.meta.url), 'utf8'), reportContext);
  callbacks.at(-1)();
  return { container, bar, legend };
}

const populatedDistribution = renderDistribution(['critical', 'high', 'high', 'medium'].map(textContent => ({ textContent })));
assert.equal(populatedDistribution.container.style.display, 'block');
assert.ok(populatedDistribution.bar.innerHTML.includes('critical'));
assert.ok(populatedDistribution.legend.innerHTML.includes('高危 (High): <span class="legend-count">2'));
assert.equal(renderDistribution([]).container.style.display, 'none');
