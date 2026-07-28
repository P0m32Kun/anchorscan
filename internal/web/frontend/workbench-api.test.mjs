import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = fs.readFileSync(new URL('./workbench-api.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
});
const api = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

test('normalizes malformed verification detail DTOs', () => {
  const detail = api.normalizeVerificationDetail({
    Verification: { Outcome: 'future_outcome', Severity: 'future_severity' },
    Assets: null,
    Sources: undefined,
    Evidence: null,
  });

  assert.equal(detail.Verification.Outcome, 'inconclusive');
  assert.equal(detail.Verification.Severity, 'low');
  assert.deepEqual(detail.Assets, []);
  assert.deepEqual(detail.Sources, []);
  assert.deepEqual(detail.Evidence, []);
});

test('keeps supported verification enum values', () => {
  assert.equal(api.normalizeOutcome('confirmed'), 'confirmed');
  assert.equal(api.normalizeSeverity('critical'), 'critical');
});
