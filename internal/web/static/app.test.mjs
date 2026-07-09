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
