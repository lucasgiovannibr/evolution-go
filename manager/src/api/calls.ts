import { api } from '@/lib/http';

/** Phases of a call as the engine reports them (pkg/call/engine). */
export type CallPhase = 'calling' | 'ringing' | 'connecting' | 'active' | 'ended' | 'other';
export type CallDirection = 'incoming' | 'outgoing';

export interface StreamInfo {
  attached: boolean;
  toClient: number;
  fromClient: number;
  droppedToClient: number;
  droppedFromClient: number;
}

export interface CallInfo {
  callId: string;
  peer: string;
  direction: CallDirection;
  phase: CallPhase;
  video: boolean;
  startedAt: string;
  mediaStalled?: boolean;
  stream?: StreamInfo;
}

export type EngineState = 'active' | 'hook_failed' | 'blocked_by_proxy';

/** GET /call/active: the engine of the instance and the calls it follows. */
export interface ActiveCalls {
  enabled: boolean;
  state?: EngineState;
  error?: string;
  calls: CallInfo[];
}

export async function getActiveCalls(token: string): Promise<ActiveCalls> {
  const res = await api<Partial<ActiveCalls>>('/call/active', { apikey: token });
  return { enabled: !!res.enabled, state: res.state, error: res.error, calls: res.calls ?? [] };
}

export async function answerCall(token: string, callId: string): Promise<void> {
  await api('/call/answer', { method: 'POST', apikey: token, body: { callId } });
}

/** Ends a call in any phase; an incoming call that still rings is rejected. */
export async function hangupCall(token: string, callId: string): Promise<void> {
  await api('/call/hangup', { method: 'POST', apikey: token, body: { callId } });
}

/** What a stream ticket can ask for; the panel always asks for the same thing (see STREAM_OPTIONS). */
export interface StreamOptions {
  encoding: string;
  sampleRate: number;
  binary: boolean;
  speechEvents: boolean;
  /** The stream also carries the call's video (H.264 access units). */
  video?: boolean;
}

export interface StreamTicket {
  ticket: string;
  expiresInSeconds: number;
  path: string;
  encoding: string;
  sampleRate: number;
  binary: boolean;
  speechEvents: boolean;
}

export async function getStreamTicket(token: string, callId: string, options: StreamOptions): Promise<StreamTicket> {
  return api<StreamTicket>('/call/stream-ticket', { method: 'POST', apikey: token, body: { callId, ...options } });
}

/** Places a call (a video call when `video` is true). The audio is connected afterwards with a stream ticket. */
export async function dialCall(token: string, number: string, video = false): Promise<CallInfo> {
  return api<CallInfo>('/call/dial', { method: 'POST', apikey: token, body: { number, video } });
}

export type CallOutcome = 'answered' | 'missed' | 'rejected' | 'cancelled' | 'unanswered' | 'busy' | 'failed';

/** One row of GET /call/history (pkg/call/history). */
export interface CallRecord {
  id: string;
  callId: string;
  peer: string;
  peerPhone?: string;
  direction: CallDirection;
  video: boolean;
  outcome: CallOutcome;
  reason: string;
  startedAt: string;
  answeredAt?: string;
  endedAt: string;
  talkSeconds: number;
  ringSeconds: number;
}

export interface HistoryPage {
  records: CallRecord[];
  next?: string;
}

export interface HistoryQuery {
  direction?: CallDirection | '';
  outcome?: CallOutcome | '';
  peer?: string;
  limit?: number;
  cursor?: string;
}

export async function getCallHistory(token: string, q: HistoryQuery = {}): Promise<HistoryPage> {
  const res = await api<Partial<HistoryPage>>('/call/history', {
    apikey: token,
    query: { direction: q.direction || undefined, outcome: q.outcome || undefined, peer: q.peer || undefined, limit: q.limit, cursor: q.cursor },
  });
  return { records: res.records ?? [], next: res.next || undefined };
}

/** Erases the history of the instance; with `before` (RFC 3339) only what started before it. */
export async function deleteCallHistory(token: string, before?: string): Promise<number> {
  const res = await api<{ deleted?: number }>('/call/history', { method: 'DELETE', apikey: token, query: { before } });
  return res.deleted ?? 0;
}
