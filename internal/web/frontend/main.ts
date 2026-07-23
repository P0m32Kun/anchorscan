import { createApp } from 'vue';
import ScanCreate from './ScanCreate.vue';
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

function mountScanCreate() {
  const mountPoint = document.querySelector<HTMLElement>('[data-scan-create]');
  if (!mountPoint) return;
  const props = JSON.parse(mountPoint.dataset.scanCreateProps || '{}');
  props.defaultZoneId = mountPoint.dataset.defaultZoneId || '';
  props.errors ||= [];
  props.warnings ||= [];
  createApp(ScanCreate, props).mount(mountPoint);
  mountPoint.dataset.mounted = 'true';
}

function mountApps() {
  mountThemeControls();
  mountScanCreate();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', mountApps);
} else {
  mountApps();
}
