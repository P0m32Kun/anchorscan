import { ref } from 'vue';

// Ticket 06: narrow evidence queue module shared by the positive verify dialog
// and the negative verification dialog. Encapsulates pending file staging
// (paste / drag-drop / file select), caption, in-queue reorder, upload with
// per-item failure retention, retry, and the shared error banner. It is a
// queue of *pending* uploads only — persisted evidence is managed by the
// callers via the verification detail.

export type PendingEvidence = {
  file: File;
  caption: string;
  objectUrl: string;
  status: 'pending' | 'uploading' | 'uploaded' | 'failed';
  error?: string;
};

export type UploadResult = { uploaded: number; failed: number };

export function useEvidenceQueue() {
  const pending = ref<PendingEvidence[]>([]);
  const error = ref('');

  function stage(file: File) {
    pending.value.push({ file, caption: '', objectUrl: URL.createObjectURL(file), status: 'pending' });
  }

  function remove(index: number) {
    const item = pending.value[index];
    if (!item) return;
    URL.revokeObjectURL(item.objectUrl);
    pending.value.splice(index, 1);
  }

  // move reorders an item within the queue by delta (-1 up, +1 down).
  function move(index: number, delta: number) {
    const target = index + delta;
    if (target < 0 || target >= pending.value.length) return;
    const items = pending.value;
    const [item] = items.splice(index, 1);
    items.splice(target, 0, item);
  }

  function clear() {
    for (const item of pending.value) URL.revokeObjectURL(item.objectUrl);
    pending.value = [];
    error.value = '';
  }

  function hasFailed() {
    return pending.value.some((item) => item.status === 'failed');
  }

  // uploadAll tries every pending item, revoking successful ones and keeping
  // failed ones queued for retry. Returns how many succeeded and failed.
  async function uploadAll(verificationID: string, upload: (item: PendingEvidence, verificationID: string) => Promise<unknown>): Promise<UploadResult> {
    let uploaded = 0;
    let failed = 0;
    for (const item of pending.value) {
      item.status = 'uploading';
      item.error = undefined;
      try {
        await upload(item, verificationID);
        URL.revokeObjectURL(item.objectUrl);
        item.status = 'uploaded';
        uploaded += 1;
      } catch (uploadError: any) {
        item.status = 'failed';
        item.error = uploadError?.message || '上传失败';
        failed += 1;
      }
    }
    pending.value = pending.value.filter((item) => item.status !== 'uploaded');
    if (failed > 0) error.value = `部分截图上传失败（${failed} 张）；请重试失败项。`;
    return { uploaded, failed };
  }

  // retry uploads a single failed item; removes it by identity (not the caller
  // index) so a concurrent removal of an earlier item cannot splice the wrong
  // file, and clears the banner when the queue drains.
  async function retry(index: number, verificationID: string, upload: (item: PendingEvidence, verificationID: string) => Promise<unknown>): Promise<boolean> {
    const item = pending.value[index];
    if (!item || item.status === 'uploading') return false;
    item.status = 'uploading';
    item.error = undefined;
    try {
      await upload(item, verificationID);
      URL.revokeObjectURL(item.objectUrl);
      pending.value = pending.value.filter((candidate) => candidate !== item);
      if (!hasFailed()) error.value = '';
      return true;
    } catch (uploadError: any) {
      item.status = 'failed';
      item.error = uploadError?.message || '上传失败';
      return false;
    }
  }

  return { pending, error, stage, remove, move, clear, hasFailed, uploadAll, retry };
}
