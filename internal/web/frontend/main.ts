import { createApp } from 'vue';
import ThemeToggle from './ThemeToggle.vue';
import { initTheme } from './theme';

initTheme();

function mountThemeControls() {
  const mountPoints = document.querySelectorAll('[data-theme-control]');
  mountPoints.forEach((el) => {
    const app = createApp(ThemeToggle);
    app.mount(el);
  });
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', mountThemeControls);
} else {
  mountThemeControls();
}
