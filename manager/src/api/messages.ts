import { api } from '@/lib/http';

export type SendKind = 'text' | 'button' | 'list' | 'carousel';

/** Sends through the public /send/* endpoints using the instance token. */
export async function sendMessage(token: string, kind: SendKind, payload: unknown): Promise<{ messageId: string }> {
  const res = await api<{ data?: { Info?: { ID?: string } } }>(`/send/${kind}`, {
    method: 'POST',
    apikey: token,
    body: payload,
  });
  return { messageId: res?.data?.Info?.ID ?? '' };
}
