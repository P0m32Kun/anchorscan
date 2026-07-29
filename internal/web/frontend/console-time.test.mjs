import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = fs.readFileSync(new URL('./console-time.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
});
const { formatConsoleTime } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

test('formats UTC timestamps in Shanghai with milliseconds', () => {
  assert.equal(formatConsoleTime('2026-07-29T07:18:40.714Z'), '2026-07-29 15:18:40.714 UTC+8');
  assert.equal(formatConsoleTime('2026-07-29T15:59:59.007Z'), '2026-07-29 23:59:59.007 UTC+8');
  assert.equal(formatConsoleTime('2026-07-29T16:00:00.000Z'), '2026-07-30 00:00:00.000 UTC+8');
});

test('falls back explicitly for invalid console timestamps', () => {
  assert.equal(formatConsoleTime('not-a-time'), 'invalid time: not-a-time');
});
