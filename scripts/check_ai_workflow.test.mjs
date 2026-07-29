import assert from 'node:assert/strict';
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

const root = new URL('..', import.meta.url).pathname;
const run = (fixture) => spawnSync('node', ['scripts/check_ai_workflow.mjs', '--root', fixture], { cwd: root, encoding: 'utf8' });
const fixture = () => {
  const dir = mkdtempSync(join(tmpdir(), 'anchorscan-harness-'));
  for (const name of ['.trellis', '.codex', '.pi', '.agents', 'docs']) cpSync(join(root, name), join(dir, name), { recursive: true });
  return dir;
};
const mutate = (dir, relative, from, to) => writeFileSync(join(dir, relative), readFileSync(join(dir, relative), 'utf8').replaceAll(from, to));

for (const [name, relative, from, expected] of [
  ['TDD/review', '.trellis/workflow.md', 'TDD Red', 'missing TDD Red'],
  ['task gate', '.trellis/scripts/task.py', 'validate_task_gate(full_path, repo_root, "ready")', 'strict lifecycle gate missing'],
  ['seed-only JSONL gate', '.trellis/scripts/common/task_context.py', '_real_context_entries(target_dir / name, repo_root) == 0', 'seed-only JSONL rejection'],
  ['placeholder', '.trellis/spec/backend/quality-guidelines.md', '# Backend Quality Guidelines', 'bootstrap placeholder remains'],
  ['check boundary', '.pi/agents/trellis-check.md', 'must not claim independent review', 'self-check boundary'],
  ['delivery commit/PR gate', '.trellis/scripts/common/task_context.py', 'if not delivery.get("commit") or not delivery.get("pr"):', 'delivery commit/PR gate'],
  ['non-recursive delivery', '.trellis/workflow.md', 'Never open a follow-up PR solely', 'non-recursive delivery'],
  ['continuous delivery autonomy', '.trellis/workflow.md', 'Do not ask again for branch creation, commit, push, PR, merge, archive, or journal.', 'continuous delivery autonomy'],
  ['external delivery escalation', '.trellis/workflow.md', 'Trellis upstream changes, global installation, or npm publication', 'external delivery escalation'],
  ['Pi non-recursive delivery', '.pi/prompts/trellis-continue.md', '不得产生纯元数据后续 PR', 'Pi non-recursive delivery'],
  ['local skill autonomy', '.agents/skills/trellis-continue/SKILL.md', '分支、提交、push、PR、合并、归档和 journal 都直接执行', 'local continue autonomy'],
  ['local finish escalation', '.agents/skills/trellis-finish-work/SKILL.md', 'Trellis upstream/global-install/npm-publication actions', 'local finish escalation'],
  ['Pi finish autonomy', '.pi/prompts/trellis-finish-work.md', 'Under continuous authorization, archive/journal are routine', 'Pi finish autonomy'],
]) {
  const dir = fixture();
  if (name === 'placeholder') writeFileSync(join(dir, relative), 'To be filled by the team');
  else mutate(dir, relative, from, 'removed');
  const result = run(dir);
  rmSync(dir, { recursive: true, force: true });
  assert.notEqual(result.status, 0, name);
  assert.match(result.stderr, new RegExp(expected));
}

{
  const dir = fixture();
  const relative = '.trellis/scripts/common/task_context.py';
  const path = join(dir, relative);
  mutate(dir, relative, 'if not delivery.get("commit") or not delivery.get("pr"):', 'if not delivery.get("commit") or not delivery.get("pr") or not delivery.get("merged_at"):');
  const result = run(dir);
  rmSync(dir, { recursive: true, force: true });
  assert.notEqual(result.status, 0, 'merged_at remains observed-only');
  assert.match(result.stderr, /merged_at must remain an observed field/);
}
