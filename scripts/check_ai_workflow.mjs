#!/usr/bin/env node
import { existsSync, readFileSync, readdirSync } from 'node:fs';
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
const contextGate = read('.trellis/scripts/common/task_context.py');
for (const anchor of ['_real_context_entries(target_dir / name, repo_root) == 0', 'seed-only or has no valid curated entries']) {
  if (!contextGate.includes(anchor)) errors.push(`task context: missing seed-only JSONL rejection`);
}
if (!contextGate.includes('if not delivery.get("commit") or not delivery.get("pr"):')) {
  errors.push('task context: missing delivery commit/PR gate');
}
const completeGate = contextGate.slice(contextGate.indexOf('if mode == "complete":'), contextGate.indexOf('\n    if errors:'));
if (/not\s+delivery\.get\("merged_at"\)/.test(completeGate)) {
  errors.push('task context: merged_at must remain an observed field, not a complete gate');
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
for (const relative of ['.codex/agents/trellis-check.toml', '.pi/agents/trellis-check.md', '.trellis/agents/check.md']) {
  requireText(relative, 'must not claim independent review', 'self-check boundary');
}
requireText('.pi/prompts/trellis-continue.md', 'Standards review', 'continue review route');
for (const [relative, text, label] of [
  ['.trellis/workflow.md', 'Never open a follow-up PR solely', 'non-recursive delivery'],
  ['.trellis/workflow.md', 'Do not ask again for branch creation, commit, push, PR, merge, archive, or journal.', 'continuous delivery autonomy'],
  ['.trellis/workflow.md', 'Trellis upstream changes, global installation, or npm publication', 'external delivery escalation'],
  ['.agents/skills/trellis-continue/SKILL.md', '分支、提交、push、PR、合并、归档和 journal 都直接执行', 'local continue autonomy'],
  ['.agents/skills/trellis-continue/SKILL.md', 'Trellis 上游、全局安装、npm 发布等外部持久变更升级', 'local continue escalation'],
  ['.agents/skills/trellis-finish-work/SKILL.md', 'Under continuous authorization, archive/journal are routine', 'local finish autonomy'],
  ['.agents/skills/trellis-finish-work/SKILL.md', 'Trellis upstream/global-install/npm-publication actions', 'local finish escalation'],
  ['.pi/prompts/trellis-continue.md', '分支、提交、push、PR、合并、归档和 journal 都直接执行', 'Pi continue autonomy'],
  ['.pi/prompts/trellis-continue.md', 'Trellis 上游、全局安装、npm 发布等外部持久变更需要升级', 'Pi continue escalation'],
  ['.pi/prompts/trellis-finish-work.md', 'Under continuous authorization, archive/journal are routine', 'Pi finish autonomy'],
  ['.pi/prompts/trellis-finish-work.md', 'Trellis upstream/global-install/npm-publication actions', 'Pi finish escalation'],
  ['.agents/skills/trellis-finish-work/SKILL.md', 'Do not create a follow-up metadata PR', 'local finish non-recursive delivery'],
  ['.pi/prompts/trellis-continue.md', '不得产生纯元数据后续 PR', 'Pi non-recursive delivery'],
  ['.pi/prompts/trellis-finish-work.md', 'Do not create a follow-up metadata PR', 'Pi finish non-recursive delivery'],
]) requireText(relative, text, label);

const walkTasks = (dir) => existsSync(dir) ? readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
  const path = `${dir}/${entry.name}`;
  return entry.isDirectory() ? walkTasks(path) : entry.name === 'task.json' ? [path] : [];
}) : [];
for (const taskPath of walkTasks(resolve(root, '.trellis/tasks'))) {
  const task = JSON.parse(readFileSync(taskPath, 'utf8'));
  const taskDir = taskPath.slice(0, -'/task.json'.length);
  if (task.status === 'completed' && existsSync(`${taskDir}/quality-evidence.json`)) {
    const evidence = JSON.parse(readFileSync(`${taskDir}/quality-evidence.json`, 'utf8'));
    if (evidence.schema === 1 && (task.meta?.bootstrap || typeof task.meta?.fixed_point === 'string')) {
      const delivery = evidence.delivery ?? {};
      if (!delivery.commit || !delivery.pr) errors.push(`${taskPath}: completed evidence lacks delivery commit/PR`);
    }
  }
  const source = task.meta?.source_of_truth;
  if (source?.type === 'docs-ticket' && typeof source.ticket === 'string') {
    const ticket = read(source.ticket);
    if (task.status === 'planning' && !ticket.includes('**Status:** ready-for-agent')) errors.push(`${taskPath}: planning task references non-ready ticket`);
  }
}
if (errors.length) {
  console.error(`AI workflow contract failed:\n- ${errors.join('\n- ')}`);
  process.exit(1);
}
console.log('AI workflow contract passed.');
