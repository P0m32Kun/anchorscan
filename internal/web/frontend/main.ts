import { createApp } from 'vue';
import ConfirmActions from './ConfirmActions.vue';
import ReportInteractions from './ReportInteractions.vue';
import RunDetail from './RunDetail.vue';
import ScanCreate from './ScanCreate.vue';
import ThemeToggle from './ThemeToggle.vue';
import ToolRunFeedback from './ToolRunFeedback.vue';
import Workbench from './Workbench.vue';
import { normalizeOutcome, normalizeSeverity } from './workbench-api';
import { initTheme } from './theme';

initTheme();

function mountThemeControls() {
  const mountPoints = document.querySelectorAll('[data-theme-control]');
  mountPoints.forEach((el) => {
    const app = createApp(ThemeToggle);
    app.mount(el);
  });
}

function mountConfirmActions() {
  const mountPoint = document.querySelector<HTMLElement>('[data-confirm-actions]');
  if (mountPoint) createApp(ConfirmActions).mount(mountPoint);
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
  props.verifications = props.verifications.map((verification: Record<string, unknown>) => ({
    ...verification,
    Outcome: normalizeOutcome(verification.Outcome),
    Severity: normalizeSeverity(verification.Severity),
  }));
  props.candidates = props.candidates.map((candidate: Record<string, unknown>) => ({
    ...candidate,
    Assets: Array.isArray(candidate.Assets) ? candidate.Assets : [],
    Sources: Array.isArray(candidate.Sources) ? candidate.Sources : [],
  }));
  props.counts ||= { positive: 0, negative: 0, incomplete: 0 };
  createApp(Workbench, props).mount(mountPoint);
  mountPoint.dataset.mounted = 'true';
}

function mountReportInteractions() {
  const mountPoint = document.querySelector<HTMLElement>('[data-report-interactions]');
  if (!mountPoint) return;
  let serviceFacets: unknown = [];
  try {
    serviceFacets = JSON.parse(mountPoint.dataset.serviceFacets || '[]');
  } catch {
    // The server always emits this JSON; retain an empty list if it is malformed.
  }
  let pivotFacets: unknown = [];
  try {
    pivotFacets = JSON.parse(mountPoint.dataset.pivotFacets || '[]');
  } catch {
    pivotFacets = [];
  }
  let serviceMatrix: unknown = null;
  try {
    serviceMatrix = JSON.parse(mountPoint.dataset.serviceMatrix || 'null');
  } catch {
    serviceMatrix = null;
  }
  createApp(ReportInteractions, { serviceFacets, pivotFacets, serviceMatrix }).mount(mountPoint);
  mountPoint.dataset.mounted = 'true';
}

function mountRunDetail() {
  const mountPoint = document.querySelector<HTMLElement>('[data-run-detail]');
  if (!mountPoint) return;
  createApp(RunDetail, JSON.parse(mountPoint.dataset.runProps || '{}')).mount(mountPoint);
  mountPoint.dataset.mounted = 'true';
}

function mountToolRunFeedback() {
  const mountPoint = document.querySelector<HTMLElement>('[data-tool-run-feedback]');
  if (!mountPoint) return;
  createApp(ToolRunFeedback).mount(mountPoint);
  mountPoint.dataset.mounted = 'true';
}

function mountApps() {
  mountThemeControls();
  mountConfirmActions();
  mountScanCreate();
  mountWorkbench();
  mountReportInteractions();
  mountRunDetail();
  mountToolRunFeedback();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', mountApps);
} else {
  mountApps();
}
