export type ThemePreference = 'system' | 'light' | 'dark';

const STORAGE_KEY = 'anchor-theme';

function getStoredPreference(): ThemePreference {
  const stored = (typeof localStorage !== 'undefined' && localStorage ? localStorage.getItem(STORAGE_KEY) : '') || '';
  if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
  return 'system';
}

function resolveTheme(pref: ThemePreference): 'light' | 'dark' {
  if (pref === 'light' || pref === 'dark') return pref;
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  return 'light';
}

function applyTheme(theme: 'light' | 'dark'): void {
  const html = document.documentElement;
  html.setAttribute('data-theme', theme);
  html.style.colorScheme = theme;
}

export function getTheme(): ThemePreference {
  return getStoredPreference();
}

export function setTheme(pref: ThemePreference): void {
  if (typeof localStorage !== 'undefined' && localStorage) {
    if (pref === 'system') {
      localStorage.removeItem(STORAGE_KEY);
    } else {
      localStorage.setItem(STORAGE_KEY, pref);
    }
  }
  applyTheme(resolveTheme(pref));
}

export function initTheme(): ThemePreference {
  const pref = getStoredPreference();
  applyTheme(resolveTheme(pref));

  if (typeof window !== 'undefined' && window.matchMedia) {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const listener = () => {
      if (getStoredPreference() === 'system') {
        applyTheme(resolveTheme('system'));
      }
    };
    if (mq.addEventListener) {
      mq.addEventListener('change', listener);
    } else if ((mq as any).addListener) {
      (mq as any).addListener(listener);
    }
  }

  return pref;
}

