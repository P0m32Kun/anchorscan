const outcomes = ['confirmed', 'inconclusive', 'not_observed'] as const;
const severities = ['critical', 'high', 'medium', 'low', 'info'] as const;

export function normalizeOutcome(value: unknown) {
  return typeof value === 'string' && outcomes.includes(value as typeof outcomes[number]) ? value : 'inconclusive';
}

export function normalizeSeverity(value: unknown) {
  return typeof value === 'string' && severities.includes(value as typeof severities[number]) ? value : 'low';
}

export function normalizeVerificationDetail<T extends Record<string, unknown>>(detail: T): T {
  const verification = detail.Verification as Record<string, unknown> | undefined;
  return {
    ...detail,
    Verification: verification ? {
      ...verification,
      Outcome: normalizeOutcome(verification.Outcome),
      Severity: normalizeSeverity(verification.Severity),
    } : verification,
    Assets: Array.isArray(detail.Assets) ? detail.Assets : [],
    Sources: Array.isArray(detail.Sources) ? detail.Sources : [],
    Evidence: Array.isArray(detail.Evidence) ? detail.Evidence : [],
  } as T;
}
async function errorMessage(response: Response, fallback: string): Promise<string> {
  return (await response.text()).trim() || fallback;
}

export async function getJSON<T>(url: string, fallback: string): Promise<T> {
  const response = await fetch(url);
  if (!response.ok) throw new Error(await errorMessage(response, fallback));
  return await response.json() as T;
}

export async function postJSON<T>(url: string, payload: unknown, fallback: string): Promise<T> {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!response.ok) throw new Error(await errorMessage(response, fallback));
  return await response.json() as T;
}

export async function uploadEvidence<T>(url: string, file: File, caption: string): Promise<T> {
  const form = new FormData();
  form.append('file', file);
  form.append('caption', caption);
  const response = await fetch(url, { method: 'POST', body: form });
  if (!response.ok) throw new Error(await errorMessage(response, '上传失败'));
  return await response.json() as T;
}
