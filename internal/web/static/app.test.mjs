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
assert.equal(context.queueNameFromHash('#negative'), 'negative');

const ansi = context.ansiSegments('[[34mINF[0m] <img>');
assert.equal(ansi.map((segment) => segment.text).join(''), '[INF] <img>');
assert.ok(ansi.some((segment) => segment.text === 'INF' && segment.color));
const markup = context.ansiHTML('\x1b[31mred\x1b[0m <img>');
assert.ok(markup.includes('style="color:#cd3131"'));
assert.ok(markup.includes('&lt;img&gt;'));
