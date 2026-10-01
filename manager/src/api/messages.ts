import { api } from '@/lib/http';

export interface SendResult {
  messageId: string;
  /** Set when the message went out as something else than asked ("buttons" for a refused list). */
  fallback?: string;
  /** How many messages that took. */
  parts?: number;
}

/** Sends through the public /send/* endpoints using the instance token; `path` is what follows /send/. */
export async function sendMessage(token: string, path: string, payload: unknown): Promise<SendResult> {
  const res = await api<{ data?: { Info?: { ID?: string }; Fallback?: string; Parts?: number } }>(`/send/${path}`, {
    method: 'POST',
    apikey: token,
    body: payload,
  });
  return { messageId: res?.data?.Info?.ID ?? '', fallback: res?.data?.Fallback, parts: res?.data?.Parts };
}
