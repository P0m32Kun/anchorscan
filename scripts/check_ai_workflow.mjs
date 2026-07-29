#!/usr/bin/env node
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const rootArg = process.argv.indexOf('--root');
const root = rootArg >= 0 ? resolve(process.argv[rootArg + 1]) : resolve(import.meta.dirname, '..');
const errors = [];
const read = (relative) => {
  const path = resolve(root, relative);
  if (!existsSync(path)) {
    errors.push(`missing tracked contract file: ${relative}`);
    return '';
  }
  return readFileSync(path, 'utf8');
};
const requireText = (relative, text, label) => {
  if (!read(relative).includes(text)) errors.push(`${relative}: missing ${label}`);
};

const workflow = read('.trellis/workflow.md');
for (const state of ['in_progress', 'in_progress-inline']) {
  const start = workflow.indexOf(`[workflow-state:${state}]`);
  const end = workflow.indexOf(`[/workflow-state:${state}]`, start);
  const block = start >= 0 && end > start ? workflow.slice(start, end) : '';
  for (const anchor of ['TDD Red', 'Standards review', 'Spec/AC review', 'full verification', 'PR']) {
    if (!block.includes(anchor)) errors.push(`workflow ${state}: missing ${anchor}`);
  }
}
for (const needle of ['validate_task_gate(full_path, repo_root, "ready")', 'validate_task_gate(task_dir, repo_root, "complete")']) {
  const file = needle.includes('full_path') ? '.trellis/scripts/task.py' : '.trellis/scripts/common/task_store.py';
  if (!read(file).includes(needle)) errors.push(`${file}: strict lifecycle gate missing`);
}
for (const relative of ['.trellis/spec/backend/index.md', '.trellis/spec/frontend/index.md']) {
  const text = read(relative);
  for (const section of ['Pre-Development Checklist', 'Quality Check']) {
    if (!text.includes(section)) errors.push(`${relative}: missing ${section}`);
  }
}
for (const relative of ['.trellis/spec/backend', '.trellis/spec/frontend']) {
  // Stable representative files are enough to reject restored bootstrap placeholders.
  for (const name of relative.endsWith('backend') ? ['quality-guidelines.md'] : ['quality-guidelines.md']) {
    if (read(`${relative}/${name}`).includes('To be filled by the team')) errors.push(`${relative}/${name}: bootstrap placeholder remains`);
  }
}
for (const relative of ['.codex/agents/trellis-implement.toml', '.pi/agents/trellis-implement.md', '.trellis/agents/implement.md']) {
  requireText(relative, 'TDD Red', 'TDD anchor');
}
if (errors.length) {
  console.error(`AI workflow contract failed:\n- ${errors.join('\n- ')}`);
  process.exit(1);
}
console.log('AI workflow contract passed.');
