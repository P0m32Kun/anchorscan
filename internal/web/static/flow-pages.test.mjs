import assert from 'node:assert/strict';
import fs from 'node:fs';

// Nmap Viewer consolidation, ticket 03: project/scan/run/tool flow contract.
// Guards: no orphan design tokens or inline styles on tool pages, visible
// dangerous operations on project detail, run context preservation, and
// unified cancel/stage/lifecycle feedback between the two run surfaces.

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), 'utf8');
const toolPage = read('../templates/tool_page.html');
const projectDetail = read('../templates/project_detail.html');
const runDetail = read('../frontend/RunDetail.vue');
const toolFeedback = read('../frontend/ToolRunFeedback.vue');
const scanCreate = read('../frontend/ScanCreate.vue');

// Tool page: no inline styles and no orphan (undefined) design tokens.
assert.doesNotMatch(toolPage, /style="/, 'tool_page.html must not use inline styles');
assert.doesNotMatch(toolPage, /--color-surface-2|--color-warning/, 'tool_page.html must not reference undefined CSS tokens');
assert.match(toolPage, /alert alert-warning tool-run-warning/, 'native-args warning must use the shared alert-warning primitive');

// Project detail: dangerous operations are visible and use the shared confirm flow.
assert.match(projectDetail, /danger-zone-panel/, 'project detail must show a danger zone section');
assert.match(projectDetail, /name="_method" value="delete"/, 'project delete must reuse the method-override delete endpoint');
assert.match(projectDetail, /data-confirm-form data-confirm-title="删除项目"/, 'project delete must use the shared confirmation dialog');

// Run detail: project context survives navigation.
assert.match(runDetail, /返回项目/, 'run detail must link back to the owning project');
assert.match(runDetail, /\/projects\/\$\{project_id\}/, 'return-to-project link must use the run project id');

// Tool feedback: cancel, stage, and lifecycle wording unified with the run monitor.
assert.match(toolFeedback, /\/runs\/\$\{encodeURIComponent\(runID\.value\)\}\/cancel/, 'tool feedback must call the shared cancel endpoint');
assert.match(toolFeedback, /中止工具/, 'tool feedback must expose a cancel action while running');
assert.match(toolFeedback, /当前阶段：\$\{latestStage\}/, 'tool feedback must show the current stage while running');
assert.match(toolFeedback, /completed_with_errors/, 'tool feedback must share the run lifecycle vocabulary');
assert.match(toolFeedback, /interrupted/, 'tool feedback must report interrupted runs explicitly');

// Scan create: field-level error targeting and required/optional framing.
assert.match(scanCreate, /aria-invalid/, 'scan create must mark invalid fields with aria-invalid');
assert.match(scanCreate, /带 \* 为必填项/, 'scan create must explain required vs optional fields');
assert.match(scanCreate, /errorSummary/, 'scan create must keep the focusable error summary');

// Shell primitive: danger/link submit buttons must not inherit the primary
// submit look. A classed .button-danger or .link-button of type=submit would
// otherwise render as a solid primary pill (blue circle on zone delete).
const style = read('style.css');
assert.match(style, /button\[type="submit"\]:not\(\.button, \.link-button\)/, 'submit styling must exclude classed buttons so danger/link actions keep their own look');
assert.doesNotMatch(style, /\.button-primary,\nbutton\[type="submit"\] \{/, 'primary submit rule must not fall back to styling all submit buttons');
