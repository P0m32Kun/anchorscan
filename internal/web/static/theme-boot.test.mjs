import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const baseHtml = fs.readFileSync(new URL('../templates/base.html', import.meta.url), 'utf8');
const themeSource = fs.readFileSync(new URL('../frontend/theme.ts', import.meta.url), 'utf8');

function extractInlineBootScript() {
  const m = baseHtml.match(/\<script\>([\s\S]*?)\<\/script\>/);
  assert.ok(m, 'base.html should contain an inline theme boot script');
  return m[1];
}

function makeContext(initialStorage = {}, prefersDark = false, localStorageAvailable = true) {
  const store = { ...initialStorage };
  const ls = localStorageAvailable ? {
    getItem: (key) => store[key] ?? null,
    setItem: (key, value) => { store[key] = value; },
    removeItem: (key) => { delete store[key]; },
  } : null;
  const doc = {
    documentElement: {
      attributes: {},
      style: {},
      setAttribute(key, value) { this.attributes[key] = value; },
    },
  };
  const win = {
    localStorage: ls,
    matchMedia: (query) => ({
      matches: query === '(prefers-color-scheme: dark)' && prefersDark,
      addEventListener: () => {},
      addListener: () => {},
    }),
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => {},
  };
  // Both the inline script and theme.ts use `window` as their global.
  const ctx = { window: win, document: doc, CustomEvent: class {
    constructor(type, options = {}) {
      this.type = type;
      this.detail = options.detail;
    }
  } };
  if (localStorageAvailable) {
    ctx.localStorage = ls;
  }
  return ctx;
}

function runInlineScript(ctx) {
  const script = extractInlineBootScript();
  vm.createContext(ctx);
  vm.runInContext(script, ctx);
  return ctx.document.documentElement.attributes['data-theme'];
}

function runThemeTs(ctx) {
  const js = themeSource
    .replace(/export type ThemePreference = .*/g, '')
    .replace(/: ThemePreference/g, '')
    .replace(/: 'light' \| 'dark'/g, '')
    .replace(/: void/g, '')
    .replace(/\(mq as any\)/g, 'mq')
    .replace(/export /g, '');
  vm.createContext(ctx);
  vm.runInContext(js, ctx);
  ctx.initTheme();
  return ctx.document.documentElement.attributes['data-theme'];
}

function compareScenario(label, storage, prefersDark, localStorageAvailable = true) {
  const ctxA = makeContext(storage, prefersDark, localStorageAvailable);
  const ctxB = makeContext(storage, prefersDark, localStorageAvailable);
  const a = runInlineScript(ctxA);
  const b = runThemeTs(ctxB);
  assert.equal(a, b, `${label}: inline boot (${a}) != theme.ts (${b})`);
}

// system preference on first load
compareScenario('system-light', {}, false);
compareScenario('system-dark', {}, true);

// explicit stored preference
compareScenario('stored-light', { 'anchor-theme': 'light' }, false);
compareScenario('stored-dark', { 'anchor-theme': 'dark' }, false);
compareScenario('stored-system-follows-dark', { 'anchor-theme': 'system' }, true);

// invalid stored value falls back to system
compareScenario('invalid-stored', { 'anchor-theme': 'neon' }, false);

// localStorage unavailable does not throw and both resolve to system light
compareScenario('no-localstorage', {}, false, false);
