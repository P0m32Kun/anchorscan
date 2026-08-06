import assert from 'node:assert/strict';
import fs from 'node:fs';

// Nmap Viewer consolidation, ticket 04: report reading & filtering contract.
// Guards: finding detail opens in a side drawer (not an inline expanded row),
// long commands stay bounded, the risk summary wraps on narrow screens, and the
// copy/view/filter semantics stay intact.

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), 'utf8');
const report = read('../templates/report.html');
const interactions = read('../frontend/ReportInteractions.vue');
const style = read('style.css');

// Finding detail is now a drawer: the trigger carries the detail payload and no
// inline expand row is rendered.
assert.match(report, /data-detail-output="\{\{\.Output\}\}"/, 'details trigger must carry the finding evidence for the drawer');
assert.match(report, /aria-haspopup="dialog"/, 'details trigger must announce it opens a dialog');
assert.doesNotMatch(report, /class="details-row"/, 'inline finding detail rows must be removed in favor of the drawer');

// Report interactions: drawer state, open/close, and evidence highlight hook.
assert.match(interactions, /const detailFinding = ref/, 'report interactions must hold finding detail state');
assert.match(interactions, /detailDialog\.value\?\.showModal\(\)/, 'finding detail must open a native dialog');
assert.match(interactions, /highlightAllEvidences/, 'drawer must re-run evidence highlighting after opening');
assert.match(interactions, /report-detail-evidence/, 'drawer must expose a copyable evidence target');

// Long command content stays bounded inside its dialog.
assert.match(style, /\.command-pre\s*\{[^}]*max-height/, 'command output must be height-bounded so long commands stay readable');

// Risk summary wraps instead of forcing a fixed 5-column overflow on phones.
const narrow = style.slice(style.lastIndexOf('@media (max-width: 900px)'));
assert.match(narrow, /\.risk-summary\s*\{[^}]*auto-fit/, 'risk summary must use an auto-fit grid so it wraps on narrow screens');
assert.match(narrow, /\.popover-panel\s*\{[^}]*position:\s*static/, 'filter popovers must join the flow on narrow screens so they never overflow');
