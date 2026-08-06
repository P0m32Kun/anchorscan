import assert from 'node:assert/strict';
import fs from 'node:fs';

// Nmap Viewer consolidation, ticket 02: shell token and responsive-shell contract.
// Guards the approved product-design baseline: normalized spacing/control/motion
// tokens, a single global CSS entry, and a horizontal-strip sidebar below 900px
// so no page scrolls horizontally at the document level.

const style = fs.readFileSync(new URL('./style.css', import.meta.url), 'utf8');
const dark = fs.readFileSync(new URL('./dark.css', import.meta.url), 'utf8');
const base = fs.readFileSync(new URL('../templates/base.html', import.meta.url), 'utf8');

function ruleBlock(selector) {
  const start = style.indexOf(selector);
  assert.notEqual(start, -1, `${selector} rule must exist`);
  const open = style.indexOf('{', start);
  const close = style.indexOf('}', open);
  return style.slice(open, close);
}

// Spacing scale: 4/8/12/16/24/32/48px.
for (const [token, value] of [
  ['--space-1', '4px'],
  ['--space-2', '8px'],
  ['--space-3', '12px'],
  ['--space-4', '16px'],
  ['--space-5', '24px'],
  ['--space-6', '32px'],
  ['--space-7', '48px'],
]) {
  assert.match(style, new RegExp(`${token}:\\s*${value}`), `${token} must be ${value}`);
}
assert.match(style, /--control-height:\s*34px/, 'control height token must exist');
for (const token of ['--motion-fast', '--motion-normal', '--motion-slow']) {
  assert.match(style, new RegExp(`${token}:`), `${token} must exist`);
}

// Shared primitives consume the tokens instead of anonymous spacing values.
assert.match(ruleBlock('.sidebar {'), /padding:\s*var\(--space-/, '.sidebar padding must use space tokens');
assert.match(ruleBlock('.nav-item {'), /padding:\s*var\(--space-/, '.nav-item padding must use space tokens');
assert.match(ruleBlock('.page-header {'), /gap:\s*var\(--space-/, '.page-header gap must use space tokens');
assert.match(ruleBlock('.panel {'), /padding:\s*var\(--space-/, '.panel padding must use space tokens');
assert.match(ruleBlock('.data-table th,'), /padding:\s*var\(--space-/, 'table cells must use space tokens');
assert.match(ruleBlock('input,\nselect,\ntextarea {\n  width: 100%;'), /min-height:\s*var\(--control-height\)/, 'inputs must use the control height token');
assert.match(ruleBlock('.button,\nbutton[type="submit"] {\n  appearance: none;'), /min-height:\s*var\(--control-height\)/, 'buttons must use the control height token');
assert.match(ruleBlock('.status-badge {'), /padding:\s*var\(--space-/, 'badges must use space tokens');

// Responsive shell: sidebar becomes a local horizontal strip, page never scrolls sideways.
const shellQuery = style.indexOf('@media (max-width: 900px)');
assert.notEqual(shellQuery, -1, 'responsive shell media query must exist');
const shellSection = style.slice(shellQuery);
assert.match(shellSection, /\.app-shell\s*\{[^}]*flex-direction:\s*column/, 'app shell stacks below 900px');
assert.match(shellSection, /\.sidebar-nav\s*\{[^}]*overflow-x:\s*auto/, 'sidebar nav scrolls locally below 900px');
assert.match(shellSection, /\.page-shell\s*\{[^}]*width:\s*100%/, 'page shell uses full width below 900px');
assert.match(shellSection, /\.form-grid\s*\{[^}]*grid-template-columns:\s*1fr/, 'forms stack below 900px');

// Wide tables scroll inside their own panel, not the page.
for (const panel of ['.projects-table-panel', '.runs-table-panel', '.project-runs-panel']) {
  assert.match(style, new RegExp(`${panel.replace('.', '\\.')}\\s*\\{[^}]*overflow-x:\\s*auto`), `${panel} must scroll horizontally inside itself`);
}

// Reduced motion stays intact.
assert.match(style, /@media \(prefers-reduced-motion: reduce\)/, 'reduced-motion media query must exist');

// Single global CSS entry: base template loads exactly these stylesheets, no extra.
const links = [...base.matchAll(/<link rel="stylesheet" href="([^"]+)"/g)].map((match) => match[1]);
assert.deepEqual(links, ['/static/style.css', '/static/dark.css', '/static/dist/assets/main.css'], 'base template must keep the single style entry plus theme and built assets');
assert.doesNotMatch(style, /@import/, 'style.css must not import a second global stylesheet');

// Spacing/control/motion tokens are theme-agnostic: they live only in style.css.
for (const token of ['--space-1', '--control-height', '--motion-fast']) {
  assert.equal(dark.includes(token), false, `dark.css must not redefine ${token}`);
}
