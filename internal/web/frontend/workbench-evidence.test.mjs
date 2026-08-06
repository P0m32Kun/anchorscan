import assert from 'node:assert/strict';
import test from 'node:test';
import { useEvidenceQueue } from './workbench-evidence.ts';

// Ticket 06: the evidence queue composable centralizes paste/drag/select/upload/
// retry/reorder behaviour shared by the positive and negative workbench dialogs.

const png = new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'shot.png', { type: 'image/png' });

test('stage adds pending items and remove drops them', () => {
  const q = useEvidenceQueue();
  q.stage(png);
  q.stage(png);
  assert.equal(q.pending.value.length, 2);
  assert.equal(q.pending.value[0].status, 'pending');
  q.remove(0);
  assert.equal(q.pending.value.length, 1);
});

test('move reorders pending items without losing them', () => {
  const q = useEvidenceQueue();
  q.stage(png);
  q.stage(png);
  q.stage(png);
  q.move(2, -1);
  q.move(0, 1);
  assert.equal(q.pending.value.length, 3);
});

test('uploadAll drains the queue on success', async () => {
  const q = useEvidenceQueue();
  q.stage(png);
  q.stage(png);
  const results = await q.uploadAll('v1', async () => ({ id: 'e1' }));
  assert.deepEqual(results, { uploaded: 2, failed: 0 });
  assert.equal(q.pending.value.length, 0);
});

test('uploadAll keeps failures for retry and sets an error banner', async () => {
  const q = useEvidenceQueue();
  q.stage(png);
  q.stage(png);
  let calls = 0;
  const results = await q.uploadAll('v1', async () => {
    calls += 1;
    if (calls === 1) throw new Error('boom');
    return { id: 'e2' };
  });
  assert.deepEqual(results, { uploaded: 1, failed: 1 });
  assert.equal(q.pending.value.length, 1);
  assert.equal(q.pending.value[0].status, 'failed');
  assert.match(q.pending.value[0].error || '', /boom/);
  assert.ok(q.error.value.length > 0);
});

test('retry uploads one failed item and clears the banner when drained', async () => {
  const q = useEvidenceQueue();
  q.stage(png);
  await q.uploadAll('v1', async () => { throw new Error('boom'); });
  assert.equal(q.pending.value.length, 1);
  const ok = await q.retry(0, 'v1', async () => ({ id: 'e2' }));
  assert.equal(ok, true);
  assert.equal(q.pending.value.length, 0);
  assert.equal(q.error.value, '');
});

test('retry failure keeps the item failed', async () => {
  const q = useEvidenceQueue();
  q.stage(png);
  await q.uploadAll('v1', async () => { throw new Error('first'); });
  const ok = await q.retry(0, 'v1', async () => { throw new Error('second'); });
  assert.equal(ok, false);
  assert.equal(q.pending.value.length, 1);
  assert.equal(q.pending.value[0].status, 'failed');
});

test('clear revokes and empties state', () => {
  const q = useEvidenceQueue();
  q.stage(png);
  q.error.value = 'stale';
  q.clear();
  assert.equal(q.pending.value.length, 0);
  assert.equal(q.error.value, '');
});
