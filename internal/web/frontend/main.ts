import { createApp } from 'vue';
import ScanCreate from './ScanCreate.vue';
import ThemeToggle from './ThemeToggle.vue';
import Workbench from './Workbench.vue';
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

function mountWorkbench() {
  const mountPoint = document.querySelector<HTMLElement>('[data-workbench]');
  if (!mountPoint) return;
  const props = JSON.parse(mountPoint.dataset.workbenchProps || '{}');
  props.candidates ||= [];
  props.negative_groups ||= [];
  props.incomplete_checks ||= [];
  props.verifications ||= [];
  props.zones ||= [];
  props.zone_names ||= {};
  props.counts ||= { positive: 0, negative: 0, incomplete: 0 };
  createApp(Workbench, props).mount(mountPoint);
  mountPoint.dataset.mounted = 'true';
}

function mountApps() {
  mountThemeControls();
  mountScanCreate();
  mountWorkbench();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', mountApps);
} else {
  mountApps();
}
