import { api } from '@/lib/http';

/** Sends through the public /send/* endpoints using the instance token; `path` is what follows /send/. */
export async function sendMessage(token: string, path: string, payload: unknown): Promise<{ messageId: string }> {
  const res = await api<{ data?: { Info?: { ID?: string } } }>(`/send/${path}`, {
    method: 'POST',
    apikey: token,
    body: payload,
  });
  return { messageId: res?.data?.Info?.ID ?? '' };
}
