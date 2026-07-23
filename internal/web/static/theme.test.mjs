import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../frontend/theme.ts', import.meta.url), 'utf8');

function runWithThemeContext(initialStorage = {}, prefersDark = false) {
  const store = { ...initialStorage };
  const ctx = {
    localStorage: {
      getItem: (key) => store[key] ?? null,
      setItem: (key, value) => { store[key] = value; },
      removeItem: (key) => { delete store[key]; },
    },
    window: {
      matchMedia: (query) => ({
        matches: query === '(prefers-color-scheme: dark)' && prefersDark,
        addEventListener: () => {},
        addListener: () => {},
      }),
    },
    document: {
      documentElement: {
        attributes: {},
        style: {},
        setAttribute(key, value) { this.attributes[key] = value; },
      },
    },
  };
  vm.createContext(ctx);
  // Strip TypeScript type-only syntax that vm cannot parse.
  const js = source
    .replace(/export type ThemePreference = .*/g, '')
    .replace(/: ThemePreference/g, '')
    .replace(/: 'light' \| 'dark'/g, '')
    .replace(/: void/g, '')
    .replace(/\(mq as any\)/g, 'mq')
    .replace(/export /g, '');
  vm.runInContext(js, ctx);
  return ctx;
}

// Invalid stored value falls back to system.
{
  const ctx = runWithThemeContext({ 'anchor-theme': 'neon' });
  assert.equal(ctx.getTheme(), 'system');
  ctx.setTheme('system');
  assert.equal(ctx.document.documentElement.attributes['data-theme'], 'light');
}

// Stored light/dark is respected regardless of system preference.
{
  const ctx = runWithThemeContext({ 'anchor-theme': 'dark' }, false);
  assert.equal(ctx.getTheme(), 'dark');
  ctx.setTheme('dark');
  assert.equal(ctx.document.documentElement.attributes['data-theme'], 'dark');
}

// System preference resolves to dark when matchMedia reports dark.
{
  const ctx = runWithThemeContext({}, true);
  assert.equal(ctx.getTheme(), 'system');
  ctx.setTheme('system');
  assert.equal(ctx.document.documentElement.attributes['data-theme'], 'dark');
}

// setTheme writes, removes, and applies the resolved theme.
{
  const ctx = runWithThemeContext({}, false);
  ctx.setTheme('dark');
  assert.equal(ctx.localStorage.getItem('anchor-theme'), 'dark');
  assert.equal(ctx.document.documentElement.attributes['data-theme'], 'dark');
  assert.equal(ctx.document.documentElement.style.colorScheme, 'dark');

  ctx.setTheme('system');
  assert.equal(ctx.localStorage.getItem('anchor-theme'), null);
  assert.equal(ctx.document.documentElement.attributes['data-theme'], 'light');
}

// initTheme applies theme and returns current preference.
{
  const ctx = runWithThemeContext({ 'anchor-theme': 'light' }, true);
  assert.equal(ctx.initTheme(), 'light');
  assert.equal(ctx.document.documentElement.attributes['data-theme'], 'light');
}

// localStorage unavailable does not throw.
{
  const ctx = runWithThemeContext();
  ctx.localStorage = null;
  assert.doesNotThrow(() => ctx.setTheme('dark'));
}
