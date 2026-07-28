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
