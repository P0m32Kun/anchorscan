import assert from 'node:assert/strict';
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

const root = new URL('..', import.meta.url).pathname;
const run = (fixture) => spawnSync('node', ['scripts/check_ai_workflow.mjs', '--root', fixture], { cwd: root, encoding: 'utf8' });
const fixture = () => {
  const dir = mkdtempSync(join(tmpdir(), 'anchorscan-harness-'));
  for (const name of ['.trellis', '.codex', '.pi']) cpSync(join(root, name), join(dir, name), { recursive: true });
  return dir;
};
const mutate = (dir, relative, from, to) => writeFileSync(join(dir, relative), readFileSync(join(dir, relative), 'utf8').replaceAll(from, to));

for (const [name, relative, from, expected] of [
  ['TDD/review', '.trellis/workflow.md', 'TDD Red', 'missing TDD Red'],
  ['task gate', '.trellis/scripts/task.py', 'validate_task_gate(full_path, repo_root, "ready")', 'strict lifecycle gate missing'],
  ['seed-only JSONL gate', '.trellis/scripts/common/task_context.py', '_real_context_entries(target_dir / name, repo_root) == 0', 'seed-only JSONL rejection'],
  ['placeholder', '.trellis/spec/backend/quality-guidelines.md', '# Backend Quality Guidelines', 'bootstrap placeholder remains'],
]) {
  const dir = fixture();
  if (name === 'placeholder') writeFileSync(join(dir, relative), 'To be filled by the team');
  else mutate(dir, relative, from, 'removed');
  const result = run(dir);
  rmSync(dir, { recursive: true, force: true });
  assert.notEqual(result.status, 0, name);
  assert.match(result.stderr, new RegExp(expected));
}
